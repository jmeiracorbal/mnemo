// Package store implements the persistent memory engine for mnemo.
//
// It uses SQLite with FTS5 full-text search to store and retrieve
// observations from AI coding sessions. CLI, MCP, and agent integrations
// talk to this package.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
	dbmigrate "github.com/jmeiracorbal/mnemo/internal/db/migrate"
	_ "modernc.org/sqlite"
)

var openDB = sql.Open

// ─── Config ──────────────────────────────────────────────────────────────────

type Config struct {
	DataDir              string
	AllowUpgradeMaintenance bool
	MaxObservationLength int
	MaxContextResults    int
	MaxSearchResults     int
	DedupeWindow         time.Duration
}

func DefaultConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("mnemo: determine home directory: %w", err)
	}
	return Config{
		DataDir:              filepath.Join(home, ".mnemo"),
		MaxObservationLength: 50000,
		MaxContextResults:    20,
		MaxSearchResults:     20,
		DedupeWindow:         15 * time.Minute,
	}, nil
}

// FallbackConfig returns a Config with the given DataDir and default values.
func FallbackConfig(dataDir string) Config {
	return Config{
		DataDir:              dataDir,
		MaxObservationLength: 50000,
		MaxContextResults:    20,
		MaxSearchResults:     20,
		DedupeWindow:         15 * time.Minute,
	}
}

// MaxObservationLength returns the configured maximum content length for observations.
func (s *Store) MaxObservationLength() int {
	return s.cfg.MaxObservationLength
}

// DataDir returns the store-owned data directory. Controllers use it to keep
// JetStream persistence beside the database without exposing database handles.
func (s *Store) DataDir() string {
	return s.cfg.DataDir
}

// ─── Store ───────────────────────────────────────────────────────────────────

type Store struct {
	db       *sql.DB
	q        *dbgen.Queries
	cfg      Config
	hooks    storeHooks
	activeTx *sql.Tx
}

type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type sqlRowScanner struct {
	rows *sql.Rows
}

func (r sqlRowScanner) Next() bool             { return r.rows.Next() }
func (r sqlRowScanner) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r sqlRowScanner) Err() error             { return r.rows.Err() }
func (r sqlRowScanner) Close() error           { return r.rows.Close() }

type storeHooks struct {
	query   func(db queryer, query string, args ...any) (*sql.Rows, error)
	queryIt func(db queryer, query string, args ...any) (rowScanner, error)
	beginTx func(db *sql.DB) (*sql.Tx, error)
	commit  func(tx *sql.Tx) error
}

func defaultStoreHooks() storeHooks {
	return storeHooks{
		query: func(db queryer, query string, args ...any) (*sql.Rows, error) {
			return db.Query(query, args...)
		},
		queryIt: func(db queryer, query string, args ...any) (rowScanner, error) {
			rows, err := db.Query(query, args...)
			if err != nil {
				return nil, err
			}
			return sqlRowScanner{rows: rows}, nil
		},
		beginTx: func(db *sql.DB) (*sql.Tx, error) {
			return db.Begin()
		},
		commit: func(tx *sql.Tx) error {
			return tx.Commit()
		},
	}
}

func (s *Store) queryHook(db queryer, query string, args ...any) (*sql.Rows, error) {
	if s.hooks.query != nil {
		return s.hooks.query(db, query, args...)
	}
	return db.Query(query, args...)
}

func (s *Store) queryItHook(db queryer, query string, args ...any) (rowScanner, error) {
	if s.hooks.queryIt != nil {
		return s.hooks.queryIt(db, query, args...)
	}
	rows, err := s.queryHook(db, query, args...)
	if err != nil {
		return nil, err
	}
	return sqlRowScanner{rows: rows}, nil
}

func (s *Store) beginTxHook() (*sql.Tx, error) {
	if s.hooks.beginTx != nil {
		return s.hooks.beginTx(s.db)
	}
	return s.db.Begin()
}

func (s *Store) commitHook(tx *sql.Tx) error {
	if s.hooks.commit != nil {
		return s.hooks.commit(tx)
	}
	return tx.Commit()
}

func New(cfg Config) (*Store, error) {
	if !filepath.IsAbs(cfg.DataDir) {
		return nil, fmt.Errorf("mnemo: data directory must be an absolute path, got %q", cfg.DataDir)
	}
	// Ordinary CLI and MCP processes must not open or migrate SQLite during a
	// release rollback. Only the managed controller's verification run is allowed.
	if !cfg.AllowUpgradeMaintenance {
		if _,err:=os.Stat(filepath.Join(cfg.DataDir,"upgrade.maintenance"));err==nil {
			return nil,fmt.Errorf("mnemo: SQLite upgrade transaction is active")
		} else if !os.IsNotExist(err) {
			return nil,fmt.Errorf("mnemo: inspect upgrade transaction: %w",err)
		}
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mnemo: create data dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "memory.db")
	// modernc.org/sqlite applies _pragma parameters to every physical
	// connection and applies busy_timeout before the other pragmas. This is
	// important because database/sql may create more than one connection.
	db, err := openDB("sqlite", dbPath+"?_pragma=busy_timeout%3d5000")
	if err != nil {
		return nil, fmt.Errorf("mnemo: open database: %w", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mnemo: read journal mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("mnemo: pragma %q: %w", "PRAGMA journal_mode = WAL", err)
		}
	}

	pragmas := []string{
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("mnemo: pragma %q: %w", p, err)
		}
	}

	s := &Store{db: db, q: dbgen.New(db), cfg: cfg, hooks: defaultStoreHooks()}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mnemo: migration: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// ─── Migrations ──────────────────────────────────────────────────────────────

func (s *Store) migrate() error {
	_, err := dbmigrate.Apply(s.db)
	return err
}

// checkUpgradeWrites prevents already-open MCP and CLI store connections
// from committing between the SQLite snapshot and release decision.
func (s *Store) checkUpgradeWrites() error {
	if _,err:=os.Stat(filepath.Join(s.cfg.DataDir,"upgrade.maintenance"));err==nil {
		return fmt.Errorf("Mnemo release transaction in progress; write rejected")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot verify upgrade write fence: %w",err)
	}
	return nil
}

func (s *Store) withTx(fn func(tx *sql.Tx) error) error {
	if err:=s.checkUpgradeWrites();err!=nil{return err}
	if s.activeTx != nil {
		return fn(s.activeTx)
	}
	tx, err := s.beginTxHook()
	if err != nil {return err}
	defer tx.Rollback()
	if err := fn(tx); err != nil {return err}
	if err:=s.checkUpgradeWrites();err!=nil{return err}
	return s.commitHook(tx)
}

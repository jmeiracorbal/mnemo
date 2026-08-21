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
	"regexp"
	"time"

	dbschema "github.com/jmeiracorbal/mnemo/internal/db"
	dbgen "github.com/jmeiracorbal/mnemo/internal/db/generated"
	_ "modernc.org/sqlite"
)

var openDB = sql.Open

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var allowedColumnDefinitions = map[string]bool{
	"TEXT":                            true,
	"TEXT NOT NULL DEFAULT 'project'": true,
	"INTEGER NOT NULL DEFAULT 1":      true,
	"TEXT NOT NULL DEFAULT ''":        true,
	"INTEGER REFERENCES provenance_contexts(id)": true,
}

// ─── Config ──────────────────────────────────────────────────────────────────

type Config struct {
	DataDir              string
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

// ─── Store ───────────────────────────────────────────────────────────────────

type Store struct {
	db    *sql.DB
	q     *dbgen.Queries
	cfg   Config
	hooks storeHooks
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
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
	exec    func(db execer, query string, args ...any) (sql.Result, error)
	query   func(db queryer, query string, args ...any) (*sql.Rows, error)
	queryIt func(db queryer, query string, args ...any) (rowScanner, error)
	beginTx func(db *sql.DB) (*sql.Tx, error)
	commit  func(tx *sql.Tx) error
}

func defaultStoreHooks() storeHooks {
	return storeHooks{
		exec: func(db execer, query string, args ...any) (sql.Result, error) {
			return db.Exec(query, args...)
		},
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

func (s *Store) execHook(db execer, query string, args ...any) (sql.Result, error) {
	if s.hooks.exec != nil {
		return s.hooks.exec(db, query, args...)
	}
	return db.Exec(query, args...)
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
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mnemo: create data dir: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "memory.db")
	db, err := openDB("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("mnemo: open database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
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
	if err := s.repairEnrolledProjectSyncMutations(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mnemo: repair enrolled sync journal: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// ─── Migrations ──────────────────────────────────────────────────────────────

func (s *Store) migrate() error {
	if _, err := s.execHook(s.db, dbschema.ApplyTableSQL()); err != nil {
		return err
	}
	if err := s.seedProvenanceCatalog(); err != nil {
		return err
	}

	observationColumns := []struct {
		name       string
		definition string
	}{
		{name: "sync_id", definition: "TEXT"},
		{name: "scope", definition: "TEXT NOT NULL DEFAULT 'project'"},
		{name: "topic_key", definition: "TEXT"},
		{name: "normalized_hash", definition: "TEXT"},
		{name: "revision_count", definition: "INTEGER NOT NULL DEFAULT 1"},
		{name: "duplicate_count", definition: "INTEGER NOT NULL DEFAULT 1"},
		{name: "last_seen_at", definition: "TEXT"},
		{name: "updated_at", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "deleted_at", definition: "TEXT"},
	}
	for _, c := range observationColumns {
		if err := s.addColumnIfNotExists("observations", c.name, c.definition); err != nil {
			return err
		}
	}

	if err := s.migrateLegacyObservationsTable(); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("observations", "provenance_id", "INTEGER REFERENCES provenance_contexts(id)"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("sessions", "provenance_id", "INTEGER REFERENCES provenance_contexts(id)"); err != nil {
		return err
	}

	if err := s.addColumnIfNotExists("user_prompts", "sync_id", "TEXT"); err != nil {
		return err
	}
	if err := s.addColumnIfNotExists("user_prompts", "provenance_id", "INTEGER REFERENCES provenance_contexts(id)"); err != nil {
		return err
	}

	if err := s.addColumnIfNotExists("sync_mutations", "project", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	if _, err := s.execHook(s.db, dbschema.ApplyObjectSQL()); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `
		INSERT INTO observations_fts(observations_fts) VALUES('rebuild');
		INSERT INTO prompts_fts(prompts_fts) VALUES('rebuild');
	`); err != nil {
		return err
	}

	if _, err := s.execHook(s.db, `
		UPDATE sync_mutations
		SET project = COALESCE(json_extract(payload, '$.project'), '')
		WHERE project = '' AND payload != ''
	`); err != nil {
		return err
	}

	if _, err := s.execHook(s.db, `UPDATE observations SET scope = 'project' WHERE scope IS NULL OR scope = ''`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `UPDATE observations SET topic_key = NULL WHERE topic_key = ''`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `UPDATE observations SET revision_count = 1 WHERE revision_count IS NULL OR revision_count < 1`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `UPDATE observations SET duplicate_count = 1 WHERE duplicate_count IS NULL OR duplicate_count < 1`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `UPDATE observations SET updated_at = created_at WHERE updated_at IS NULL OR updated_at = ''`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `UPDATE observations SET sync_id = 'obs-' || lower(hex(randomblob(16))) WHERE sync_id IS NULL OR sync_id = ''`); err != nil {
		return err
	}

	if _, err := s.execHook(s.db, `UPDATE user_prompts SET project = '' WHERE project IS NULL`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `UPDATE user_prompts SET sync_id = 'prompt-' || lower(hex(randomblob(16))) WHERE sync_id IS NULL OR sync_id = ''`); err != nil {
		return err
	}
	if _, err := s.execHook(s.db, `INSERT OR IGNORE INTO sync_state (target_key, lifecycle, updated_at) VALUES ('cloud', 'idle', datetime('now'))`); err != nil {
		return err
	}

	return nil
}

func (s *Store) addColumnIfNotExists(tableName, columnName, definition string) error {
	if !sqlIdentifierPattern.MatchString(tableName) || !sqlIdentifierPattern.MatchString(columnName) {
		return fmt.Errorf("invalid migration identifier")
	}
	if !allowedColumnDefinitions[definition] {
		return fmt.Errorf("unsupported migration column definition %q", definition)
	}
	rows, err := s.queryItHook(s.db, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, definition))
	return err
}

func (s *Store) migrateLegacyObservationsTable() error {
	rows, err := s.queryItHook(s.db, "PRAGMA table_info(observations)")
	if err != nil {
		return err
	}
	defer rows.Close()

	var hasID bool
	var idIsPrimaryKey bool
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "id" {
			hasID = true
			idIsPrimaryKey = pk == 1
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasID || idIsPrimaryKey {
		return nil
	}

	tx, err := s.beginTxHook()
	if err != nil {
		return fmt.Errorf("migrate legacy observations: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := s.execHook(tx, `
		CREATE TABLE observations_migrated (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			sync_id    TEXT,
			session_id TEXT    NOT NULL,
			type       TEXT    NOT NULL,
			title      TEXT    NOT NULL,
			content    TEXT    NOT NULL,
			tool_name  TEXT,
			project    TEXT,
			scope      TEXT    NOT NULL DEFAULT 'project',
			topic_key  TEXT,
			normalized_hash TEXT,
			revision_count INTEGER NOT NULL DEFAULT 1,
			duplicate_count INTEGER NOT NULL DEFAULT 1,
			last_seen_at TEXT,
			created_at TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT    NOT NULL DEFAULT (datetime('now')),
			deleted_at TEXT,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
	`); err != nil {
		return fmt.Errorf("migrate legacy observations: create table: %w", err)
	}

	if _, err := s.execHook(tx, `
		INSERT INTO observations_migrated (
			id, sync_id, session_id, type, title, content, tool_name, project,
			scope, topic_key, normalized_hash, revision_count, duplicate_count,
			last_seen_at, created_at, updated_at, deleted_at
		)
		SELECT
			CASE
				WHEN id IS NULL THEN NULL
				WHEN ROW_NUMBER() OVER (PARTITION BY id ORDER BY rowid) = 1 THEN CAST(id AS INTEGER)
				ELSE NULL
			END,
			'obs-' || lower(hex(randomblob(16))),
			session_id,
			COALESCE(NULLIF(type, ''), 'manual'),
			COALESCE(NULLIF(title, ''), 'Untitled observation'),
			COALESCE(content, ''),
			tool_name,
			project,
			CASE WHEN scope IS NULL OR scope = '' THEN 'project' ELSE scope END,
			NULLIF(topic_key, ''),
			normalized_hash,
			CASE WHEN revision_count IS NULL OR revision_count < 1 THEN 1 ELSE revision_count END,
			CASE WHEN duplicate_count IS NULL OR duplicate_count < 1 THEN 1 ELSE duplicate_count END,
			last_seen_at,
			COALESCE(NULLIF(created_at, ''), datetime('now')),
			COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, ''), datetime('now')),
			deleted_at
		FROM observations
		ORDER BY rowid;
	`); err != nil {
		return fmt.Errorf("migrate legacy observations: copy rows: %w", err)
	}

	if _, err := s.execHook(tx, "DROP TABLE observations"); err != nil {
		return fmt.Errorf("migrate legacy observations: drop old table: %w", err)
	}

	if _, err := s.execHook(tx, "ALTER TABLE observations_migrated RENAME TO observations"); err != nil {
		return fmt.Errorf("migrate legacy observations: rename table: %w", err)
	}

	if _, err := s.execHook(tx, `
		DROP TRIGGER IF EXISTS obs_fts_insert;
		DROP TRIGGER IF EXISTS obs_fts_update;
		DROP TRIGGER IF EXISTS obs_fts_delete;
		DROP TABLE IF EXISTS observations_fts;
		CREATE VIRTUAL TABLE observations_fts USING fts5(
			title,
			content,
			tool_name,
			type,
			project,
			content='observations',
			content_rowid='id'
		);
		INSERT INTO observations_fts(rowid, title, content, tool_name, type, project)
		SELECT id, title, content, tool_name, type, project
		FROM observations
		WHERE deleted_at IS NULL;
	`); err != nil {
		return fmt.Errorf("migrate legacy observations: rebuild fts: %w", err)
	}

	if err := s.commitHook(tx); err != nil {
		return fmt.Errorf("migrate legacy observations: commit: %w", err)
	}

	return nil
}

func (s *Store) withTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.beginTxHook()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return s.commitHook(tx)
}

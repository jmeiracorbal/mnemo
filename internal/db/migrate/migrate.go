package migrate

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	dbfiles "github.com/jmeiracorbal/mnemo/database"
	_ "modernc.org/sqlite"
)

const metadataDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    dirty INTEGER NOT NULL DEFAULT 0,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);`

type State string

const (
	StateMissing      State = "missing"
	StateUpToDate     State = "up_to_date"
	StatePending      State = "pending"
	StateApplied      State = "applied"
	StateInconsistent State = "inconsistent"
)

type Migration struct {
	Version  string `json:"version"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
}

type AppliedMigration struct {
	Version   string `json:"version"`
	Name      string `json:"name"`
	Checksum  string `json:"checksum"`
	Dirty     bool   `json:"dirty"`
	AppliedAt string `json:"applied_at"`
}

type Status struct {
	State          State              `json:"state"`
	DBPath         string             `json:"db_path,omitempty"`
	Exists         bool               `json:"exists"`
	CurrentVersion string             `json:"current_version,omitempty"`
	LatestVersion  string             `json:"latest_version,omitempty"`
	Applied        []AppliedMigration `json:"applied,omitempty"`
	Pending        []Migration        `json:"pending,omitempty"`
	Message        string             `json:"message,omitempty"`
}

type InconsistentError struct {
	Reason string
}

func (e *InconsistentError) Error() string {
	if e == nil || strings.TrimSpace(e.Reason) == "" {
		return "database schema is inconsistent; run `mnemo db migrate` or restore from backup"
	}
	return "database schema is inconsistent: " + e.Reason + "; run `mnemo db migrate` or restore from backup"
}

func DBPath(dataDir string) string {
	return filepath.Join(dataDir, "memory.db")
}

func ApplyDataDir(dataDir string) (Status, error) {
	if !filepath.IsAbs(dataDir) {
		return Status{}, fmt.Errorf("data directory must be an absolute path, got %q", dataDir)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return Status{}, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := DBPath(dataDir)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return Status{}, fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := applyPragmas(db); err != nil {
		return Status{}, err
	}
	status, err := Apply(db)
	status.DBPath = dbPath
	status.Exists = true
	return status, err
}

func CheckDataDir(dataDir string) (Status, error) {
	if !filepath.IsAbs(dataDir) {
		return Status{}, fmt.Errorf("data directory must be an absolute path, got %q", dataDir)
	}
	dbPath := DBPath(dataDir)
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		migrations, loadErr := LoadMigrations()
		if loadErr != nil {
			return Status{}, loadErr
		}
		return Status{State: StateMissing, DBPath: dbPath, Exists: false, LatestVersion: latestVersion(migrations), Message: "memory database not found yet"}, nil
	} else if err != nil {
		return Status{}, fmt.Errorf("stat database: %w", err)
	}
	db, err := sql.Open("sqlite", sqliteReadOnlyDBURI(dbPath))
	if err != nil {
		return Status{}, fmt.Errorf("open database read-only: %w", err)
	}
	defer func() { _ = db.Close() }()
	status, err := Check(db)
	status.DBPath = dbPath
	status.Exists = true
	return status, err
}

func Apply(db *sql.DB) (Status, error) {
	migrations, err := LoadMigrations()
	if err != nil {
		return Status{}, err
	}
	if _, err := db.Exec(metadataDDL); err != nil {
		return Status{}, fmt.Errorf("create migration metadata: %w", err)
	}
	status, err := checkAgainstMigrations(db, migrations)
	if err != nil {
		return status, err
	}
	hadPending := len(status.Pending) > 0
	for _, migration := range status.Pending {
		if err := applyMigration(db, migration); err != nil {
			return status, err
		}
	}
	status, err = checkAgainstMigrations(db, migrations)
	if err != nil {
		return status, err
	}
	if err := ValidateCurrent(db); err != nil {
		status.State = StateInconsistent
		status.Message = err.Error()
		return status, err
	}
	if hadPending {
		status.State = StateApplied
		status.Message = "database migrations applied"
	}
	return status, nil
}

func Check(db *sql.DB) (Status, error) {
	migrations, err := LoadMigrations()
	if err != nil {
		return Status{}, err
	}
	status, err := checkAgainstMigrations(db, migrations)
	if err != nil {
		return status, err
	}
	if len(status.Pending) > 0 {
		status.State = StatePending
		status.Message = "database requires migration; run `mnemo db migrate`"
		return status, nil
	}
	if err := ValidateCurrent(db); err != nil {
		status.State = StateInconsistent
		status.Message = err.Error()
		return status, err
	}
	status.State = StateUpToDate
	status.Message = "database schema is up to date"
	return status, nil
}

func LoadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(dbfiles.Migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var migrations []Migration
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, ok := parseMigrationName(entry.Name())
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		if seen[version] {
			return nil, fmt.Errorf("duplicate migration version %q", version)
		}
		seen[version] = true
		content, err := fs.ReadFile(dbfiles.Migrations, filepath.ToSlash(filepath.Join("migrations", entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, Migration{Version: version, Name: strings.TrimSuffix(name, ".sql"), Checksum: checksum(content)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no embedded migrations found")
	}
	return migrations, nil
}

func applyMigration(db *sql.DB, migration Migration) error {
	content, err := migrationSQL(migration)
	if err != nil {
		return err
	}
	shouldRun, err := migrationGuardsPass(db, content)
	if err != nil {
		return fmt.Errorf("migration %s guard: %w", migration.Version, err)
	}
	if _, err := db.Exec(`INSERT OR REPLACE INTO schema_migrations (version, name, checksum, dirty, applied_at) VALUES (?, ?, ?, 1, datetime('now'))`, migration.Version, migration.Name, migration.Checksum); err != nil {
		return fmt.Errorf("mark migration %s dirty: %w", migration.Version, err)
	}
	if shouldRun && strings.TrimSpace(content) != "" {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.Version, err)
		}
		if _, err := tx.Exec(content); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s %s: %w", migration.Version, migration.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.Version, err)
		}
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET dirty = 0, applied_at = datetime('now') WHERE version = ?`, migration.Version); err != nil {
		return fmt.Errorf("mark migration %s applied: %w", migration.Version, err)
	}
	return nil
}

func checkAgainstMigrations(db *sql.DB, migrations []Migration) (Status, error) {
	status := Status{LatestVersion: latestVersion(migrations), State: StateUpToDate}
	applied, err := readApplied(db)
	if err != nil {
		return status, err
	}
	known := map[string]Migration{}
	for _, migration := range migrations {
		known[migration.Version] = migration
	}
	appliedByVersion := map[string]AppliedMigration{}
	for _, row := range applied {
		migration, ok := known[row.Version]
		if !ok {
			status.State = StateInconsistent
			status.Message = fmt.Sprintf("database has unknown migration %s", row.Version)
			return status, &InconsistentError{Reason: status.Message}
		}
		if row.Dirty {
			status.State = StateInconsistent
			status.Message = fmt.Sprintf("database has dirty migration %s", row.Version)
			return status, &InconsistentError{Reason: status.Message}
		}
		if row.Checksum != migration.Checksum {
			status.State = StateInconsistent
			status.Message = fmt.Sprintf("database migration %s checksum changed", row.Version)
			return status, &InconsistentError{Reason: status.Message}
		}
		appliedByVersion[row.Version] = row
		status.Applied = append(status.Applied, row)
	}
	for _, migration := range migrations {
		if _, ok := appliedByVersion[migration.Version]; !ok {
			status.Pending = append(status.Pending, migration)
		}
	}
	if len(status.Applied) > 0 {
		status.CurrentVersion = status.Applied[len(status.Applied)-1].Version
	}
	if len(status.Pending) > 0 {
		status.State = StatePending
		status.Message = "database requires migration; run `mnemo db migrate`"
	} else {
		status.State = StateUpToDate
		status.Message = "database schema is up to date"
	}
	return status, nil
}

func readApplied(db *sql.DB) ([]AppliedMigration, error) {
	if !tableExists(db, "schema_migrations") {
		return nil, nil
	}
	rows, err := db.Query(`SELECT version, name, checksum, dirty, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	var applied []AppliedMigration
	for rows.Next() {
		var row AppliedMigration
		var dirty int
		if err := rows.Scan(&row.Version, &row.Name, &row.Checksum, &dirty, &row.AppliedAt); err != nil {
			return nil, err
		}
		row.Dirty = dirty != 0
		applied = append(applied, row)
	}
	return applied, rows.Err()
}

func ValidateCurrent(db *sql.DB) error {
	spec := parseSchemaSpec(dbfiles.Schema)
	for _, table := range spec.Tables {
		if !tableExists(db, table.Name) {
			return &InconsistentError{Reason: "missing table " + table.Name}
		}
		cols, err := tableColumns(db, table.Name)
		if err != nil {
			return err
		}
		for _, col := range table.Columns {
			if !cols[col] {
				return &InconsistentError{Reason: fmt.Sprintf("missing column %s.%s", table.Name, col)}
			}
		}
	}
	for _, object := range spec.Objects {
		if !objectExists(db, object) {
			return &InconsistentError{Reason: "missing schema object " + object}
		}
	}
	return nil
}

type schemaSpec struct {
	Tables  []tableSpec
	Objects []string
}

type tableSpec struct {
	Name    string
	Columns []string
}

var createTablePattern = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*?)\);`)
var createObjectPattern = regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?(?:VIRTUAL\s+TABLE|INDEX|TRIGGER)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
var migrationFilePattern = regexp.MustCompile(`^(\d{4})-([a-z0-9][a-z0-9-]*\.sql)$`)
var sqliteIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func parseSchemaSpec(sqlText string) schemaSpec {
	var spec schemaSpec
	for _, match := range createTablePattern.FindAllStringSubmatch(sqlText, -1) {
		table := tableSpec{Name: match[1]}
		for _, line := range strings.Split(match[2], "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, ","))
			if line == "" {
				continue
			}
			upper := strings.ToUpper(line)
			if strings.HasPrefix(upper, "FOREIGN") || strings.HasPrefix(upper, "PRIMARY") || strings.HasPrefix(upper, "UNIQUE") || strings.HasPrefix(upper, "CONSTRAINT") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) > 0 {
				table.Columns = append(table.Columns, strings.Trim(fields[0], "`\"[]"))
			}
		}
		spec.Tables = append(spec.Tables, table)
	}
	for _, match := range createObjectPattern.FindAllStringSubmatch(sqlText, -1) {
		spec.Objects = append(spec.Objects, match[1])
	}
	return spec
}

func migrationSQL(migration Migration) (string, error) {
	path := filepath.ToSlash(filepath.Join("migrations", migration.Version+"-"+migration.Name+".sql"))
	content, err := fs.ReadFile(dbfiles.Migrations, path)
	if err != nil {
		return "", fmt.Errorf("read migration %s: %w", path, err)
	}
	return string(content), nil
}

func migrationGuardsPass(db *sql.DB, content string) (bool, error) {
	shouldRun := true
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-- mnemo:when-") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(line, "-- mnemo:"))
		if len(parts) != 3 {
			return false, fmt.Errorf("invalid guard %q", line)
		}
		ok, err := evalGuard(db, parts[0], parts[1], parts[2])
		if err != nil {
			return false, err
		}
		shouldRun = shouldRun && ok
	}
	return shouldRun, nil
}

func evalGuard(db *sql.DB, kind, table, column string) (bool, error) {
	switch kind {
	case "when-column-missing":
		cols, err := tableColumns(db, table)
		if err != nil {
			return false, err
		}
		return !cols[column], nil
	case "when-column-not-primary-key":
		pk, exists, err := columnPrimaryKey(db, table, column)
		if err != nil {
			return false, err
		}
		return exists && pk != 1, nil
	default:
		return false, fmt.Errorf("unknown guard %q", kind)
	}
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	if !sqliteIdentifierPattern.MatchString(table) {
		return nil, fmt.Errorf("invalid table identifier %q", table)
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func columnPrimaryKey(db *sql.DB, table, column string) (int, bool, error) {
	if !sqliteIdentifierPattern.MatchString(table) || !sqliteIdentifierPattern.MatchString(column) {
		return 0, false, fmt.Errorf("invalid table or column identifier")
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return 0, false, err
		}
		if name == column {
			return pk, true, nil
		}
	}
	return 0, false, rows.Err()
}

func tableExists(db *sql.DB, name string) bool {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, name).Scan(&count)
	return count > 0
}

func objectExists(db *sql.DB, name string) bool {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).Scan(&count)
	return count > 0
}

func checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func parseMigrationName(filename string) (string, string, bool) {
	match := migrationFilePattern.FindStringSubmatch(filename)
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

func latestVersion(migrations []Migration) string {
	if len(migrations) == 0 {
		return ""
	}
	return migrations[len(migrations)-1].Version
}

func applyPragmas(db *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}
	return nil
}

func sqliteReadOnlyDBURI(dbPath string) string {
	return "file:" + filepath.ToSlash(dbPath) + "?mode=ro&immutable=1"
}

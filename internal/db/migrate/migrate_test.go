package migrate

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	dbfiles "github.com/jmeiracorbal/mnemo/database"
	_ "modernc.org/sqlite"
)

func TestMigratedSchemaMatchesCanonicalSchemaStructure(t *testing.T) {
	migratedDir := t.TempDir()
	if _, err := ApplyDataDir(migratedDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	migrated := openTestDB(t, filepath.Join(migratedDir, "memory.db"))
	t.Cleanup(func() { _ = migrated.Close() })

	canonical := openTestDB(t, filepath.Join(t.TempDir(), "canonical.db"))
	t.Cleanup(func() { _ = canonical.Close() })
	if _, err := canonical.Exec(dbfiles.Schema); err != nil {
		t.Fatalf("apply canonical schema: %v", err)
	}

	gotObjects := schemaObjects(t, migrated)
	wantObjects := schemaObjects(t, canonical)
	if !reflect.DeepEqual(gotObjects, wantObjects) {
		t.Fatalf("migrated schema objects differ from canonical schema\ngot:  %#v\nwant: %#v", gotObjects, wantObjects)
	}

	for _, table := range schemaTables(t, canonical) {
		gotCols := columnList(t, migrated, table)
		wantCols := columnList(t, canonical, table)
		if !reflect.DeepEqual(gotCols, wantCols) {
			t.Fatalf("columns for %s differ\ngot:  %#v\nwant: %#v", table, gotCols, wantCols)
		}
	}
}

func TestColumnAlterMigrationsHaveIdempotenceGuards(t *testing.T) {
	for _, migration := range []string{
		"0002-add-observations-sync-id.sql",
		"0003-add-observations-scope.sql",
		"0004-add-observations-topic-key.sql",
		"0005-add-observations-normalized-hash.sql",
		"0006-add-observations-revision-count.sql",
		"0007-add-observations-duplicate-count.sql",
		"0008-add-observations-last-seen-at.sql",
		"0009-add-observations-updated-at.sql",
		"0010-add-observations-deleted-at.sql",
		"0011-add-observations-provenance-id.sql",
		"0012-add-sessions-provenance-id.sql",
		"0013-add-user-prompts-sync-id.sql",
		"0014-add-user-prompts-project.sql",
		"0015-add-user-prompts-provenance-id.sql",
		"0016-add-sync-mutations-project.sql",
	} {
		content, err := migrationSQL(Migration{Version: migration[:4], Name: strings.TrimSuffix(migration[5:], ".sql")})
		if err != nil {
			t.Fatalf("read migration %s: %v", migration, err)
		}
		if strings.Contains(strings.ToUpper(content), "ALTER TABLE") && !strings.Contains(content, "-- mnemo:when-column-missing") {
			t.Fatalf("migration %s has ALTER TABLE without an idempotence guard", migration)
		}
	}
}

func TestApplyDataDirCreatesCurrentSchemaAndIsIdempotent(t *testing.T) {
	dataDir := t.TempDir()
	first, err := ApplyDataDir(dataDir)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if first.LatestVersion != "0024" {
		t.Fatalf("latest version = %q, want 0024", first.LatestVersion)
	}
	second, err := ApplyDataDir(dataDir)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if second.State != StateUpToDate || len(second.Pending) != 0 {
		t.Fatalf("second apply status = %+v, want up to date", second)
	}

	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	t.Cleanup(func() { _ = db.Close() })
	if err := ValidateCurrent(db); err != nil {
		t.Fatalf("validate current schema: %v", err)
	}
}

func TestProjectColumnsReferenceProjects(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := ApplyDataDir(dataDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	t.Cleanup(func() { _ = db.Close() })

	for _, tc := range []struct {
		table string
		from  string
	}{
		{table: "sessions", from: "project"},
		{table: "sync_mutations", from: "project"},
		{table: "sync_enrolled_projects", from: "project"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			fks := foreignKeys(t, db, tc.table)
			for _, fk := range fks {
				if fk.From == tc.from && fk.Table == "projects" && fk.To == "id" {
					return
				}
			}
			t.Fatalf("%s.%s does not reference projects(id): %+v", tc.table, tc.from, fks)
		})
	}
}

func TestMemoryProjectColumnsAreDerivedFromSessions(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := ApplyDataDir(dataDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{"observations", "user_prompts"} {
		for _, col := range columnList(t, db, table) {
			if col == "project" {
				t.Fatalf("%s.project should not be stored; project is derived from sessions.project", table)
			}
		}
	}

	for _, table := range []string{"observations_fts", "user_prompts_fts"} {
		for _, col := range columnList(t, db, table) {
			if col == "project" {
				t.Fatalf("%s.project should not be indexed in FTS; project filtering joins sessions", table)
			}
		}
	}
}

func TestApplyDataDirUpgradesPartialLegacyDB(t *testing.T) {
	dataDir := t.TempDir()
	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	_, err := db.Exec(`
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			directory TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			summary TEXT
		);
		CREATE TABLE observations (
			id INT,
			session_id TEXT NOT NULL,
			type TEXT,
			title TEXT,
			content TEXT,
			tool_name TEXT,
			project TEXT,
			created_at TEXT
		);
		CREATE TABLE user_prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO sessions (id, project, directory) VALUES ('s1', 'legacy', '/tmp/legacy');
		INSERT INTO observations (id, session_id, type, title, content, project, created_at)
		VALUES (1, 's1', '', '', 'legacy content', 'legacy', '');
		INSERT INTO user_prompts (session_id, content) VALUES ('s1', 'legacy prompt');
	`)
	if err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	status, err := ApplyDataDir(dataDir)
	if err != nil {
		t.Fatalf("apply legacy migrations: %v", err)
	}
	if status.State != StateApplied && status.State != StateUpToDate {
		t.Fatalf("unexpected status: %+v", status)
	}

	db = openTestDB(t, filepath.Join(dataDir, "memory.db"))
	t.Cleanup(func() { _ = db.Close() })
	if err := ValidateCurrent(db); err != nil {
		t.Fatalf("validate migrated legacy schema: %v", err)
	}
	var projectRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects WHERE id = 'legacy'`).Scan(&projectRows); err != nil {
		t.Fatalf("query migrated project root: %v", err)
	}
	if projectRows != 1 {
		t.Fatalf("legacy project root rows = %d, want 1", projectRows)
	}
	var pk int
	if err := db.QueryRow(`SELECT pk FROM pragma_table_info('observations') WHERE name = 'id'`).Scan(&pk); err != nil {
		t.Fatalf("query observations id pk: %v", err)
	}
	if pk != 1 {
		t.Fatalf("observations.id pk = %d, want 1", pk)
	}
	var title, syncID, promptSyncID string
	if err := db.QueryRow(`SELECT title, sync_id FROM observations WHERE content = 'legacy content'`).Scan(&title, &syncID); err != nil {
		t.Fatalf("query migrated observation: %v", err)
	}
	if title != "Untitled observation" || syncID == "" {
		t.Fatalf("unexpected migrated observation title=%q sync_id=%q", title, syncID)
	}
	if err := db.QueryRow(`SELECT sync_id FROM user_prompts WHERE content = 'legacy prompt'`).Scan(&promptSyncID); err != nil {
		t.Fatalf("query migrated prompt: %v", err)
	}
	if promptSyncID == "" {
		t.Fatal("prompt sync_id was not backfilled")
	}
}

func TestCheckDataDirReportsPendingForUnversionedDatabase(t *testing.T) {
	dataDir := t.TempDir()
	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	if _, err := db.Exec(`CREATE TABLE sessions (id TEXT)`); err != nil {
		t.Fatalf("seed unversioned db: %v", err)
	}
	_ = db.Close()

	status, err := CheckDataDir(dataDir)
	if err != nil {
		t.Fatalf("check unversioned db: %v", err)
	}
	if status.State != StatePending || len(status.Pending) == 0 {
		t.Fatalf("status = %+v, want pending migrations", status)
	}
}

func TestApplyRejectsDirtyChecksumAndFutureMigration(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := ApplyDataDir(dataDir); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	if _, err := db.Exec(`UPDATE schema_migrations SET dirty = 1 WHERE version = '0001'`); err != nil {
		t.Fatalf("mark dirty: %v", err)
	}
	_ = db.Close()
	if _, err := ApplyDataDir(dataDir); err == nil || !strings.Contains(err.Error(), "dirty migration 0001") {
		t.Fatalf("expected dirty migration error, got %v", err)
	}

	dataDir = t.TempDir()
	if _, err := ApplyDataDir(dataDir); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	db = openTestDB(t, filepath.Join(dataDir, "memory.db"))
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, checksum, dirty) VALUES ('9999', 'future', 'sha256:future', 0)`); err != nil {
		t.Fatalf("insert future migration: %v", err)
	}
	_ = db.Close()
	if _, err := ApplyDataDir(dataDir); err == nil || !strings.Contains(err.Error(), "unknown migration 9999") {
		t.Fatalf("expected future migration error, got %v", err)
	}
}

func TestValidateCurrentDetectsMissingSchemaObject(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := ApplyDataDir(dataDir); err != nil {
		t.Fatalf("apply current schema: %v", err)
	}
	db := openTestDB(t, filepath.Join(dataDir, "memory.db"))
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DROP TRIGGER IF EXISTS obs_fts_insert`); err != nil {
		t.Fatalf("drop trigger: %v", err)
	}
	if err := ValidateCurrent(db); err == nil || !strings.Contains(err.Error(), "obs_fts_insert") {
		t.Fatalf("expected missing trigger validation error, got %v", err)
	}
}

func schemaObjects(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT type || ':' || name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name NOT GLOB '*_data' AND name NOT GLOB '*_idx' AND name NOT GLOB '*_docsize' AND name NOT GLOB '*_config' ORDER BY 1`)
	if err != nil {
		t.Fatalf("query schema objects: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan schema object: %v", err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema object rows: %v", err)
	}
	return out
}

func schemaTables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT GLOB '*_data' AND name NOT GLOB '*_idx' AND name NOT GLOB '*_docsize' AND name NOT GLOB '*_config' ORDER BY name`)
	if err != nil {
		t.Fatalf("query schema tables: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan schema table: %v", err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema table rows: %v", err)
	}
	return out
}

func columnList(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name || ':' || type || ':' || "notnull" || ':' || pk FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatalf("query columns for %s: %v", table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan column for %s: %v", table, err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("column rows for %s: %v", table, err)
	}
	sort.Strings(out)
	return out
}

type foreignKey struct {
	Table string
	From  string
	To    string
}

func foreignKeys(t *testing.T, db *sql.DB, table string) []foreignKey {
	t.Helper()
	rows, err := db.Query(`SELECT "table", "from", "to" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, table)
	if err != nil {
		t.Fatalf("query foreign keys for %s: %v", table, err)
	}
	defer rows.Close()
	var out []foreignKey
	for rows.Next() {
		var fk foreignKey
		if err := rows.Scan(&fk.Table, &fk.From, &fk.To); err != nil {
			t.Fatalf("scan foreign key for %s: %v", table, err)
		}
		out = append(out, fk)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("foreign key rows for %s: %v", table, err)
	}
	return out
}

func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

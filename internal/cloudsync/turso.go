package cloudsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"

	dbfiles "github.com/jmeiracorbal/mnemo/database"
)

// TursoBackend implements CloudBackend against a Turso/libSQL database using the
// Hrana v2 HTTP pipeline protocol. It executes the same SQLite SQL as the local
// store, so the cloud database is an exact schema mirror of local.
type TursoBackend struct {
	httpURL  string
	token    string
	clientID string
	client   *http.Client
}

// NewTursoBackend builds a TursoBackend from cfg.
// cfg.URL may be libsql:// or https://.
func NewTursoBackend(cfg Config) (*TursoBackend, error) {
	validated, err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	httpURL := strings.Replace(validated.URL, "libsql://", "https://", 1)
	return &TursoBackend{
		httpURL:  strings.TrimRight(httpURL, "/"),
		token:    validated.Key,
		clientID: validated.ClientID,
		client:   &http.Client{Timeout: validated.Timeout},
	}, nil
}

// Migrate runs local SQLite migrations against the Turso database and then
// applies the cloud-only extensions needed for multi-client sync
// (origin_id + client_seq columns on sync_mutations).
// It is idempotent and safe to call on every startup.
func (b *TursoBackend) Migrate() error {
	return b.migrate(dbfiles.Migrations)
}

func (b *TursoBackend) migrate(migrations fs.FS) error {
	// 1. Ensure schema_migrations table exists.
	if _, err := b.pipeline([]hranaStmt{{SQL: `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL,
		dirty INTEGER NOT NULL DEFAULT 0,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`}}); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// 2. Read which migrations have already been applied.
	results, err := b.pipeline([]hranaStmt{{SQL: "SELECT version FROM schema_migrations"}})
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	applied := map[string]bool{}
	if len(results) > 0 {
		for _, row := range results[0].Rows {
			if len(row) > 0 {
				applied[row[0].Value] = true
			}
		}
	}

	// 3. Apply pending migrations in order.
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".sql")
		if applied[version] {
			continue
		}
		sqlBytes, err := fs.ReadFile(migrations, "migrations/"+entry.Name())
		if err != nil {
			return err
		}
		hash := sha256.Sum256(sqlBytes)
		checksum := hex.EncodeToString(hash[:])

		// Evaluate -- mnemo:when-* guards before executing SQL. If guards fail the
		// migration is skipped but still recorded so it is not retried.
		shouldRun, err := b.migrateGuardsPass(string(sqlBytes))
		if err != nil {
			return fmt.Errorf("migration %s guard: %w", version, err)
		}

		batch := []hranaStmt{{SQL: "BEGIN"}}
		if shouldRun {
			for _, stmt := range splitSQL(string(sqlBytes)) {
				batch = append(batch, hranaStmt{SQL: stmt})
			}
		}
		batch = append(batch,
			hranaStmt{
				SQL:  `INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)`,
				Args: []hranaValue{textVal(version), textVal(entry.Name()), textVal(checksum)},
			},
			hranaStmt{SQL: "COMMIT"},
		)
		if _, err := b.pipeline(batch); err != nil {
			return fmt.Errorf("migration %s: %w", version, err)
		}
	}

	// 4. Cloud-only extensions: origin_id + client_seq on sync_mutations for
	//    multi-client idempotent dedup. These are not in the local schema.
	b.addColumnIfMissing("sync_mutations", "origin_id", "TEXT NOT NULL DEFAULT ''")
	b.addColumnIfMissing("sync_mutations", "client_seq", "INTEGER NOT NULL DEFAULT 0")
	if _, err := b.pipeline([]hranaStmt{{
		SQL: `CREATE UNIQUE INDEX IF NOT EXISTS ux_sync_mutations_cloud ON sync_mutations(origin_id, client_seq) WHERE origin_id != ''`,
	}}); err != nil {
		return fmt.Errorf("create cloud dedup index: %w", err)
	}

	// 5. Ensure sync_state has a cloud target row (needed for sync_mutations FK).
	if _, err := b.pipeline([]hranaStmt{{
		SQL:  `INSERT OR IGNORE INTO sync_state (target_key, lifecycle) VALUES (?, 'idle')`,
		Args: []hranaValue{textVal("cloud")},
	}}); err != nil {
		return fmt.Errorf("ensure sync_state: %w", err)
	}

	return nil
}

// PushMutations applies mutation payloads to the cloud data tables and records
// them in sync_mutations for pull by other clients.
// The operation is idempotent: INSERT OR REPLACE for data rows and
// INSERT OR IGNORE (via unique index) for journal rows.
func (b *TursoBackend) PushMutations(entries []MutationEntry) (*PushResult, error) {
	if len(entries) == 0 {
		return &PushResult{}, nil
	}

	stmts := []hranaStmt{{SQL: "BEGIN"}}

	for _, e := range entries {
		datStmts, err := b.mutationToSQL(e)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, datStmts...)

		// Record in cloud journal for pull by other clients.
		stmts = append(stmts, hranaStmt{
			SQL: `INSERT OR IGNORE INTO sync_mutations
				(target_key, entity, entity_key, op, payload, source, project, origin_id, client_seq, occurred_at)
				VALUES (?, ?, ?, ?, ?, 'remote', ?, ?, ?, ?)`,
			Args: []hranaValue{
				textVal("cloud"), textVal(e.Entity), textVal(e.EntityKey), textVal(e.Op),
				textVal(string(e.Payload)), textVal(e.Project),
				textVal(b.clientID), intVal(e.LocalSeq), textVal(e.OccurredAt),
			},
		})
	}

	stmts = append(stmts, hranaStmt{SQL: "COMMIT"})

	if _, err := b.pipeline(stmts); err != nil {
		return nil, fmt.Errorf("cloud sync push: %w", err)
	}

	out := make([]int64, len(entries))
	for i, e := range entries {
		out[i] = e.LocalSeq
	}
	return &PushResult{AcceptedSeqs: out}, nil
}

// mutationToSQL converts a single MutationEntry into the SQL statements that
// apply it to the cloud data tables.
func (b *TursoBackend) mutationToSQL(e MutationEntry) ([]hranaStmt, error) {
	switch e.Entity {
	case "session":
		var p mutationSessionPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode session payload seq=%d: %w", e.LocalSeq, err)
		}
		stmts := []hranaStmt{{
			SQL: `INSERT OR REPLACE INTO sessions (id, project, directory, ended_at, summary)
				VALUES (?, ?, ?, ?, ?)`,
			Args: []hranaValue{textVal(p.ID), textVal(p.Project), textVal(p.Directory),
				maybeTextVal(p.EndedAt), maybeTextVal(p.Summary)},
		}}
		// INSERT OR REPLACE cascades-deletes existing session_tags; re-insert new ones.
		if p.Tags != nil {
			for _, tag := range *p.Tags {
				stmts = append(stmts, hranaStmt{
					SQL:  `INSERT OR IGNORE INTO session_tags (session_id, tag) VALUES (?, ?)`,
					Args: []hranaValue{textVal(p.ID), textVal(tag)},
				})
			}
		}
		return stmts, nil

	case "observation":
		var p mutationObservationPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode observation payload seq=%d: %w", e.LocalSeq, err)
		}
		if e.Op == "delete" || p.Deleted {
			deletedAt := p.DeletedAt
			if deletedAt == nil && e.OccurredAt != "" {
				deletedAt = &e.OccurredAt
			}
			return []hranaStmt{{
				SQL:  `UPDATE observations SET deleted_at = ? WHERE sync_id = ?`,
				Args: []hranaValue{maybeTextVal(deletedAt), textVal(p.SyncID)},
			}}, nil
		}
		scope := p.Scope
		if scope == "" {
			scope = "project"
		}
		stmts := []hranaStmt{{
			// INSERT OR REPLACE cascades-deletes existing observation_tags.
			SQL: `INSERT OR REPLACE INTO observations
				(sync_id, session_id, type, title, content, tool_name, project, scope, topic_key)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			Args: []hranaValue{
				textVal(p.SyncID), textVal(p.SessionID), textVal(p.Type),
				textVal(p.Title), textVal(p.Content), maybeTextVal(p.ToolName),
				maybeTextVal(p.Project), textVal(scope), maybeTextVal(p.TopicKey),
			},
		}}
		// Re-insert tags via subquery (no integer id needed).
		if p.Tags != nil {
			for _, tag := range *p.Tags {
				stmts = append(stmts, hranaStmt{
					SQL:  `INSERT OR IGNORE INTO observation_tags (observation_id, tag) SELECT id, ? FROM observations WHERE sync_id = ?`,
					Args: []hranaValue{textVal(tag), textVal(p.SyncID)},
				})
			}
		}
		return stmts, nil

	case "prompt":
		var p mutationPromptPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode prompt payload seq=%d: %w", e.LocalSeq, err)
		}
		return []hranaStmt{{
			SQL:  `INSERT OR REPLACE INTO user_prompts (sync_id, session_id, content, project) VALUES (?, ?, ?, ?)`,
			Args: []hranaValue{textVal(p.SyncID), textVal(p.SessionID), textVal(p.Content), maybeTextVal(p.Project)},
		}}, nil

	default:
		return nil, fmt.Errorf("unknown entity %q", e.Entity)
	}
}

// PullMutations returns mutations from other clients that have a cloud seq
// greater than sinceSeq.
func (b *TursoBackend) PullMutations(sinceSeq int64, limit int) (*PullResult, error) {
	if limit <= 0 {
		limit = 100
	}
	results, err := b.pipeline([]hranaStmt{{
		SQL: `SELECT seq, origin_id, client_seq, project, entity, entity_key, op, payload, occurred_at
			FROM sync_mutations
			WHERE seq > ? AND origin_id != '' AND origin_id != ?
			ORDER BY seq ASC LIMIT ?`,
		Args: []hranaValue{intVal(sinceSeq), textVal(b.clientID), intVal(int64(limit))},
	}})
	if err != nil {
		return nil, fmt.Errorf("cloud sync pull: %w", err)
	}

	var mutations []PulledMutation
	latest := sinceSeq
	if len(results) > 0 {
		for _, row := range results[0].Rows {
			if len(row) < 9 {
				continue
			}
			seq, _ := strconv.ParseInt(row[0].Value, 10, 64)
			clientSeq, _ := strconv.ParseInt(row[2].Value, 10, 64)
			if seq > latest {
				latest = seq
			}
			mutations = append(mutations, PulledMutation{
				Seq: seq, OriginID: row[1].Value, ClientSeq: clientSeq,
				Project: row[3].Value, Entity: row[4].Value, EntityKey: row[5].Value,
				Op: row[6].Value, Payload: json.RawMessage(row[7].Value), OccurredAt: row[8].Value,
			})
		}
	}
	return &PullResult{Mutations: mutations, HasMore: len(mutations) == limit, LatestSeq: latest}, nil
}

// ---------------------------------------------------------------------------
// Hrana v2 HTTP protocol
// ---------------------------------------------------------------------------

type hranaStmt struct {
	SQL  string       `json:"sql"`
	Args []hranaValue `json:"args,omitempty"`
}

type hranaValue struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

// MarshalJSON ensures null values serialize as {"type":"null"} (no value field)
// and all other types always include "value", even when it is an empty string.
// The struct tag omitempty alone would drop "value":"" for text/integer types,
// causing Turso's Hrana parser to report "missing field value".
func (v hranaValue) MarshalJSON() ([]byte, error) {
	if v.Type == "null" {
		return []byte(`{"type":"null"}`), nil
	}
	return json.Marshal(struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}{Type: v.Type, Value: v.Value})
}

type hranaRequest struct {
	Type string     `json:"type"`
	Stmt *hranaStmt `json:"stmt,omitempty"`
}

type hranaCol struct {
	Name     string  `json:"name"`
	DeclType *string `json:"decltype"`
}

type hranaExecResult struct {
	Cols            []hranaCol     `json:"cols"`
	Rows            [][]hranaValue `json:"rows"`
	RowsAffected    int64          `json:"rows_affected"`
	LastInsertRowID *string        `json:"last_insert_rowid"`
}

type hranaResultItem struct {
	Type     string `json:"type"`
	Response *struct {
		Type   string           `json:"type"`
		Result *hranaExecResult `json:"result,omitempty"`
	} `json:"response,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (b *TursoBackend) pipeline(stmts []hranaStmt) ([]hranaExecResult, error) {
	reqs := make([]hranaRequest, 0, len(stmts)+1)
	for i := range stmts {
		reqs = append(reqs, hranaRequest{Type: "execute", Stmt: &stmts[i]})
	}
	reqs = append(reqs, hranaRequest{Type: "close"})

	body, err := json.Marshal(map[string]any{"requests": reqs})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, b.httpURL+"/v2/pipeline", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("turso: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("turso: pipeline returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var pr struct {
		Results []hranaResultItem `json:"results"`
	}
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return nil, fmt.Errorf("turso: decode response: %w: %s", err, strings.TrimSpace(string(respBody)))
	}

	var out []hranaExecResult
	for i, r := range pr.Results {
		if r.Type == "error" {
			msg := ""
			if r.Error != nil {
				msg = r.Error.Message
			}
			return nil, fmt.Errorf("turso: statement %d failed: %s", i, msg)
		}
		if r.Response != nil && r.Response.Result != nil {
			out = append(out, *r.Response.Result)
		} else {
			out = append(out, hranaExecResult{})
		}
	}
	return out, nil
}

// migrateGuardsPass parses -- mnemo:when-* directives from a migration file and
// evaluates each against the remote schema. Returns false if any guard says the
// migration should be skipped (column already exists, etc.).
func (b *TursoBackend) migrateGuardsPass(content string) (bool, error) {
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
		ok, err := b.evalMigrateGuard(parts[0], parts[1], parts[2])
		if err != nil {
			return false, err
		}
		shouldRun = shouldRun && ok
	}
	return shouldRun, nil
}

// evalMigrateGuard evaluates a single migration guard directive against the remote schema.
func (b *TursoBackend) evalMigrateGuard(kind, table, column string) (bool, error) {
	results, err := b.pipeline([]hranaStmt{{SQL: "PRAGMA table_info(" + table + ")"}})
	if err != nil {
		return false, err
	}
	switch kind {
	case "when-column-missing":
		if len(results) > 0 {
			for _, row := range results[0].Rows {
				// PRAGMA table_info: cid, name, type, notnull, dflt_value, pk
				if len(row) > 1 && row[1].Value == column {
					return false, nil // column exists → skip
				}
			}
		}
		return true, nil // column absent → run
	case "when-column-not-primary-key":
		if len(results) > 0 {
			for _, row := range results[0].Rows {
				if len(row) > 5 && row[1].Value == column {
					pk, _ := strconv.ParseInt(row[5].Value, 10, 64)
					return pk != 1, nil // true when NOT a primary key → run
				}
			}
		}
		return false, nil // column not found → skip
	default:
		return false, fmt.Errorf("unknown migration guard %q", kind)
	}
}

// addColumnIfMissing adds colName colDef to table if the column does not exist.
// Errors are silently swallowed — the column either already exists or the main
// migration will fail with a clearer message.
func (b *TursoBackend) addColumnIfMissing(table, colName, colDef string) {
	results, err := b.pipeline([]hranaStmt{{SQL: "PRAGMA table_info(" + table + ")"}})
	if err != nil {
		return
	}
	if len(results) > 0 {
		for _, row := range results[0].Rows {
			// PRAGMA table_info: cid, name, type, notnull, dflt_value, pk
			if len(row) > 1 && row[1].Value == colName {
				return
			}
		}
	}
	_, _ = b.pipeline([]hranaStmt{{SQL: "ALTER TABLE " + table + " ADD COLUMN " + colName + " " + colDef}})
}

// ---------------------------------------------------------------------------
// Hrana value helpers
// ---------------------------------------------------------------------------

func textVal(s string) hranaValue        { return hranaValue{Type: "text", Value: s} }
func intVal(n int64) hranaValue          { return hranaValue{Type: "integer", Value: strconv.FormatInt(n, 10)} }
func nullVal() hranaValue                { return hranaValue{Type: "null"} }
func maybeTextVal(s *string) hranaValue  {
	if s == nil {
		return nullVal()
	}
	return textVal(*s)
}

// ---------------------------------------------------------------------------
// SQL statement splitter
// ---------------------------------------------------------------------------

// splitSQL splits a SQL script into individual statements, correctly handling
// SQLite triggers (BEGIN/END) and single-quoted string literals.
func splitSQL(script string) []string {
	var stmts []string
	var cur strings.Builder
	depth := 0 // tracks BEGIN/END inside triggers
	inStr := false

	for i := 0; i < len(script); i++ {
		ch := script[i]

		// Inside a single-quoted string literal.
		if inStr {
			cur.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					// Escaped quote: ''
					cur.WriteByte(script[i+1])
					i++
				} else {
					inStr = false
				}
			}
			continue
		}

		// Line comment: skip to end of line.
		if ch == '-' && i+1 < len(script) && script[i+1] == '-' {
			for i < len(script) && script[i] != '\n' {
				i++
			}
			cur.WriteByte('\n')
			continue
		}

		if ch == '\'' {
			inStr = true
			cur.WriteByte(ch)
			continue
		}

		// Track BEGIN/END depth for trigger bodies.
		up := strings.ToUpper(script[i:])
		if strings.HasPrefix(up, "BEGIN") && !isWordChar(script, i+5) &&
			strings.Contains(strings.ToUpper(cur.String()), "TRIGGER") {
			depth++
		} else if strings.HasPrefix(up, "END") && !isWordChar(script, i+3) && depth > 0 {
			depth--
		}

		cur.WriteByte(ch)

		if ch == ';' && depth == 0 {
			if stmt := strings.TrimSpace(cur.String()); stmt != "" && stmt != ";" {
				stmts = append(stmts, stmt)
			}
			cur.Reset()
		}
	}
	if stmt := strings.TrimSpace(cur.String()); stmt != "" && stmt != ";" {
		stmts = append(stmts, stmt)
	}
	return stmts
}

func isWordChar(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	c := s[i]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

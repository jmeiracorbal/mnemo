package turso

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
	"time"

	dbfiles "github.com/jmeiracorbal/mnemo/database"
	"github.com/jmeiracorbal/mnemo/internal/cloudsync"
)

// Backend implements cloudsync.CloudBackend against a Turso/libSQL database
// using the Hrana v2 HTTP pipeline protocol. It executes the same SQLite SQL
// as the local store, so the cloud database is an exact schema mirror of local.
type Backend struct {
	httpURL  string
	token    string
	clientID string
	client   *http.Client
}

// New builds a Backend from cfg. cfg.URL may be libsql:// or https://.
func New(cfg cloudsync.Config) (*Backend, error) {
	validated, err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	httpURL := strings.Replace(validated.URL, "libsql://", "https://", 1)
	timeout := validated.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Backend{
		httpURL:  strings.TrimRight(httpURL, "/"),
		token:    validated.Key,
		clientID: validated.ClientID,
		client:   &http.Client{Timeout: timeout},
	}, nil
}

// Migrate runs local SQLite migrations against the Turso database and then
// applies cloud-only extensions (origin_id + client_seq on sync_mutations).
// It is idempotent and safe to call on every startup.
func (b *Backend) Migrate() error {
	return b.migrate(dbfiles.Migrations)
}

func (b *Backend) migrate(migrations fs.FS) error {
	if _, err := b.pipeline([]hranaStmt{{SQL: `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL,
		dirty INTEGER NOT NULL DEFAULT 0,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`}}); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

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

	b.addColumnIfMissing("sync_mutations", "origin_id", "TEXT NOT NULL DEFAULT ''")
	b.addColumnIfMissing("sync_mutations", "client_seq", "INTEGER NOT NULL DEFAULT 0")
	if _, err := b.pipeline([]hranaStmt{{
		SQL: `CREATE UNIQUE INDEX IF NOT EXISTS ux_sync_mutations_cloud ON sync_mutations(origin_id, client_seq) WHERE origin_id != ''`,
	}}); err != nil {
		return fmt.Errorf("create cloud dedup index: %w", err)
	}

	if _, err := b.pipeline([]hranaStmt{{
		SQL:  `INSERT OR IGNORE INTO sync_state (target_key, lifecycle) VALUES (?, 'idle')`,
		Args: []hranaValue{textVal("cloud")},
	}}); err != nil {
		return fmt.Errorf("ensure sync_state: %w", err)
	}

	return nil
}

// PushMutations applies mutation payloads to the cloud data tables and records
// them in sync_mutations. The operation is idempotent.
func (b *Backend) PushMutations(entries []cloudsync.MutationEntry) (*cloudsync.PushResult, error) {
	if len(entries) == 0 {
		return &cloudsync.PushResult{}, nil
	}

	stmts := []hranaStmt{{SQL: "BEGIN"}}

	for _, e := range entries {
		datStmts, err := b.mutationToSQL(e)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, datStmts...)

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
	return &cloudsync.PushResult{AcceptedSeqs: out}, nil
}

func (b *Backend) mutationToSQL(e cloudsync.MutationEntry) ([]hranaStmt, error) {
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
			SQL: `INSERT OR REPLACE INTO observations
				(sync_id, session_id, type, title, content, tool_name, project, scope, topic_key)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			Args: []hranaValue{
				textVal(p.SyncID), textVal(p.SessionID), textVal(p.Type),
				textVal(p.Title), textVal(p.Content), maybeTextVal(p.ToolName),
				maybeTextVal(p.Project), textVal(scope), maybeTextVal(p.TopicKey),
			},
		}}
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

// PullMutations returns mutations from other clients with cloud seq > sinceSeq.
func (b *Backend) PullMutations(sinceSeq int64, limit int) (*cloudsync.PullResult, error) {
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

	var mutations []cloudsync.PulledMutation
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
			mutations = append(mutations, cloudsync.PulledMutation{
				Seq: seq, OriginID: row[1].Value, ClientSeq: clientSeq,
				Project: row[3].Value, Entity: row[4].Value, EntityKey: row[5].Value,
				Op: row[6].Value, Payload: json.RawMessage(row[7].Value), OccurredAt: row[8].Value,
			})
		}
	}
	return &cloudsync.PullResult{Mutations: mutations, HasMore: len(mutations) == limit, LatestSeq: latest}, nil
}

// ---------------------------------------------------------------------------
// Mutation payload types (internal to Turso SQL mapping)
// ---------------------------------------------------------------------------

type mutationSessionPayload struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	Directory string    `json:"directory"`
	EndedAt   *string   `json:"ended_at,omitempty"`
	Summary   *string   `json:"summary,omitempty"`
	Tags      *[]string `json:"tags,omitempty"`
}

type mutationObservationPayload struct {
	SyncID    string    `json:"sync_id"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	ToolName  *string   `json:"tool_name,omitempty"`
	Project   *string   `json:"project,omitempty"`
	Scope     string    `json:"scope"`
	TopicKey  *string   `json:"topic_key,omitempty"`
	Tags      *[]string `json:"tags,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`
	DeletedAt *string   `json:"deleted_at,omitempty"`
}

type mutationPromptPayload struct {
	SyncID    string  `json:"sync_id"`
	SessionID string  `json:"session_id"`
	Content   string  `json:"content"`
	Project   *string `json:"project,omitempty"`
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

func (b *Backend) pipeline(stmts []hranaStmt) ([]hranaExecResult, error) {
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
	defer func() { _ = resp.Body.Close() }()
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

func (b *Backend) migrateGuardsPass(content string) (bool, error) {
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

func (b *Backend) evalMigrateGuard(kind, table, column string) (bool, error) {
	results, err := b.pipeline([]hranaStmt{{SQL: "PRAGMA table_info(" + table + ")"}})
	if err != nil {
		return false, err
	}
	switch kind {
	case "when-column-missing":
		if len(results) > 0 {
			for _, row := range results[0].Rows {
				if len(row) > 1 && row[1].Value == column {
					return false, nil
				}
			}
		}
		return true, nil
	case "when-column-not-primary-key":
		if len(results) > 0 {
			for _, row := range results[0].Rows {
				if len(row) > 5 && row[1].Value == column {
					pk, _ := strconv.ParseInt(row[5].Value, 10, 64)
					return pk != 1, nil
				}
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown migration guard %q", kind)
	}
}

func (b *Backend) addColumnIfMissing(table, colName, colDef string) {
	results, err := b.pipeline([]hranaStmt{{SQL: "PRAGMA table_info(" + table + ")"}})
	if err != nil {
		return
	}
	if len(results) > 0 {
		for _, row := range results[0].Rows {
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

func textVal(s string) hranaValue       { return hranaValue{Type: "text", Value: s} }
func intVal(n int64) hranaValue         { return hranaValue{Type: "integer", Value: strconv.FormatInt(n, 10)} }
func nullVal() hranaValue               { return hranaValue{Type: "null"} }
func maybeTextVal(s *string) hranaValue {
	if s == nil {
		return nullVal()
	}
	return textVal(*s)
}

// ---------------------------------------------------------------------------
// SQL statement splitter
// ---------------------------------------------------------------------------

func splitSQL(script string) []string {
	var stmts []string
	var cur strings.Builder
	depth := 0
	inStr := false

	for i := 0; i < len(script); i++ {
		ch := script[i]
		if inStr {
			cur.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					cur.WriteByte(script[i+1])
					i++
				} else {
					inStr = false
				}
			}
			continue
		}
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

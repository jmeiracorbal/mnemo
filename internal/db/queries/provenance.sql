-- name: UpsertAgent :exec
INSERT INTO agents (id, display_name, kind)
VALUES (sqlc.arg('id'), sqlc.arg('display_name'), sqlc.arg('kind'))
ON CONFLICT(id) DO UPDATE SET
  display_name = excluded.display_name,
  kind = excluded.kind,
  is_deleted = 0;

-- name: UpsertSourceKind :exec
INSERT INTO source_kinds (id, display_name)
VALUES (sqlc.arg('id'), sqlc.arg('display_name'))
ON CONFLICT(id) DO UPDATE SET
  display_name = excluded.display_name,
  is_deleted = 0;

-- name: UpsertTool :exec
INSERT INTO tools (id, display_name)
VALUES (sqlc.arg('id'), sqlc.arg('display_name'))
ON CONFLICT(id) DO UPDATE SET
  display_name = excluded.display_name,
  is_deleted = 0;

-- name: UpsertModel :exec
INSERT INTO models (id, provider, display_name)
VALUES (sqlc.arg('id'), sqlc.arg('provider'), sqlc.arg('display_name'))
ON CONFLICT(id) DO UPDATE SET
  provider = excluded.provider,
  display_name = excluded.display_name,
  is_deleted = 0;

-- name: UpsertMCPClient :exec
INSERT INTO mcp_clients (id, name, version, transport)
VALUES (sqlc.arg('id'), sqlc.arg('name'), sqlc.arg('version'), sqlc.arg('transport'))
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  version = excluded.version,
  transport = excluded.transport,
  is_deleted = 0;

-- name: InsertProvenanceContext :one
INSERT OR IGNORE INTO provenance_contexts (
  agent_id, source_kind_id, tool_id, model_id, mcp_client_id
) VALUES (
  sqlc.arg('agent_id'), sqlc.arg('source_kind_id'), sqlc.arg('tool_id'),
  sqlc.arg('model_id'), sqlc.arg('mcp_client_id')
)
RETURNING id;

-- name: GetProvenanceContextID :one
SELECT id
FROM provenance_contexts
WHERE agent_id = sqlc.arg('agent_id')
  AND source_kind_id = sqlc.arg('source_kind_id')
  AND tool_id = sqlc.arg('tool_id')
  AND model_id = sqlc.arg('model_id')
  AND mcp_client_id = sqlc.arg('mcp_client_id')
LIMIT 1;

-- name: GetProvenanceContext :one
SELECT
  p.id,
  p.agent_id,
  a.display_name AS agent_display_name,
  a.kind AS agent_kind,
  p.source_kind_id,
  sk.display_name AS source_display_name,
  p.tool_id,
  t.display_name AS tool_display_name,
  p.model_id,
  m.provider AS model_provider,
  m.display_name AS model_display_name,
  p.mcp_client_id,
  mc.name AS mcp_client_name,
  mc.version AS mcp_client_version,
  mc.transport AS mcp_client_transport,
  p.created_at
FROM provenance_contexts p
JOIN agents a ON a.id = p.agent_id
JOIN source_kinds sk ON sk.id = p.source_kind_id
JOIN tools t ON t.id = p.tool_id
JOIN models m ON m.id = p.model_id
JOIN mcp_clients mc ON mc.id = p.mcp_client_id
WHERE p.id = ?;

-- name: CountObservationsByAgent :many
SELECT COALESCE(p.agent_id, 'unknown') AS agent_id, COUNT(o.id) AS observation_count
FROM observations o
LEFT JOIN provenance_contexts p ON p.id = o.provenance_id
WHERE o.is_deleted = 0
GROUP BY COALESCE(p.agent_id, 'unknown')
ORDER BY observation_count DESC, agent_id ASC;

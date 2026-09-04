-- name: ListObservations :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.is_deleted, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'))
  AND (
    sqlc.arg('tag_count') = 0 OR o.id IN (
      SELECT ot.observation_id
      FROM observation_tags ot
      WHERE ot.is_deleted = 0 AND ot.tag IN (SELECT value FROM json_each(sqlc.arg('tags_json')))
      GROUP BY ot.observation_id
      HAVING COUNT(DISTINCT ot.tag) = sqlc.arg('tag_count')
    )
  )
ORDER BY o.created_at DESC, o.id DESC
LIMIT sqlc.arg('result_limit');

-- name: ListRecentObservations :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.is_deleted, o.provenance_id,
       (CASE WHEN sqlc.arg('topic_key') != '' AND o.topic_key = sqlc.arg('topic_key') THEN 1 ELSE 0 END) +
       (SELECT COUNT(*) FROM observation_tags bt
        WHERE bt.observation_id = o.id
          AND bt.is_deleted = 0
          AND bt.tag IN (SELECT value FROM json_each(sqlc.arg('prefer_tags_json')))) AS preference_score
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'))
  AND (
    sqlc.arg('tag_count') = 0 OR o.id IN (
      SELECT ot.observation_id
      FROM observation_tags ot
      WHERE ot.is_deleted = 0 AND ot.tag IN (SELECT value FROM json_each(sqlc.arg('tags_json')))
      GROUP BY ot.observation_id
      HAVING COUNT(DISTINCT ot.tag) = sqlc.arg('tag_count')
    )
  )
ORDER BY preference_score DESC, o.created_at DESC, o.id DESC
LIMIT sqlc.arg('result_limit');

-- name: ListSessionObservations :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.is_deleted, o.provenance_id
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.session_id = sqlc.arg('session_id') AND o.is_deleted = 0
ORDER BY o.created_at ASC
LIMIT sqlc.arg('result_limit');

-- name: SearchObservationsByFilter :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.is_deleted, o.provenance_id,
       CAST((SELECT COUNT(*) FROM observation_tags bt
             WHERE bt.observation_id = o.id
               AND bt.is_deleted = 0
               AND bt.tag IN (SELECT value FROM json_each(sqlc.arg('prefer_tags_json')))) AS REAL) AS relevance
FROM observations o
JOIN sessions s ON s.id = o.session_id
WHERE o.is_deleted = 0
  AND (sqlc.arg('type') = '' OR o.type = sqlc.arg('type'))
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'))
  AND (sqlc.arg('topic_key') = '' OR o.topic_key = sqlc.arg('topic_key'))
  AND (
    sqlc.arg('tag_count') = 0 OR o.id IN (
      SELECT ot.observation_id
      FROM observation_tags ot
      WHERE ot.is_deleted = 0 AND ot.tag IN (SELECT value FROM json_each(sqlc.arg('tags_json')))
      GROUP BY ot.observation_id
      HAVING COUNT(DISTINCT ot.tag) = sqlc.arg('tag_count')
    )
  )
ORDER BY relevance DESC, o.created_at DESC, o.id DESC
LIMIT sqlc.arg('result_limit');

-- name: SearchObservationsFTS :many
SELECT o.id, ifnull(o.sync_id, '') AS sync_id, o.session_id, o.type, o.title, o.content,
       o.tool_name, s.project AS project, o.scope, o.topic_key, o.revision_count, o.duplicate_count,
       o.last_seen_at, o.created_at, o.updated_at, o.is_deleted, o.provenance_id,
       CAST(observations_fts.rank - CAST((SELECT COUNT(*) FROM observation_tags bt
                        WHERE bt.observation_id = o.id
                          AND bt.tag IN (SELECT value FROM json_each(sqlc.arg('prefer_tags_json')))) AS REAL) * 0.5 AS REAL) AS relevance
FROM observations_fts(sqlc.arg('fts_query'))
JOIN observations o ON o.id = observations_fts.rowid
JOIN sessions s ON s.id = o.session_id
WHERE o.is_deleted = 0
  AND (sqlc.arg('type') = '' OR o.type = sqlc.arg('type'))
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
  AND (sqlc.arg('scope') = '' OR o.scope = sqlc.arg('scope'))
  AND (sqlc.arg('topic_key') = '' OR o.topic_key = sqlc.arg('topic_key'))
  AND (
    sqlc.arg('tag_count') = 0 OR o.id IN (
      SELECT ot.observation_id
      FROM observation_tags ot
      WHERE ot.is_deleted = 0 AND ot.tag IN (SELECT value FROM json_each(sqlc.arg('tags_json')))
      GROUP BY ot.observation_id
      HAVING COUNT(DISTINCT ot.tag) = sqlc.arg('tag_count')
    )
  )
ORDER BY relevance
LIMIT sqlc.arg('result_limit');

-- name: ListRecentPrompts :many
SELECT p.id, ifnull(p.sync_id, '') AS sync_id, p.session_id, p.content,
       s.project AS project, p.created_at, p.provenance_id
FROM user_prompts p
JOIN sessions s ON s.id = p.session_id
WHERE p.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
ORDER BY p.created_at DESC
LIMIT sqlc.arg('result_limit');

-- name: SearchPromptsFTS :many
SELECT p.id, ifnull(p.sync_id, '') AS sync_id, p.session_id, p.content,
       s.project AS project, p.created_at, p.provenance_id
FROM user_prompts_fts(sqlc.arg('fts_query'))
JOIN user_prompts p ON p.id = user_prompts_fts.rowid
JOIN sessions s ON s.id = p.session_id
WHERE p.is_deleted = 0
  AND (sqlc.arg('project') = '' OR s.project = sqlc.arg('project'))
ORDER BY user_prompts_fts.rank
LIMIT sqlc.arg('result_limit');

-- mnemo:foreign-keys-off
-- Repair legacy project keys before 0022 adds the projects foreign keys. Each
-- distinct orphan key receives one UUID project root and all references are
-- rewritten to that root.
CREATE TEMP TABLE project_reference_repair (
    legacy_project TEXT PRIMARY KEY,
    project_id TEXT NOT NULL UNIQUE
);

INSERT INTO project_reference_repair (legacy_project, project_id)
SELECT refs.project,
       lower(hex(randomblob(4))) || '-' ||
       lower(hex(randomblob(2))) || '-' ||
       lower(hex(randomblob(2))) || '-' ||
       lower(hex(randomblob(2))) || '-' ||
       lower(hex(randomblob(6)))
FROM (
    SELECT project FROM sessions WHERE project IS NOT NULL AND trim(project) != ''
    UNION
    SELECT project FROM observations WHERE project IS NOT NULL AND trim(project) != ''
    UNION
    SELECT project FROM user_prompts WHERE project IS NOT NULL AND trim(project) != ''
    UNION
    SELECT project FROM sync_mutations WHERE project IS NOT NULL AND trim(project) != ''
    UNION
    SELECT project FROM sync_enrolled_projects WHERE project IS NOT NULL AND trim(project) != ''
) AS refs
WHERE NOT EXISTS (
    SELECT 1 FROM projects WHERE projects.id = refs.project
);

INSERT INTO projects (id, name)
SELECT project_id, legacy_project
FROM project_reference_repair;

UPDATE sessions
SET project = (SELECT project_id FROM project_reference_repair WHERE legacy_project = sessions.project)
WHERE project IN (SELECT legacy_project FROM project_reference_repair);

UPDATE observations
SET project = (SELECT project_id FROM project_reference_repair WHERE legacy_project = observations.project)
WHERE project IN (SELECT legacy_project FROM project_reference_repair);

UPDATE user_prompts
SET project = (SELECT project_id FROM project_reference_repair WHERE legacy_project = user_prompts.project)
WHERE project IN (SELECT legacy_project FROM project_reference_repair);

UPDATE sync_mutations
SET project = (SELECT project_id FROM project_reference_repair WHERE legacy_project = sync_mutations.project)
WHERE project IN (SELECT legacy_project FROM project_reference_repair);

UPDATE sync_enrolled_projects
SET project = (SELECT project_id FROM project_reference_repair WHERE legacy_project = sync_enrolled_projects.project)
WHERE project IN (SELECT legacy_project FROM project_reference_repair);

DROP TABLE project_reference_repair;

-- mnemo:foreign-keys-off
-- Empty legacy session project keys are invalid once sessions.project
-- references the projects aggregate root. Keep those rows by assigning them
-- a recoverable UUID project instead of inventing a foreign-key parent named ''.
CREATE TEMP TABLE empty_project_repair (
    project_id TEXT PRIMARY KEY
);

INSERT INTO empty_project_repair (project_id)
SELECT lower(hex(randomblob(4))) || '-' ||
       lower(hex(randomblob(2))) || '-' ||
       lower(hex(randomblob(2))) || '-' ||
       lower(hex(randomblob(2))) || '-' ||
       lower(hex(randomblob(6)))
 WHERE EXISTS (SELECT 1 FROM sessions WHERE project IS NULL OR trim(project) = '');

INSERT INTO projects (id, name)
SELECT project_id, 'Recovered legacy project'
FROM empty_project_repair;

UPDATE sessions
SET project = (SELECT project_id FROM empty_project_repair)
WHERE project IS NULL OR trim(project) = '';

DROP TABLE empty_project_repair;

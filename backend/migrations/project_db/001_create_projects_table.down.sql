-- Rollback project tables
DROP INDEX IF EXISTS idx_collaborators_user_id;
DROP INDEX IF EXISTS idx_collaborators_project_id;
DROP TABLE IF EXISTS project_collaborators;

DROP INDEX IF EXISTS idx_secrets_project_id;
DROP TABLE IF EXISTS secrets;

DROP INDEX IF EXISTS idx_projects_github_repo_id;
DROP INDEX IF EXISTS idx_projects_user_id;
DROP TABLE IF EXISTS projects;


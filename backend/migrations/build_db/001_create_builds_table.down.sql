-- Rollback build tables
DROP INDEX IF EXISTS idx_deployments_subdomain;
DROP INDEX IF EXISTS idx_deployments_status;
DROP INDEX IF EXISTS idx_deployments_project_id;
DROP INDEX IF EXISTS idx_deployments_build_id;
DROP TABLE IF EXISTS deployments;

DROP INDEX IF EXISTS idx_build_logs_build_id;
DROP TABLE IF EXISTS build_logs;

DROP INDEX IF EXISTS idx_builds_project_status;
DROP INDEX IF EXISTS idx_builds_created_at;
DROP INDEX IF EXISTS idx_builds_status;
DROP INDEX IF EXISTS idx_builds_project_id;
DROP TABLE IF EXISTS builds;


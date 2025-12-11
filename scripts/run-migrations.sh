#!/bin/bash
set -e

echo "========================================="
echo "Running NexusDeploy Database Migrations"
echo "========================================="

# Check if golang-migrate is installed
if ! command -v migrate &> /dev/null; then
    echo "ERROR: golang-migrate not found"
    echo "Install: https://github.com/golang-migrate/migrate"
    echo "macOS: brew install golang-migrate"
    echo "Linux: Download from releases page"
    exit 1
fi

# Database connection strings
AUTH_DB="postgresql://nexus:nexus_dev@localhost:5432/auth_db?sslmode=disable"
PROJECT_DB="postgresql://nexus:nexus_dev@localhost:5432/project_db?sslmode=disable"
BUILD_DB="postgresql://nexus:nexus_dev@localhost:5432/build_db?sslmode=disable"

# Run migrations for auth_db
echo ""
echo "=== Migrating auth_db ==="
migrate -path backend/migrations/auth_db -database "$AUTH_DB" up
echo "✓ auth_db migrated"

# Run migrations for project_db
echo ""
echo "=== Migrating project_db ==="
migrate -path backend/migrations/project_db -database "$PROJECT_DB" up
echo "✓ project_db migrated"

# Run migrations for build_db
echo ""
echo "=== Migrating build_db ==="
migrate -path backend/migrations/build_db -database "$BUILD_DB" up
echo "✓ build_db migrated"

echo ""
echo "========================================="
echo "✓ All migrations completed successfully!"
echo "========================================="


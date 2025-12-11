#!/bin/bash
# Frontend Setup Validation Script
# DevOps Engineer - 2025-11-03

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASS=0
FAIL=0

echo "========================================"
echo "Frontend Setup Validation"
echo "========================================"
echo ""

# Function to check file exists
check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓${NC} $1 exists"
        ((PASS++))
        return 0
    else
        echo -e "${RED}✗${NC} $1 missing"
        ((FAIL++))
        return 1
    fi
}

# Function to check directory exists
check_dir() {
    if [ -d "$1" ]; then
        echo -e "${GREEN}✓${NC} $1 exists"
        ((PASS++))
        return 0
    else
        echo -e "${RED}✗${NC} $1 missing"
        ((FAIL++))
        return 1
    fi
}

# Function to check file contains pattern
check_content() {
    if grep -q "$2" "$1" 2>/dev/null; then
        echo -e "${GREEN}✓${NC} $1 contains '$2'"
        ((PASS++))
        return 0
    else
        echo -e "${RED}✗${NC} $1 missing '$2'"
        ((FAIL++))
        return 1
    fi
}

echo "=== Critical Files ==="
check_file "frontend/Dockerfile"
check_file "frontend/.dockerignore"
check_file "frontend/next.config.js"
check_file "frontend/.env.example"
check_file "frontend/.gitignore"
check_file "docs/auth-strategy.md"

echo ""
echo "=== Directory Structure ==="
check_dir "frontend/lib"
check_dir "frontend/lib/api"
check_dir "frontend/lib/store"
check_dir "frontend/lib/hooks"
check_dir "frontend/components/ui"
check_dir "frontend/components/layout"
check_dir "frontend/components/common"
check_dir "frontend/types"

echo ""
echo "=== Next.js Configuration ==="
check_content "frontend/next.config.js" "output: 'standalone'"
check_content "frontend/next.config.js" "images:"
check_content "frontend/next.config.js" "rewrites"

echo ""
echo "=== Docker Configuration ==="
check_content "docker-compose.yml" "frontend:"
check_content "docker-compose.yml" "build:"
check_content "docker-compose.yml" "NEXT_PUBLIC_API_URL"
check_file "docker-compose.dev.yml"

echo ""
echo "=== Dockerfile Multi-stage Build ==="
check_content "frontend/Dockerfile" "FROM node:20-alpine AS deps"
check_content "frontend/Dockerfile" "FROM node:20-alpine AS builder"
check_content "frontend/Dockerfile" "FROM node:20-alpine AS runner"
check_content "frontend/Dockerfile" "HEALTHCHECK"

echo ""
echo "========================================"
echo "Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}"
echo "========================================"

if [ $FAIL -gt 0 ]; then
    echo -e "${RED}Validation FAILED${NC}"
    echo "Fix the issues above before proceeding."
    exit 1
else
    echo -e "${GREEN}Validation PASSED${NC}"
    echo "Frontend setup is complete!"
    exit 0
fi


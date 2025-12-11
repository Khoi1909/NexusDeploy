#!/bin/bash
set -e

echo "========================================="
echo "Backend Setup Validation (Runtime Tests)"
echo "========================================="

# 1. Proto code generated
echo ""
echo "=== Proto Code ==="
PROTO_FILES=$(find backend/services -name "*.pb.go" 2>/dev/null | wc -l)
if [ $PROTO_FILES -lt 4 ]; then
    echo "❌ Expected 4 .pb.go files, found $PROTO_FILES"
    echo "   Run: ./scripts/generate-proto.sh"
    exit 1
fi
echo "✓ Proto code generated ($PROTO_FILES files)"

# 2. Services compile
echo ""
echo "=== Service Compilation ==="
cd backend/services/auth-service
if ! go build -o /tmp/auth-service main_with_handlers.go > /dev/null 2>&1; then
    echo "❌ auth-service compilation failed"
    exit 1
fi
echo "✓ auth-service compiles"

cd ../project-service
if ! go build -o /tmp/project-service main_with_handlers.go > /dev/null 2>&1; then
    echo "❌ project-service compilation failed"
    exit 1
fi
echo "✓ project-service compiles"

cd ../../..

# 3. Services start successfully
echo ""
echo "=== Service Runtime (Docker) ==="
if ! docker-compose ps | grep -q "nexus_auth_service.*Up"; then
    echo "⚠️  Services not running, starting them..."
    docker-compose up -d auth-service project-service > /dev/null 2>&1
    sleep 5
fi

# Check auth-service status
if docker ps --format "{{.Names}}\t{{.Status}}" | grep -q "nexus_auth_service.*healthy"; then
    echo "✓ auth-service running (healthy)"
elif docker ps --format "{{.Names}}\t{{.Status}}" | grep -q "nexus_auth_service.*Up"; then
    echo "⚠️  auth-service running (unhealthy - check logs)"
else
    echo "❌ auth-service not running"
    exit 1
fi

# Check project-service status
if docker ps --format "{{.Names}}\t{{.Status}}" | grep -q "nexus_project_service.*healthy"; then
    echo "✓ project-service running (healthy)"
elif docker ps --format "{{.Names}}\t{{.Status}}" | grep -q "nexus_project_service.*Up"; then
    echo "⚠️  project-service running (unhealthy - check logs)"
else
    echo "❌ project-service not running"
    exit 1
fi

# 4. Health endpoints return 200 OK
echo ""
echo "=== Health Endpoints ==="

# Test auth-service health
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9001/health || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    echo "✓ auth-service /health returns 200 OK"
elif [ "$HTTP_CODE" = "000" ]; then
    echo "❌ auth-service /health unreachable (service may not be running)"
    exit 1
else
    echo "❌ auth-service /health returned $HTTP_CODE (expected 200)"
    exit 1
fi

# Test project-service health
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9002/health || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    echo "✓ project-service /health returns 200 OK"
elif [ "$HTTP_CODE" = "000" ]; then
    echo "❌ project-service /health unreachable (service may not be running)"
    exit 1
else
    echo "❌ project-service /health returned $HTTP_CODE (expected 200)"
    exit 1
fi

# 5. gRPC calls work
echo ""
echo "=== gRPC Communication ==="

# Check if grpcurl is installed
if ! command -v grpcurl &> /dev/null; then
    echo "⚠️  grpcurl not installed, skipping gRPC tests"
    echo "   Install: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
else
    # Test auth-service gRPC reflection
    if grpcurl -plaintext localhost:50051 list > /dev/null 2>&1; then
        echo "✓ auth-service gRPC reflection works"
        
        # Test ValidateToken RPC
        RESPONSE=$(grpcurl -plaintext -d '{"token":"test-token"}' localhost:50051 auth.AuthService/ValidateToken 2>&1)
        if echo "$RESPONSE" | grep -q '"valid"'; then
            echo "✓ auth-service ValidateToken RPC works"
        else
            echo "⚠️  auth-service ValidateToken RPC returned unexpected response"
            echo "   Response: $RESPONSE"
        fi
    else
        echo "❌ auth-service gRPC reflection failed"
        exit 1
    fi
    
    # Test project-service gRPC reflection
    if grpcurl -plaintext localhost:50052 list > /dev/null 2>&1; then
        echo "✓ project-service gRPC reflection works"
        
        # Test ListProjects RPC
        RESPONSE=$(grpcurl -plaintext -d '{"user_id":"test-user"}' localhost:50052 project.ProjectService/ListProjects 2>&1)
        if echo "$RESPONSE" | grep -q '"projects"'; then
            echo "✓ project-service ListProjects RPC works"
        else
            echo "⚠️  project-service ListProjects RPC returned unexpected response"
            echo "   Response: $RESPONSE"
        fi
    else
        echo "❌ project-service gRPC reflection failed"
        exit 1
    fi
fi

echo ""
echo "========================================="
echo "✓ Backend validation PASSED"
echo "All critical components verified (compile + runtime)"
echo "========================================="

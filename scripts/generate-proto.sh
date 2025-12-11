#!/bin/bash
set -e

echo "========================================="
echo "Generating Protobuf Code"
echo "========================================="

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "ERROR: protoc not found"
    echo "Install: https://grpc.io/docs/protoc-installation/"
    echo "macOS: brew install protobuf"
    echo "Linux: apt install -y protobuf-compiler"
    exit 1
fi

# Check protoc-gen-go
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check protoc-gen-go-grpc
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

echo ""
echo "=== Generating auth-service proto ==="
cd backend/services/auth-service
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/auth.proto
echo "✓ auth.proto generated"

cd ../../..

echo ""
echo "=== Generating project-service proto ==="
cd backend/services/project-service
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/project.proto
echo "✓ project.proto generated"

cd ../../..

echo ""
echo "========================================="
echo "✓ All proto files generated successfully!"
echo "========================================="


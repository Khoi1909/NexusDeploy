# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install dependencies including protoc and protoc-gen-go
RUN apk add --no-cache git protobuf
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy go.mod and go.sum first for caching
COPY services/api-gateway/go.mod services/api-gateway/go.sum* ./services/api-gateway/
COPY services/auth-service/proto/ ./services/auth-service/proto/
COPY services/project-service/proto/ ./services/project-service/proto/
COPY services/build-service/proto/ ./services/build-service/proto/
COPY services/deployment-service/proto/ ./services/deployment-service/proto/
COPY services/ai-service/proto/ ./services/ai-service/proto/
COPY pkg/ ./pkg/

# Download dependencies
WORKDIR /build/services/api-gateway
RUN go mod download

# Generate proto files for auth-service (needed by api-gateway)
WORKDIR /build/services/auth-service
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/auth.proto

# Copy source code
WORKDIR /build
COPY services/api-gateway/ ./services/api-gateway/

# Build binary
WORKDIR /build/services/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o api-gateway .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/services/api-gateway/api-gateway .

# Create non-root user
RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus nexus && \
    chown -R nexus:nexus /app

USER nexus

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./api-gateway"]


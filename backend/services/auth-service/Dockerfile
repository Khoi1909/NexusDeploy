# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first for caching
COPY services/auth-service/go.mod services/auth-service/go.sum* ./services/auth-service/
COPY services/auth-service/proto/ ./services/auth-service/proto/
COPY pkg/ ./pkg/

# Download dependencies
WORKDIR /build/services/auth-service
RUN go mod download

# Copy source code
WORKDIR /build
COPY services/auth-service/ ./services/auth-service/

# Build binary
WORKDIR /build/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/services/auth-service/auth-service .

# Create non-root user
RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus nexus && \
    chown -R nexus:nexus /app

USER nexus

EXPOSE 8080 50051

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./auth-service"]


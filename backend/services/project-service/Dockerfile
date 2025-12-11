# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum first for caching
COPY services/project-service/go.mod services/project-service/go.sum* ./services/project-service/
COPY services/project-service/proto/ ./services/project-service/proto/
COPY pkg/ ./pkg/

# Download dependencies
WORKDIR /build/services/project-service
RUN go mod download

# Copy source code
WORKDIR /build
COPY services/project-service/ ./services/project-service/

# Build binary
WORKDIR /build/services/project-service
RUN CGO_ENABLED=0 GOOS=linux go build -o project-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /build/services/project-service/project-service .

# Create non-root user
RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus nexus && \
    chown -R nexus:nexus /app

USER nexus

EXPOSE 8080 50052

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./project-service"]


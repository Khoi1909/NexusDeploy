# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git protobuf

# Copy go.mod and go.sum first for caching
COPY services/build-service/go.mod services/build-service/go.sum* ./services/build-service/
COPY services/build-service/proto/ ./services/build-service/proto/
COPY pkg/ ./pkg/

# Download dependencies
WORKDIR /build/services/build-service
RUN go mod download

# Copy source code
WORKDIR /build
COPY services/build-service/ ./services/build-service/

# Build binary
WORKDIR /build/services/build-service
RUN CGO_ENABLED=0 GOOS=linux go build -o build-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/services/build-service/build-service .

RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus nexus && \
    chown -R nexus:nexus /app

USER nexus

EXPOSE 8080 50053

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./build-service"]


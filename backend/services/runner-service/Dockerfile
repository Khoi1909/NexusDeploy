# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY services/runner-service/go.mod services/runner-service/go.sum* ./services/runner-service/
COPY services/build-service/proto/ ./services/build-service/proto/
COPY services/project-service/proto/ ./services/project-service/proto/
COPY pkg/ ./pkg/

WORKDIR /build/services/runner-service
RUN go mod download

WORKDIR /build
COPY services/runner-service/ ./services/runner-service/

WORKDIR /build/services/runner-service
RUN CGO_ENABLED=0 GOOS=linux go build -o runner-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

RUN apk --no-cache add ca-certificates docker-cli git

COPY --from=builder /build/services/runner-service/runner-service .

# Add docker group with GID 124 (match host Docker group GID)
RUN addgroup -g 124 docker || true

RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus -G docker nexus && \
    chown -R nexus:nexus /app && \
    mkdir -p /tmp/nexus-builds && \
    chown -R nexus:nexus /tmp/nexus-builds

USER nexus

EXPOSE 8080 50054

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./runner-service"]


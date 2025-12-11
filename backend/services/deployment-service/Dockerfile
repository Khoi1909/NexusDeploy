# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY services/deployment-service/go.mod services/deployment-service/go.sum* ./services/deployment-service/
COPY services/deployment-service/proto/ ./services/deployment-service/proto/
COPY pkg/ ./pkg/

WORKDIR /build/services/deployment-service
RUN go mod download

WORKDIR /build
COPY services/deployment-service/ ./services/deployment-service/

WORKDIR /build/services/deployment-service
RUN CGO_ENABLED=0 GOOS=linux go build -o deployment-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

RUN apk --no-cache add ca-certificates docker-cli

COPY --from=builder /build/services/deployment-service/deployment-service .

# Add docker group with GID 124 (match host Docker group GID)
RUN addgroup -g 124 docker || true

RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus -G docker nexus && \
    chown -R nexus:nexus /app

USER nexus

EXPOSE 8080 50055

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./deployment-service"]


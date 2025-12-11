# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

COPY services/notification-service/go.mod services/notification-service/go.sum* ./services/notification-service/
COPY pkg/ ./pkg/

WORKDIR /build/services/notification-service
RUN go mod download

WORKDIR /build
COPY services/notification-service/ ./services/notification-service/

WORKDIR /build/services/notification-service
RUN CGO_ENABLED=0 GOOS=linux go build -o notification-service .

# Runtime stage
FROM alpine:3.18

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/services/notification-service/notification-service .

RUN addgroup -g 1001 nexus && \
    adduser -D -u 1001 -G nexus nexus && \
    chown -R nexus:nexus /app

USER nexus

EXPOSE 8080 50057

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./notification-service"]


package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/notification-service/pubsub"
	"github.com/nexusdeploy/backend/services/notification-service/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	httpPort = ":8080"
)

var (
	log zerolog.Logger
	cfg *cfgpkg.Config
)

func main() {
	// Load configuration
	var err error
	cfg, err = cfgpkg.LoadConfig()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	// Initialize logger
	logger.InitLogger("notification-service", cfg.LogLevel, cfg.LogFormat)
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "notification-service").
		Logger()

	log.Info().Msg("Starting Notification Service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	log.Info().Str("addr", cfg.GetRedisAddr()).Msg("Connected to Redis")

	// Create WebSocket hub
	hub := websocket.NewHub(log)
	go hub.Run(ctx)

	// Create and start Redis Pub/Sub consumer
	consumer := pubsub.NewConsumer(redisClient, hub, log)
	go consumer.Start(ctx)

	// Start HTTP server with WebSocket support
	go startHTTPServer(ctx, hub, redisClient)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Notification Service...")
	cancel()
	redisClient.Close()
	time.Sleep(2 * time.Second)
}

func startHTTPServer(ctx context.Context, hub *websocket.Hub, redisClient *redis.Client) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness check - verify Redis connection
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if redisClient.Ping(ctx).Err() == nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Redis not available"))
		}
	})

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	// WebSocket endpoint for real-time notifications
	wsHandler := websocket.NewHandler(hub, log)
	mux.HandleFunc("/ws", wsHandler.HandleWebSocket)

	// CORS wrapper for development
	corsHandler := corsMiddleware(mux)

	log.Info().Str("address", httpPort).Msg("Notification Service HTTP listening")
	if err := http.ListenAndServe(httpPort, corsHandler); err != nil {
		log.Fatal().Err(err).Msg("HTTP serve failed")
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow WebSocket origins in development
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

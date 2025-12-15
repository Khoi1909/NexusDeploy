package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	grpcpkg "github.com/nexusdeploy/backend/pkg/grpc"
	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/ai-service/handlers"
	"github.com/nexusdeploy/backend/services/ai-service/proto"
	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	grpcPort = ":50056"
	httpPort = ":8080"
)

var (
	log zerolog.Logger
	cfg *cfgpkg.Config
)

func main() {
	var err error
	cfg, err = cfgpkg.LoadConfig()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	if cfg.ServiceName == "" || cfg.ServiceName == "unknown-service" {
		cfg.ServiceName = "ai-service"
	}

	logger.InitLogger(cfg.ServiceName, cfg.LogLevel, cfg.LogFormat)
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "ai-service").
		Logger()

	log.Info().Msg("Starting AI Service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test Redis connection
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 5*time.Second)
	if err := redisClient.Ping(ctxTimeout).Err(); err != nil {
		cancelTimeout()
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	cancelTimeout()
	log.Info().Str("addr", cfg.GetRedisAddr()).Msg("Connected to Redis")

	// Initialize Build Service gRPC client
	buildConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.BuildServiceAddr,
		Timeout:            10 * time.Second,
		MaxRetries:         3,
		ServiceName:        "build-service",
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to build service")
	}
	defer buildConn.Close()

	buildClient := buildpb.NewBuildServiceClient(buildConn)

	log.Info().Str("addr", cfg.BuildServiceAddr).Msg("Connected to Build Service")

	// Create AI Service handler
	aiServer := handlers.NewAIServiceServer(cfg, redisClient, buildClient, buildConn)

	// Start gRPC server
	go startGRPCServer(ctx, aiServer)

	// Start HTTP server
	go startHTTPServer(ctx)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down AI Service...")
	cancel()
	redisClient.Close()
	buildConn.Close()
	time.Sleep(2 * time.Second)
}

func startGRPCServer(ctx context.Context, aiServer *handlers.AIServiceServer) {
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	proto.RegisterAIServiceServer(grpcServer, aiServer)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	log.Info().Str("address", grpcPort).Msg("AI Service gRPC listening")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("gRPC serve failed")
	}
}

func startHTTPServer(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})

	log.Info().Str("address", httpPort).Msg("AI Service HTTP listening")
	if err := http.ListenAndServe(httpPort, mux); err != nil {
		log.Fatal().Err(err).Msg("HTTP serve failed")
	}
}

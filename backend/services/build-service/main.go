package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/build-service/handlers"
	"github.com/nexusdeploy/backend/services/build-service/models"
	pb "github.com/nexusdeploy/backend/services/build-service/proto"
	"github.com/nexusdeploy/backend/services/build-service/queue"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	log zerolog.Logger
	cfg *cfgpkg.Config
	db  *gorm.DB
)

func main() {
	// Load configuration
	var err error
	cfg, err = cfgpkg.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.InitLogger("build-service", cfg.LogLevel, cfg.LogFormat)
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "build-service").
		Logger()

	log.Info().Msg("Starting Build Service...")

	// Connect to database
	db, err = connectDB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	log.Info().Msg("Connected to PostgreSQL")

	// Auto-migrate models
	if err := db.AutoMigrate(&models.Build{}, &models.BuildLog{}, &models.BuildStep{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to auto-migrate models")
	}
	log.Info().Msg("Database migration completed")

	// Initialize queue producer
	redisAddr := cfg.GetRedisAddr()
	producer, err := queue.NewProducer(redisAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create queue producer")
	}
	defer producer.Close()
	log.Info().Str("redis", redisAddr).Msg("Queue producer initialized")

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start servers
	go startGRPCServer(ctx, producer)
	go startHTTPServer(ctx)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Build Service...")
	cancel()
	time.Sleep(2 * time.Second)
}

func connectDB() (*gorm.DB, error) {
	dsn := cfg.GetDSN()
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func startGRPCServer(ctx context.Context, producer *queue.Producer) {
	grpcAddr := ":50053"
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer()

	// Register Build Service
	buildServer := handlers.NewBuildServiceServer(db, cfg, producer)
	pb.RegisterBuildServiceServer(grpcServer, buildServer)

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection for grpcurl
	reflection.Register(grpcServer)

	log.Info().Str("address", grpcAddr).Msg("Build Service gRPC listening")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("gRPC serve failed")
	}
}

func startHTTPServer(ctx context.Context) {
	httpAddr := ":8080"
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness check - verify DB connection
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		sqlDB, err := db.DB()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("DB connection error"))
			return
		}
		if err := sqlDB.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("DB ping failed"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})

	log.Info().Str("address", httpAddr).Msg("Build Service HTTP listening")
	if err := http.ListenAndServe(httpAddr, mux); err != nil {
		log.Fatal().Err(err).Msg("HTTP serve failed")
	}
}

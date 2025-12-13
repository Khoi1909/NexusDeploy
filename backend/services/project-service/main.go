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
	grpcpkg "github.com/nexusdeploy/backend/pkg/grpc"
	"github.com/nexusdeploy/backend/pkg/logger"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/nexusdeploy/backend/services/project-service/handlers"
	"github.com/nexusdeploy/backend/services/project-service/models"
	pb "github.com/nexusdeploy/backend/services/project-service/proto"
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
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Initialize logger
	logger.InitLogger("project-service", cfg.LogLevel, cfg.LogFormat)
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "project-service").
		Logger()

	log.Info().Msg("Starting Project Service...")

	// Connect to database
	db, err = connectDB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	log.Info().Msg("Connected to PostgreSQL")

	// Auto-migrate models
	if err := db.AutoMigrate(&models.Project{}, &models.Secret{}, &models.Webhook{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to auto-migrate models")
	}
	log.Info().Msg("Database migration completed")

	// Connect to Auth Service for permission checks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:     cfg.AuthServiceAddr,
		Timeout:     5 * time.Second,
		MaxRetries:  3,
		ServiceName: "auth-service",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Auth Service")
	}
	defer authConn.Close()
	authClient := authpb.NewAuthServiceClient(authConn)
	log.Info().Str("address", cfg.AuthServiceAddr).Msg("Connected to Auth Service")

	// Start servers
	go startGRPCServer(ctx, authClient, authConn)
	go startHTTPServer(ctx)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Project Service...")
	cancel()
	time.Sleep(2 * time.Second)
}

func connectDB() (*gorm.DB, error) {
	dsn := cfg.GetDSN()
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func startGRPCServer(ctx context.Context, authClient authpb.AuthServiceClient, authConn *grpc.ClientConn) {
	grpcAddr := fmt.Sprintf(":%d", 50052)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer()

	// Register Project Service
	projectServer := handlers.NewProjectServiceServer(db, cfg, authClient, authConn)
	pb.RegisterProjectServiceServer(grpcServer, projectServer)

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection for grpcurl
	reflection.Register(grpcServer)

	log.Info().Str("address", grpcAddr).Msg("Project Service gRPC listening")
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

	log.Info().Str("address", httpAddr).Msg("Project Service HTTP listening")
	if err := http.ListenAndServe(httpAddr, mux); err != nil {
		log.Fatal().Err(err).Msg("HTTP serve failed")
	}
}

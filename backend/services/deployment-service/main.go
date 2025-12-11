package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/deployment-service/docker"
	"github.com/nexusdeploy/backend/services/deployment-service/handlers"
	deploymentpb "github.com/nexusdeploy/backend/services/deployment-service/proto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	grpcPort = ":50055"
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
	logger.InitLogger("deployment-service", cfg.LogLevel, cfg.LogFormat)
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "deployment-service").
		Logger()

	log.Info().Msg("Starting Deployment Service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Docker executor
	portStart := getEnvAsInt("DEPLOY_PORT_RANGE_START", 12000)
	portEnd := getEnvAsInt("DEPLOY_PORT_RANGE_END", 12999)
	dockerExec, err := docker.NewExecutor(docker.ExecutorConfig{
		TraefikNetwork:      getEnv("TRAEFIK_NETWORK", "nexus-network"),
		TraefikEntrypoint:   getEnv("TRAEFIK_ENTRYPOINT", "web"),
		TraefikDomainSuffix: getEnv("TRAEFIK_DOMAIN_SUFFIX", "localhost"),
		PortRangeStart:      int32(portStart),
		PortRangeEnd:        int32(portEnd),
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Docker executor")
	}
	defer dockerExec.Close()

	// Create deployment handler
	deploymentHandler := handlers.NewDeploymentHandler(dockerExec, log)

	// Start servers
	go startGRPCServer(ctx, deploymentHandler)
	go startHTTPServer(ctx, dockerExec)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Deployment Service...")
	cancel()
	time.Sleep(2 * time.Second)
}

func startGRPCServer(ctx context.Context, handler *handlers.DeploymentHandler) {
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer()

	// Register deployment service
	deploymentpb.RegisterDeploymentServiceServer(grpcServer, handler)

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	log.Info().Str("address", grpcPort).Msg("Deployment Service gRPC listening")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("gRPC serve failed")
	}
}

func startHTTPServer(ctx context.Context, dockerExec *docker.Executor) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness check - verify Docker connection
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if dockerExec.IsHealthy(ctx) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Docker not available"))
		}
	})

	// Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	log.Info().Str("address", httpPort).Msg("Deployment Service HTTP listening")
	if err := http.ListenAndServe(httpPort, mux); err != nil {
		log.Fatal().Err(err).Msg("HTTP serve failed")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return defaultValue
}

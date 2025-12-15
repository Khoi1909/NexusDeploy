package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/runner-service/clients"
	"github.com/nexusdeploy/backend/services/runner-service/executor"
	"github.com/nexusdeploy/backend/services/runner-service/handler"
	"github.com/nexusdeploy/backend/services/runner-service/pubsub"
	"github.com/nexusdeploy/backend/services/runner-service/queue"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	grpcPort = ":50054"
	httpPort = ":8080"
)

var (
	log              zerolog.Logger
	cfg              *cfgpkg.Config
	globalDockerExec *executor.DockerExecutor
)

func main() {
	// Load configuration
	var err error
	cfg, err = cfgpkg.LoadConfig()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	// Initialize logger
	logger.InitLogger("runner-service", cfg.LogLevel, cfg.LogFormat)
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "runner-service").
		Logger()

	log.Info().Msg("Starting Runner Service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize gRPC clients
	grpcClients, err := clients.NewClients(ctx, clients.ClientsConfig{
		BuildServiceAddr:   cfg.BuildServiceAddr,
		ProjectServiceAddr: cfg.ProjectServiceAddr,
		Timeout:            10 * time.Second,
		MaxRetries:         3,
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize gRPC clients")
	}
	defer grpcClients.Close()

	// Initialize Redis Pub/Sub publisher
	publisher, err := pubsub.NewPublisher(pubsub.PublisherConfig{
		RedisAddr:     cfg.GetRedisAddr(),
		RedisPassword: cfg.RedisPassword,
		RedisDB:       cfg.RedisDB,
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Redis publisher")
	}
	defer publisher.Close()

	// Initialize Docker executor
	dockerExec, err := executor.NewDockerExecutor(executor.ExecutorConfig{
		RegistryURL:  getEnv("REGISTRY_URL", ""),
		RegistryUser: getEnv("REGISTRY_USER", ""),
		RegistryPass: getEnv("REGISTRY_PASSWORD", ""),
		WorkDir:      getEnv("BUILD_WORK_DIR", "/tmp/nexus-builds"),
	}, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Docker executor")
	}
	defer dockerExec.Close()

	// Create build handler
	buildHandler := handler.NewBuildHandler(grpcClients, dockerExec, publisher, log)

	// Store dockerExec for HTTP handlers
	globalDockerExec = dockerExec

	// Initialize queue consumer
	concurrency := getEnvAsInt("RUNNER_CONCURRENCY", 2)
	consumer := queue.NewConsumer(queue.ConsumerConfig{
		RedisAddr:   cfg.GetRedisAddr(),
		Concurrency: concurrency,
	}, buildHandler, log)

	// Start queue consumer in background
	go func() {
		log.Info().
			Int("concurrency", concurrency).
			Str("redis", cfg.GetRedisAddr()).
			Msg("Starting Asynq consumer")
		if err := consumer.Start(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start queue consumer")
		}
	}()

	// Start gRPC and HTTP servers
	go startGRPCServer(ctx)
	go startHTTPServer(ctx)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down Runner Service...")

	// Graceful shutdown
	consumer.Shutdown()
	cancel()
	time.Sleep(2 * time.Second)
}

func startGRPCServer(ctx context.Context) {
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer()

	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	log.Info().Str("address", grpcPort).Msg("Runner Service gRPC listening")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("gRPC serve failed")
	}
}

func startHTTPServer(ctx context.Context) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness check
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Runner is stateless, always ready if healthy
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})

	// Cleanup workspaces endpoint
	mux.HandleFunc("/api/cleanup-workspaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Method not allowed"))
			return
		}

		// Parse request body
		var req struct {
			BuildIDs []string `json:"build_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Error().Err(err).Msg("Failed to decode cleanup request")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid request body"))
			return
		}

		if len(req.BuildIDs) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("build_ids required"))
			return
		}

		// Cleanup workspaces
		if err := globalDockerExec.CleanupWorkspaces(req.BuildIDs); err != nil {
			log.Error().Err(err).Msg("Failed to cleanup workspaces")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to cleanup workspaces"))
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Workspaces cleaned up",
			"count":   len(req.BuildIDs),
		})
	})

	log.Info().Str("address", httpPort).Msg("Runner Service HTTP listening")
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
		var i int
		if _, err := os.Stdout.WriteString(""); err == nil {
			// Parse int manually
			for _, c := range value {
				if c >= '0' && c <= '9' {
					i = i*10 + int(c-'0')
				} else {
					return defaultValue
				}
			}
			return i
		}
	}
	return defaultValue
}

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/auth-service/database"
	"github.com/nexusdeploy/backend/services/auth-service/handlers"
	"github.com/nexusdeploy/backend/services/auth-service/models"
	pb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"
)

const (
	grpcAddr = ":50051"
	httpAddr = ":8080"
)

// Prometheus metrics
var (
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_service_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)
	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "auth_service_grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_service_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "status"},
	)
)

type application struct {
	cfg   *cfgpkg.Config
	db    *gorm.DB
	redis *redis.Client
	auth  *handlers.AuthServiceServer
}

func main() {
	cfg, err := cfgpkg.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}
	if cfg.ServiceName == "" || cfg.ServiceName == "unknown-service" {
		cfg.ServiceName = "auth-service"
	}
	logger.InitLogger(cfg.ServiceName, cfg.LogLevel, cfg.LogFormat)

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("open database")
	}
	if err := database.AutoMigrate(db, &models.User{}, &models.RefreshToken{}, &models.Permission{}); err != nil {
		log.Fatal().Err(err).Msg("auto migrate")
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		log.Fatal().Err(err).Msg("connect redis")
	}
	cancel()

	authServer := handlers.NewAuthServiceServer(cfg, db, redisClient)
	app := &application{
		cfg:   cfg,
		db:    db,
		redis: redisClient,
		auth:  authServer,
	}

	// gRPC setup with interceptor for metrics
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("listen grpc")
	}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metricsUnaryInterceptor),
	)
	pb.RegisterAuthServiceServer(grpcServer, authServer)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	go func() {
		log.Info().Str("address", grpcAddr).Msg("Auth Service gRPC listening")
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatal().Err(err).Msg("grpc serve failed")
		}
	}()

	// HTTP setup (health, ready, metrics only - no auth endpoints)
	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("address", httpAddr).Msg("Auth Service HTTP listening (health/ready/metrics)")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http serve failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down auth-service...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("http shutdown")
	}
	grpcServer.GracefulStop()
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	_ = redisClient.Close()
	log.Info().Msg("Auth-service stopped")
}

func (a *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.handleHealth)
	mux.HandleFunc("/ready", a.handleReady)
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func (a *application) handleHealth(w http.ResponseWriter, _ *http.Request) {
	httpRequestsTotal.WithLabelValues("/health", "200").Inc()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (a *application) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := a.readinessCheck(ctx); err != nil {
		log.Warn().Err(err).Msg("readiness check failed")
		httpRequestsTotal.WithLabelValues("/ready", "503").Inc()
		http.Error(w, "NOT_READY", http.StatusServiceUnavailable)
		return
	}
	httpRequestsTotal.WithLabelValues("/ready", "200").Inc()
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("READY"))
}

func (a *application) readinessCheck(ctx context.Context) error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	if err := a.redis.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}

// metricsUnaryInterceptor ghi lại metrics cho mỗi gRPC call.
func metricsUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start).Seconds()

	status := "ok"
	if err != nil {
		status = "error"
	}

	grpcRequestsTotal.WithLabelValues(info.FullMethod, status).Inc()
	grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

	return resp, err
}

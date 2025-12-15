package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	"github.com/nexusdeploy/backend/services/api-gateway/handlers"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics for API Gateway
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_gateway_http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// RateLimitConfig cấu hình rate limit
type RateLimitConfig struct {
	RequestsPerWindow int
	Window            time.Duration
}

// RouterConfig chứa các dependency cho router
type RouterConfig struct {
	Config            *cfgpkg.Config
	AuthClient        apimw.AuthClient
	AuthHandler       *handlers.AuthHandler
	ProjectHandler    *handlers.ProjectHandler
	BuildHandler      *handlers.BuildHandler
	DeploymentHandler *handlers.DeploymentHandler
	WebhookHandler    *handlers.WebhookHandler
	WebSocketProxy    *handlers.WebSocketProxy
	RateLimit         RateLimitConfig
}

// NewRouter trả về http.Handler với đầy đủ middleware và routes
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// WebSocket proxy must be registered BEFORE middleware to avoid hijacking issues
	// WebSocket needs direct access to underlying connection
	if cfg.WebSocketProxy != nil {
		mux.HandleFunc("/ws", cfg.WebSocketProxy.HandleWebSocket)
	}

	// Health, readiness & metrics
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// Auth routes (public, no JWT required)
	if cfg.AuthHandler != nil {
		mux.HandleFunc("/auth/github/login", cfg.AuthHandler.HandleGitHubLogin)
		mux.HandleFunc("/auth/github/callback", cfg.AuthHandler.HandleGitHubCallback)
		mux.HandleFunc("/auth/refresh", cfg.AuthHandler.HandleRefresh)
		mux.HandleFunc("/auth/logout", cfg.AuthHandler.HandleLogout)
	}

	// Protected user routes
	if cfg.AuthHandler != nil && cfg.AuthClient != nil {
		authMW := apimw.AuthMiddleware(cfg.AuthClient)
		mux.Handle("/api/user/info", chain(
			http.HandlerFunc(cfg.AuthHandler.HandleGetUserInfo),
			authMW,
		))
		mux.Handle("/api/user/plan", chain(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					cfg.AuthHandler.HandleGetUserPlan(w, r)
				} else if r.Method == http.MethodPut {
					cfg.AuthHandler.HandleUpdatePlan(w, r)
				} else {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					json.NewEncoder(w).Encode(map[string]string{"error": "method_not_allowed"})
				}
			}),
			authMW,
		))
	}

	// Protected routes - Project CRUD
	if cfg.ProjectHandler != nil && cfg.AuthClient != nil {
		authMW := apimw.AuthMiddleware(cfg.AuthClient)

		// Projects
		mux.Handle("/api/projects", chain(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					cfg.ProjectHandler.ListProjects(w, r)
				case http.MethodPost:
					cfg.ProjectHandler.CreateProject(w, r)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}),
			authMW,
		))

		// Single project operations
		mux.Handle("/api/projects/", chain(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check if it's a deployment path
				if cfg.DeploymentHandler != nil {
					if containsDeploy(r.URL.Path) {
						if r.Method == http.MethodPost {
							cfg.DeploymentHandler.Deploy(w, r)
							return
						}
					}
					if containsStop(r.URL.Path) {
						if r.Method == http.MethodPost {
							cfg.DeploymentHandler.StopDeployment(w, r)
							return
						}
					}
					if containsRestart(r.URL.Path) {
						if r.Method == http.MethodPost {
							cfg.DeploymentHandler.RestartDeployment(w, r)
							return
						}
					}
					if containsDeployment(r.URL.Path) {
						if r.Method == http.MethodGet {
							cfg.DeploymentHandler.GetDeploymentStatus(w, r)
							return
						}
					}
				}
				// Check if it's a builds path
				if containsBuilds(r.URL.Path) && cfg.BuildHandler != nil {
					// Check if it's clear logs: DELETE /api/projects/{id}/builds/logs
					if strings.HasSuffix(r.URL.Path, "/builds/logs") && r.Method == http.MethodDelete {
						cfg.BuildHandler.ClearBuildLogs(w, r)
						return
					}
					switch r.Method {
					case http.MethodGet:
						cfg.BuildHandler.ListBuilds(w, r)
					case http.MethodPost:
						cfg.BuildHandler.TriggerBuild(w, r)
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
					return
				}
				// Check if it's a secrets path
				if containsSecrets(r.URL.Path) {
					switch r.Method {
					case http.MethodGet:
						cfg.ProjectHandler.ListSecrets(w, r)
					case http.MethodPost:
						cfg.ProjectHandler.AddSecret(w, r)
					case http.MethodDelete:
						cfg.ProjectHandler.DeleteSecret(w, r)
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
					return
				}
				// Regular project operations
				switch r.Method {
				case http.MethodGet:
					cfg.ProjectHandler.GetProject(w, r)
				case http.MethodPut, http.MethodPatch:
					cfg.ProjectHandler.UpdateProject(w, r)
				case http.MethodDelete:
					cfg.ProjectHandler.DeleteProject(w, r)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}),
			authMW,
		))

		// GitHub repositories
		mux.Handle("/api/repos", chain(
			http.HandlerFunc(cfg.ProjectHandler.ListRepositories),
			authMW,
		))
	}

	// Build routes
	if cfg.BuildHandler != nil && cfg.AuthClient != nil {
		authMW := apimw.AuthMiddleware(cfg.AuthClient)

		// Single build details: GET /api/builds/{id}
		// Build logs: GET /api/builds/{id}/logs
		// Analyze build: POST /api/builds/{id}/analyze
		mux.Handle("/api/builds/", chain(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/analyze") && r.Method == http.MethodPost {
					cfg.BuildHandler.AnalyzeBuild(w, r)
					return
				}
				if containsLogs(r.URL.Path) {
					cfg.BuildHandler.GetBuildLogs(w, r)
					return
				}
				cfg.BuildHandler.GetBuild(w, r)
			}),
			authMW,
		))
	}

	// GitHub webhook (no auth header but signature validation inside handler)
	if cfg.WebhookHandler != nil {
		mux.Handle("/webhooks/github", http.HandlerFunc(cfg.WebhookHandler.HandleGitHubWebhook))
	}

	// Apply global middleware chain
	handler := http.Handler(mux)

	// Metrics middleware
	handler = metricsMiddleware(handler)

	// Plan-based rate limiting (after auth middleware so we can get plan from context)
	// Note: This will be applied after auth middleware in protected routes
	// For public routes, use standard rate limit
	if cfg.RateLimit.Window > 0 {
		handler = commonmw.PlanBasedRateLimit(
			cfg.RateLimit.Window,
			func(r *http.Request) string {
				return apimw.GetPlan(r.Context())
			},
			func(r *http.Request) string {
				return apimw.GetUserID(r.Context())
			},
		)(handler)
	}
	// CORS middleware với configurable allowed origins
	var allowedOrigins []string
	if cfg.Config != nil {
		allowedOrigins = cfg.Config.AllowedOrigins
	}
	handler = commonmw.CORS(allowedOrigins, handler)
	handler = commonmw.ErrorHandler(handler)
	handler = commonmw.CorrelationID(handler)

	return handler
}

// chain áp dụng các middleware theo thứ tự
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			h = middlewares[i](h)
		}
	}
	return h
}

// responseWriter wrapper để capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// containsSecrets checks if the path contains /secrets
func containsSecrets(path string) bool {
	return strings.Contains(path, "/secrets")
}

// containsBuilds checks if the path contains /builds
func containsBuilds(path string) bool {
	return strings.Contains(path, "/builds")
}

// containsLogs checks if the path contains /logs
func containsLogs(path string) bool {
	return strings.Contains(path, "/logs")
}

// containsDeploy checks if the path contains /deploy
func containsDeploy(path string) bool {
	return strings.HasSuffix(path, "/deploy") || strings.Contains(path, "/deploy/")
}

// containsStop checks if the path contains /stop
func containsStop(path string) bool {
	return strings.HasSuffix(path, "/stop") || strings.Contains(path, "/stop/")
}

// containsRestart checks if the path contains /restart
func containsRestart(path string) bool {
	return strings.HasSuffix(path, "/restart") || strings.Contains(path, "/restart/")
}

// containsDeployment checks if the path contains /deployment
func containsDeployment(path string) bool {
	return strings.HasSuffix(path, "/deployment") || strings.Contains(path, "/deployment/")
}

// metricsMiddleware ghi lại metrics cho mỗi HTTP request
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		statusStr := http.StatusText(wrapped.statusCode)
		if statusStr == "" {
			statusStr = "unknown"
		}

		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

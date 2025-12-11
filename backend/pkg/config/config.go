package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config chứa tất cả cấu hình của service
type Config struct {
	// Service discovery
	AuthServiceAddr       string
	ProjectServiceAddr    string
	BuildServiceAddr      string
	DeploymentServiceAddr string
	AIServiceAddr         string

	// Server
	ServerPort  string
	GRPCPort    string
	ServiceName string

	// Database
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBMaxOpenConns int
	DBMaxIdleConns int
	DBMaxLifetime  time.Duration

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret     string
	JWTExpiration time.Duration

	// GitHub OAuth & Webhook
	GitHubClientID           string
	GitHubClientSecret       string
	GitHubRedirectURL        string
	GitHubWebhookSecret      string
	GitHubWebhookCallbackURL string

	// Encryption Keys
	EncryptionKey       string // AES-256 key for secrets (Project Service)
	MasterEncryptionKey string // Alias for EncryptionKey

	// LLM API (cho AI Service)
	LLMAPIKey string
	LLMAPIURL string

	// Logging
	LogLevel  string
	LogFormat string // "json" hoặc "console"
}

// LoadConfig tải cấu hình từ biến môi trường
func LoadConfig() (*Config, error) {
	cfg := &Config{
		AuthServiceAddr:       getEnv("AUTH_SERVICE_ADDR", "auth-service:50051"),
		ProjectServiceAddr:    getEnv("PROJECT_SERVICE_ADDR", "project-service:50052"),
		BuildServiceAddr:      getEnv("BUILD_SERVICE_ADDR", "build-service:50053"),
		DeploymentServiceAddr: getEnv("DEPLOYMENT_SERVICE_ADDR", "deployment-service:50055"),
		AIServiceAddr:         getEnv("AI_SERVICE_ADDR", "ai-service:50056"),
		ServerPort:            getEnv("SERVER_PORT", "8080"),
		GRPCPort:              getEnv("GRPC_PORT", "50051"),
		ServiceName:           getEnv("SERVICE_NAME", "unknown-service"),

		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "nexus"),
		DBPassword:     getEnv("DB_PASSWORD", ""),
		DBName:         getEnv("DB_NAME", ""),
		DBMaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		DBMaxLifetime:  getEnvAsDuration("DB_MAX_LIFETIME", 5*time.Minute),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		JWTSecret:     getEnv("JWT_SECRET", ""),
		JWTExpiration: getEnvAsDuration("JWT_EXPIRATION", 15*time.Minute),

		GitHubClientID:           getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:       getEnv("GITHUB_CLIENT_SECRET", ""),
		GitHubRedirectURL:        getEnv("GITHUB_REDIRECT_URL", ""),
		GitHubWebhookSecret:      getEnv("GITHUB_WEBHOOK_SECRET", ""),
		GitHubWebhookCallbackURL: getEnv("GITHUB_WEBHOOK_CALLBACK_URL", "http://localhost:8000/webhooks/github"),

		EncryptionKey:       getEnv("ENCRYPTION_KEY", getEnv("MASTER_ENCRYPTION_KEY", "")),
		MasterEncryptionKey: getEnv("MASTER_ENCRYPTION_KEY", getEnv("ENCRYPTION_KEY", "")),

		LLMAPIKey: getEnv("LLM_API_KEY", ""),
		LLMAPIURL: getEnv("LLM_API_URL", ""),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
	}

	return cfg, nil
}

// getEnv lấy giá trị biến môi trường hoặc trả về giá trị mặc định
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt lấy giá trị biến môi trường dạng int hoặc trả về giá trị mặc định
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsDuration lấy giá trị biến môi trường dạng duration hoặc trả về giá trị mặc định
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetDSN trả về Data Source Name cho PostgreSQL
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

// GetRedisAddr trả về địa chỉ Redis
func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

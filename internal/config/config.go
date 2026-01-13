// Package config provides configuration management for the transaction service.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the transaction service
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Kafka       KafkaConfig
	Auth        AuthConfig
	Encryption  EncryptionConfig
	Logging     LoggingConfig
	Tracing     TracingConfig
	Resilience  ResilienceConfig
	Transaction TransactionConfig
	RateLimit   RateLimitConfig
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	MaxRequestSize  int64         `mapstructure:"max_request_size"`
}

// DatabaseConfig holds PostgreSQL connection settings
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// DSN returns the PostgreSQL connection string
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DefaultTTL   time.Duration `mapstructure:"default_ttl"`
}

// Addr returns the Redis address
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// KafkaConfig holds Kafka connection settings
type KafkaConfig struct {
	Brokers                 []string `mapstructure:"brokers"`
	ConsumerGroupID         string   `mapstructure:"consumer_group_id"`
	EnableIdempotent        bool     `mapstructure:"enable_idempotent"`
	
	// Topics
	TransferInitiatedTopic  string   `mapstructure:"transfer_initiated_topic"`
	TransferApprovedTopic   string   `mapstructure:"transfer_approved_topic"`
	TransferRejectedTopic   string   `mapstructure:"transfer_rejected_topic"`
	TransferCompletedTopic  string   `mapstructure:"transfer_completed_topic"`
	TransferWaitingTopic    string   `mapstructure:"transfer_waiting_topic"`
	FraudAnalysisTopic      string   `mapstructure:"fraud_analysis_topic"`
	FraudReviewTopic        string   `mapstructure:"fraud_review_topic"`
	AuditTopic              string   `mapstructure:"audit_topic"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	JWTPublicKeyPath string        `mapstructure:"jwt_public_key_path"`
	TokenExpiry      time.Duration `mapstructure:"token_expiry"`
	Issuer           string        `mapstructure:"issuer"`
}

// EncryptionConfig holds encryption settings
type EncryptionConfig struct {
	EncryptionKeysBase64 string `mapstructure:"encryption_keys"`
	CurrentKeyVersion    int    `mapstructure:"current_key_version"`
	AuditHMACSecret      string `mapstructure:"audit_hmac_secret"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level           string `mapstructure:"level"`
	Format          string `mapstructure:"format"`
	OutputPath      string `mapstructure:"output_path"`
	EnablePIIMask   bool   `mapstructure:"enable_pii_mask"`
	EnableRequestID bool   `mapstructure:"enable_request_id"`
}

// TracingConfig holds distributed tracing settings
type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	ServiceName  string  `mapstructure:"service_name"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	SampleRate   float64 `mapstructure:"sample_rate"`
}

// ResilienceConfig holds circuit breaker and retry settings
type ResilienceConfig struct {
	CircuitBreakerMaxRequests       uint32        `mapstructure:"cb_max_requests"`
	CircuitBreakerInterval          time.Duration `mapstructure:"cb_interval"`
	CircuitBreakerTimeout           time.Duration `mapstructure:"cb_timeout"`
	CircuitBreakerFailureRatio      float64       `mapstructure:"cb_failure_ratio"`
	RetryMaxAttempts                int           `mapstructure:"retry_max_attempts"`
	RetryInitialInterval            time.Duration `mapstructure:"retry_initial_interval"`
	RetryMaxInterval                time.Duration `mapstructure:"retry_max_interval"`
}

// TransactionConfig holds transaction-specific settings
type TransactionConfig struct {
	MaxTransferAmount       float64       `mapstructure:"max_transfer_amount"`
	MinTransferAmount       float64       `mapstructure:"min_transfer_amount"`
	DefaultDailyLimit       float64       `mapstructure:"default_daily_limit"`
	ReservationTimeout      time.Duration `mapstructure:"reservation_timeout"`
	FraudAnalysisTimeout    time.Duration `mapstructure:"fraud_analysis_timeout"`
	AutoApproveThreshold    float64       `mapstructure:"auto_approve_threshold"`
	AutoRejectThreshold     float64       `mapstructure:"auto_reject_threshold"`
	ManualReviewThreshold   float64       `mapstructure:"manual_review_threshold"`
	CompensationRetryCount  int           `mapstructure:"compensation_retry_count"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	RequestsPerMinute  int           `mapstructure:"requests_per_minute"`
	TransfersPerMinute int           `mapstructure:"transfers_per_minute"`
	BurstSize          int           `mapstructure:"burst_size"`
	WindowDuration     time.Duration `mapstructure:"window_duration"`
}

// Load reads configuration from environment variables and config files
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read from environment variables
	v.SetEnvPrefix("TRANSACTION_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Try to read config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/transaction-service")

	if err := v.ReadInConfig(); err != nil {
		// Config file is optional
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8081)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.max_request_size", 1048576) // 1MB

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.database", "transactions_db")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("database.conn_max_idle_time", "5m")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 2)
	v.SetDefault("redis.default_ttl", "15m")

	// Kafka defaults
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.consumer_group_id", "transaction-service")
	v.SetDefault("kafka.enable_idempotent", true)
	v.SetDefault("kafka.transfer_initiated_topic", "banking.transfers.initiated")
	v.SetDefault("kafka.transfer_approved_topic", "banking.transfers.approved")
	v.SetDefault("kafka.transfer_rejected_topic", "banking.transfers.rejected")
	v.SetDefault("kafka.transfer_completed_topic", "banking.transfers.completed")
	v.SetDefault("kafka.transfer_waiting_topic", "banking.transfers.waitingreview")
	v.SetDefault("kafka.fraud_analysis_topic", "banking.fraud.analysis.complete")
	v.SetDefault("kafka.fraud_review_topic", "banking.fraud.review.complete")
	v.SetDefault("kafka.audit_topic", "banking.audit.transactions")

	// Auth defaults
	v.SetDefault("auth.jwt_public_key_path", "./keys/jwt_public.pem")
	v.SetDefault("auth.token_expiry", "1h")
	v.SetDefault("auth.issuer", "banking-auth-service")

	// Encryption defaults
	v.SetDefault("encryption.current_key_version", 1)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output_path", "stdout")
	v.SetDefault("logging.enable_pii_mask", true)
	v.SetDefault("logging.enable_request_id", true)

	// Tracing defaults
	v.SetDefault("tracing.enabled", true)
	v.SetDefault("tracing.service_name", "transaction-service")
	v.SetDefault("tracing.otlp_endpoint", "localhost:4317")
	v.SetDefault("tracing.sample_rate", 0.1)

	// Resilience defaults
	v.SetDefault("resilience.cb_max_requests", 5)
	v.SetDefault("resilience.cb_interval", "10s")
	v.SetDefault("resilience.cb_timeout", "60s")
	v.SetDefault("resilience.cb_failure_ratio", 0.6)
	v.SetDefault("resilience.retry_max_attempts", 3)
	v.SetDefault("resilience.retry_initial_interval", "100ms")
	v.SetDefault("resilience.retry_max_interval", "2s")

	// Transaction defaults
	v.SetDefault("transaction.max_transfer_amount", 100000.0)
	v.SetDefault("transaction.min_transfer_amount", 0.01)
	v.SetDefault("transaction.default_daily_limit", 50000.0)
	v.SetDefault("transaction.reservation_timeout", "24h")
	v.SetDefault("transaction.fraud_analysis_timeout", "30s")
	v.SetDefault("transaction.auto_approve_threshold", 50.0)
	v.SetDefault("transaction.auto_reject_threshold", 80.0)
	v.SetDefault("transaction.manual_review_threshold", 50.0)
	v.SetDefault("transaction.compensation_retry_count", 3)

	// Rate limit defaults
	v.SetDefault("ratelimit.enabled", true)
	v.SetDefault("ratelimit.requests_per_minute", 1000)
	v.SetDefault("ratelimit.transfers_per_minute", 100)
	v.SetDefault("ratelimit.burst_size", 50)
	v.SetDefault("ratelimit.window_duration", "1m")
}

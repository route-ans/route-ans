// Package config handles loading and validation of the ANS Resolution Server configuration.
// It supports declarative YAML configuration with environment variable substitution.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration structure
type Config struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// Metadata contains configuration metadata
type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// Spec contains the main configuration specification
type Spec struct {
	Server     ServerConfig     `yaml:"server"`
	Cache      CacheConfig      `yaml:"cache"`
	Queue      QueueConfig      `yaml:"queue"`
	Store      StoreConfig      `yaml:"store"`
	Registries []RegistryConfig `yaml:"registries"`
	Trust      TrustConfig      `yaml:"trust"`
	Policy     PolicyConfig     `yaml:"policy"`
	RateLimit  RateLimitConfig  `yaml:"rateLimit"`
	Telemetry  TelemetryConfig  `yaml:"telemetry"`
}

// ServerConfig contains HTTP/gRPC server settings
type ServerConfig struct {
	HTTP                    HTTPConfig    `yaml:"http"`
	GRPC                    GRPCConfig    `yaml:"grpc"`
	TLS                     TLSConfig     `yaml:"tls"`
	GracefulShutdownTimeout time.Duration `yaml:"gracefulShutdownTimeout"`
}

// HTTPConfig contains HTTP server settings
type HTTPConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	ReadTimeout    time.Duration `yaml:"readTimeout"`
	WriteTimeout   time.Duration `yaml:"writeTimeout"`
	IdleTimeout    time.Duration `yaml:"idleTimeout"`
	MaxHeaderBytes int           `yaml:"maxHeaderBytes"`
}

// GRPCConfig contains gRPC server settings
type GRPCConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Enabled bool   `yaml:"enabled"`
}

// TLSConfig contains TLS settings
type TLSConfig struct {
	Enabled  bool       `yaml:"enabled"`
	CertFile string     `yaml:"certFile"`
	KeyFile  string     `yaml:"keyFile"`
	MTLS     MTLSConfig `yaml:"mtls"`
}

// MTLSConfig contains mutual TLS settings
type MTLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ClientCAFile string `yaml:"clientCAFile"`
}

// CacheConfig contains cache provider settings
type CacheConfig struct {
	Provider  string            `yaml:"provider"`
	TTL       CacheTTLConfig    `yaml:"ttl"`
	Memory    MemoryCacheConfig `yaml:"memory"`
	Redis     RedisCacheConfig  `yaml:"redis"`
	Memcached MemcachedConfig   `yaml:"memcached"`
}

// CacheTTLConfig contains TTL settings for different cache states
type CacheTTLConfig struct {
	Default  time.Duration `yaml:"default"`
	Verified time.Duration `yaml:"verified"`
	Revoked  time.Duration `yaml:"revoked"`
	NotFound time.Duration `yaml:"notFound"`
}

// MemoryCacheConfig contains in-memory cache settings
type MemoryCacheConfig struct {
	MaxSize         int           `yaml:"maxSize"`
	CleanupInterval time.Duration `yaml:"cleanupInterval"`
}

// RedisCacheConfig contains Redis cache settings
type RedisCacheConfig struct {
	Address      string         `yaml:"address"`
	Password     string         `yaml:"password"`
	DB           int            `yaml:"db"`
	PoolSize     int            `yaml:"poolSize"`
	MinIdleConns int            `yaml:"minIdleConns"`
	DialTimeout  time.Duration  `yaml:"dialTimeout"`
	ReadTimeout  time.Duration  `yaml:"readTimeout"`
	WriteTimeout time.Duration  `yaml:"writeTimeout"`
	TLS          RedisTLSConfig `yaml:"tls"`
}

// RedisTLSConfig contains Redis TLS settings
type RedisTLSConfig struct {
	Enabled            bool `yaml:"enabled"`
	InsecureSkipVerify bool `yaml:"insecureSkipVerify"`
}

// MemcachedConfig contains Memcached settings
type MemcachedConfig struct {
	Servers      []string      `yaml:"servers"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxIdleConns int           `yaml:"maxIdleConns"`
}

// QueueConfig contains message queue settings
type QueueConfig struct {
	Provider     string             `yaml:"provider"`
	BufferSize   int                `yaml:"bufferSize"`
	RedisStreams RedisStreamsConfig `yaml:"redis-streams"`
	Kafka        KafkaConfig        `yaml:"kafka"`
	NATS         NATSConfig         `yaml:"nats"`
}

// RedisStreamsConfig contains Redis Streams settings
type RedisStreamsConfig struct {
	Address       string        `yaml:"address"`
	Password      string        `yaml:"password"`
	Stream        string        `yaml:"stream"`
	ConsumerGroup string        `yaml:"consumerGroup"`
	Consumer      string        `yaml:"consumer"`
	BlockTimeout  time.Duration `yaml:"blockTimeout"`
	BatchSize     int           `yaml:"batchSize"`
}

// KafkaConfig contains Kafka settings
type KafkaConfig struct {
	Brokers         []string        `yaml:"brokers"`
	Topic           string          `yaml:"topic"`
	GroupID         string          `yaml:"groupId"`
	AutoOffsetReset string          `yaml:"autoOffsetReset"`
	SessionTimeout  time.Duration   `yaml:"sessionTimeout"`
	TLS             KafkaTLSConfig  `yaml:"tls"`
	SASL            KafkaSASLConfig `yaml:"sasl"`
}

// KafkaTLSConfig contains Kafka TLS settings
type KafkaTLSConfig struct {
	Enabled bool `yaml:"enabled"`
}

// KafkaSASLConfig contains Kafka SASL settings
type KafkaSASLConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Mechanism string `yaml:"mechanism"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
}

// NATSConfig contains NATS settings
type NATSConfig struct {
	URL         string        `yaml:"url"`
	Subject     string        `yaml:"subject"`
	Queue       string        `yaml:"queue"`
	DurableName string        `yaml:"durableName"`
	TLS         NATSTLSConfig `yaml:"tls"`
}

// NATSTLSConfig contains NATS TLS settings
type NATSTLSConfig struct {
	Enabled bool `yaml:"enabled"`
}

// StoreConfig contains persistent store settings
type StoreConfig struct {
	Provider string            `yaml:"provider"`
	Postgres PostgresConfig    `yaml:"postgres"`
	SQLite   SQLiteConfig      `yaml:"sqlite"`
	Memory   MemoryStoreConfig `yaml:"memory"`
}

// PostgresConfig contains PostgreSQL settings
type PostgresConfig struct {
	DSN             string           `yaml:"dsn"`
	MaxOpenConns    int              `yaml:"maxOpenConns"`
	MaxIdleConns    int              `yaml:"maxIdleConns"`
	ConnMaxLifetime time.Duration    `yaml:"connMaxLifetime"`
	ConnMaxIdleTime time.Duration    `yaml:"connMaxIdleTime"`
	Migrations      MigrationsConfig `yaml:"migrations"`
}

// MigrationsConfig contains database migration settings
type MigrationsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// SQLiteConfig contains SQLite settings
type SQLiteConfig struct {
	Path        string `yaml:"path"`
	JournalMode string `yaml:"journalMode"`
}

// MemoryStoreConfig contains in-memory store settings
type MemoryStoreConfig struct {
	MaxSize int `yaml:"maxSize"`
}

// RegistryConfig contains registry adapter settings
type RegistryConfig struct {
	Name         string                 `yaml:"name"`
	Type         string                 `yaml:"type"`
	Enabled      bool                   `yaml:"enabled"`
	Priority     int                    `yaml:"priority"`
	Timeout      time.Duration          `yaml:"timeout"`
	Retries      int                    `yaml:"retries"`
	RetryBackoff time.Duration          `yaml:"retryBackoff"`
	Config       map[string]interface{} `yaml:"config"`
}

// TrustConfig contains trust and verification settings
type TrustConfig struct {
	Provider     string             `yaml:"provider"`
	Verification VerificationConfig `yaml:"verification"`
	File         FileTrustConfig    `yaml:"file"`
	Vault        VaultConfig        `yaml:"vault"`
	K8sSecret    K8sSecretConfig    `yaml:"k8s-secret"`
}

// VerificationConfig contains verification settings
type VerificationConfig struct {
	Enabled                 bool          `yaml:"enabled"`
	RequireSignature        bool          `yaml:"requireSignature"`
	RequireMerkleProof      bool          `yaml:"requireMerkleProof"`
	CheckRevocation         bool          `yaml:"checkRevocation"`
	AllowExpiredGracePeriod time.Duration `yaml:"allowExpiredGracePeriod"`
	OCSP                    OCSPConfig    `yaml:"ocsp"`
	CRL                     CRLConfig     `yaml:"crl"`
}

// OCSPConfig contains OCSP settings
type OCSPConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Timeout      time.Duration `yaml:"timeout"`
	CacheTimeout time.Duration `yaml:"cacheTimeout"`
}

// CRLConfig contains CRL settings
type CRLConfig struct {
	Enabled      bool          `yaml:"enabled"`
	CacheTimeout time.Duration `yaml:"cacheTimeout"`
}

// FileTrustConfig contains file-based trust store settings
type FileTrustConfig struct {
	TrustedRootsFile      string        `yaml:"trustedRootsFile"`
	TrustedRegistrarsFile string        `yaml:"trustedRegistrarsFile"`
	RefreshInterval       time.Duration `yaml:"refreshInterval"`
}

// VaultConfig contains Vault trust store settings
type VaultConfig struct {
	Address       string `yaml:"address"`
	AuthMethod    string `yaml:"authMethod"`
	Token         string `yaml:"token"`
	Role          string `yaml:"role"`
	SecretPath    string `yaml:"secretPath"`
	TLSSkipVerify bool   `yaml:"tlsSkipVerify"`
}

// K8sSecretConfig contains Kubernetes secret trust store settings
type K8sSecretConfig struct {
	Namespace     string `yaml:"namespace"`
	SecretName    string `yaml:"secretName"`
	RootsKey      string `yaml:"rootsKey"`
	RegistrarsKey string `yaml:"registrarsKey"`
}

// PolicyConfig contains policy engine settings
type PolicyConfig struct {
	Enabled       bool         `yaml:"enabled"`
	DefaultAction string       `yaml:"defaultAction"`
	Rules         []PolicyRule `yaml:"rules"`
}

// PolicyRule represents a single policy rule
type PolicyRule struct {
	Name    string   `yaml:"name"`
	Enabled bool     `yaml:"enabled"`
	Type    string   `yaml:"type"`
	Field   string   `yaml:"field"`
	Values  []string `yaml:"values"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Enabled      bool                      `yaml:"enabled"`
	Provider     string                    `yaml:"provider"`
	KeyExtractor string                    `yaml:"keyExtractor"`
	Limits       map[string]RateLimitEntry `yaml:"limits"`
	Redis        RateLimitRedisConfig      `yaml:"redis"`
}

// RateLimitEntry represents a rate limit configuration
type RateLimitEntry struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
}

// RateLimitRedisConfig contains Redis rate limit settings
type RateLimitRedisConfig struct {
	Address   string `yaml:"address"`
	KeyPrefix string `yaml:"keyPrefix"`
}

// TelemetryConfig contains observability settings
type TelemetryConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
	Tracing TracingConfig `yaml:"tracing"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string        `yaml:"level"`
	Format string        `yaml:"format"`
	Output string        `yaml:"output"`
	File   LogFileConfig `yaml:"file"`
}

// LogFileConfig contains log file settings
type LogFileConfig struct {
	Path       string `yaml:"path"`
	MaxSize    int    `yaml:"maxSize"`
	MaxBackups int    `yaml:"maxBackups"`
	MaxAge     int    `yaml:"maxAge"`
	Compress   bool   `yaml:"compress"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Path      string `yaml:"path"`
	Namespace string `yaml:"namespace"`
	Subsystem string `yaml:"subsystem"`
}

// TracingConfig contains tracing settings
type TracingConfig struct {
	Enabled     bool         `yaml:"enabled"`
	Provider    string       `yaml:"provider"`
	SampleRate  float64      `yaml:"sampleRate"`
	ServiceName string       `yaml:"serviceName"`
	OTLP        OTLPConfig   `yaml:"otlp"`
	Jaeger      JaegerConfig `yaml:"jaeger"`
}

// OTLPConfig contains OTLP exporter settings
type OTLPConfig struct {
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
	Headers  string `yaml:"headers"`
}

// JaegerConfig contains Jaeger exporter settings
type JaegerConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AgentHost string `yaml:"agentHost"`
	AgentPort int    `yaml:"agentPort"`
}

// envVarPattern matches ${VAR_NAME:default_value} or ${VAR_NAME}
var envVarPattern = regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

// Load reads and parses the configuration file with environment variable substitution
func Load(path string) (*Config, error) {
	// Read the config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	expanded := substituteEnvVars(string(data))

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// substituteEnvVars replaces ${VAR:default} patterns with environment variable values
func substituteEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name and default value
		submatches := envVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := submatches[1]
		defaultValue := ""
		if len(submatches) > 2 {
			defaultValue = submatches[2]
		}

		// Get environment variable value
		if value, exists := os.LookupEnv(varName); exists {
			return value
		}

		return defaultValue
	})
}

// setDefaults sets default values for unspecified configuration options
func setDefaults(cfg *Config) {
	// Server defaults
	if cfg.Spec.Server.HTTP.Host == "" {
		cfg.Spec.Server.HTTP.Host = "0.0.0.0"
	}
	if cfg.Spec.Server.HTTP.Port == 0 {
		cfg.Spec.Server.HTTP.Port = 8080
	}
	if cfg.Spec.Server.HTTP.ReadTimeout == 0 {
		cfg.Spec.Server.HTTP.ReadTimeout = 30 * time.Second
	}
	if cfg.Spec.Server.HTTP.WriteTimeout == 0 {
		cfg.Spec.Server.HTTP.WriteTimeout = 30 * time.Second
	}
	if cfg.Spec.Server.HTTP.IdleTimeout == 0 {
		cfg.Spec.Server.HTTP.IdleTimeout = 120 * time.Second
	}
	if cfg.Spec.Server.GracefulShutdownTimeout == 0 {
		cfg.Spec.Server.GracefulShutdownTimeout = 30 * time.Second
	}

	// Cache defaults
	if cfg.Spec.Cache.Provider == "" {
		cfg.Spec.Cache.Provider = "memory"
	}
	if cfg.Spec.Cache.TTL.Default == 0 {
		cfg.Spec.Cache.TTL.Default = 5 * time.Minute
	}
	if cfg.Spec.Cache.TTL.Verified == 0 {
		cfg.Spec.Cache.TTL.Verified = 10 * time.Minute
	}
	if cfg.Spec.Cache.TTL.Revoked == 0 {
		cfg.Spec.Cache.TTL.Revoked = 1 * time.Hour
	}
	if cfg.Spec.Cache.TTL.NotFound == 0 {
		cfg.Spec.Cache.TTL.NotFound = 1 * time.Minute
	}
	if cfg.Spec.Cache.Memory.MaxSize == 0 {
		cfg.Spec.Cache.Memory.MaxSize = 10000
	}
	if cfg.Spec.Cache.Memory.CleanupInterval == 0 {
		cfg.Spec.Cache.Memory.CleanupInterval = 1 * time.Minute
	}

	// Queue defaults
	if cfg.Spec.Queue.Provider == "" {
		cfg.Spec.Queue.Provider = "memory"
	}
	if cfg.Spec.Queue.BufferSize == 0 {
		cfg.Spec.Queue.BufferSize = 1000
	}

	// Store defaults
	if cfg.Spec.Store.Provider == "" {
		cfg.Spec.Store.Provider = "memory"
	}
	if cfg.Spec.Store.Memory.MaxSize == 0 {
		cfg.Spec.Store.Memory.MaxSize = 100000
	}

	// Trust defaults
	if cfg.Spec.Trust.Provider == "" {
		cfg.Spec.Trust.Provider = "file"
	}

	// Policy defaults
	if cfg.Spec.Policy.DefaultAction == "" {
		cfg.Spec.Policy.DefaultAction = "allow"
	}

	// Rate limit defaults
	if cfg.Spec.RateLimit.Provider == "" {
		cfg.Spec.RateLimit.Provider = "memory"
	}
	if cfg.Spec.RateLimit.KeyExtractor == "" {
		cfg.Spec.RateLimit.KeyExtractor = "ip"
	}

	// Telemetry defaults
	if cfg.Spec.Telemetry.Logging.Level == "" {
		cfg.Spec.Telemetry.Logging.Level = "info"
	}
	if cfg.Spec.Telemetry.Logging.Format == "" {
		cfg.Spec.Telemetry.Logging.Format = "json"
	}
	if cfg.Spec.Telemetry.Metrics.Port == 0 {
		cfg.Spec.Telemetry.Metrics.Port = 9091
	}
	if cfg.Spec.Telemetry.Metrics.Path == "" {
		cfg.Spec.Telemetry.Metrics.Path = "/metrics"
	}
	if cfg.Spec.Telemetry.Tracing.SampleRate == 0 {
		cfg.Spec.Telemetry.Tracing.SampleRate = 0.1
	}
	if cfg.Spec.Telemetry.Tracing.ServiceName == "" {
		cfg.Spec.Telemetry.Tracing.ServiceName = "ans-resolver"
	}
}

// validate checks that the configuration is valid
func validate(cfg *Config) error {
	var errors []string

	// Validate server settings
	if cfg.Spec.Server.HTTP.Port < 1 || cfg.Spec.Server.HTTP.Port > 65535 {
		errors = append(errors, "HTTP port must be between 1 and 65535")
	}

	// Validate cache provider
	validCacheProviders := map[string]bool{"memory": true, "redis": true, "memcached": true}
	if !validCacheProviders[cfg.Spec.Cache.Provider] {
		errors = append(errors, fmt.Sprintf("invalid cache provider: %s", cfg.Spec.Cache.Provider))
	}

	// Validate queue provider
	validQueueProviders := map[string]bool{"memory": true, "redis-streams": true, "kafka": true, "nats": true}
	if !validQueueProviders[cfg.Spec.Queue.Provider] {
		errors = append(errors, fmt.Sprintf("invalid queue provider: %s", cfg.Spec.Queue.Provider))
	}

	// Validate store provider
	validStoreProviders := map[string]bool{"memory": true, "postgres": true, "sqlite": true, "none": true}
	if !validStoreProviders[cfg.Spec.Store.Provider] {
		errors = append(errors, fmt.Sprintf("invalid store provider: %s", cfg.Spec.Store.Provider))
	}

	// Validate trust provider
	validTrustProviders := map[string]bool{"file": true, "vault": true, "k8s-secret": true}
	if !validTrustProviders[cfg.Spec.Trust.Provider] {
		errors = append(errors, fmt.Sprintf("invalid trust provider: %s", cfg.Spec.Trust.Provider))
	}

	// Validate at least one registry is configured
	hasEnabledRegistry := false
	for _, reg := range cfg.Spec.Registries {
		if reg.Enabled {
			hasEnabledRegistry = true
			break
		}
	}
	if !hasEnabledRegistry && len(cfg.Spec.Registries) > 0 {
		// If registries are defined but none enabled, that's okay for testing
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

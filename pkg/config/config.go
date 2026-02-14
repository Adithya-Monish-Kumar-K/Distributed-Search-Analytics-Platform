// Package config loads and validates application configuration from YAML files
// with environment-variable overrides. It provides typed structs for every
// subsystem (Server, Postgres, Kafka, Redis, Indexer, Search, Gateway, etc.).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level application configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Postgres PostgresConfig `yaml:"postgres"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Redis    RedisConfig    `yaml:"redis"`
	Indexer  IndexerConfig  `yaml:"indexer"`
	Search   SearchConfig   `yaml:"search"`
	Gateway  GatewayConfig  `yaml:"gateway"`
	Logging  LoggingConfig  `yaml:"logging"`
	Tracing  TracingConfig  `yaml:"tracing"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
}

// PostgresConfig holds PostgreSQL connection parameters.
type PostgresConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Database        string        `yaml:"database"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"sslMode"`
	MaxOpenConns    int           `yaml:"maxOpenConns"`
	MaxIdleConns    int           `yaml:"maxIdleConns"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime"`
}

// DSN returns a lib/pq-compatible data source name.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode,
	)
}

// KafkaConfig holds Kafka broker and topic settings.
type KafkaConfig struct {
	Brokers       []string    `yaml:"brokers"`
	ConsumerGroup string      `yaml:"consumerGroup"`
	Topics        KafkaTopics `yaml:"topics"`
}

// KafkaTopics maps logical topic names to their Kafka topic strings.
type KafkaTopics struct {
	DocumentIngest  string `yaml:"documentIngest"`
	IndexComplete   string `yaml:"indexComplete"`
	CacheInvalidate string `yaml:"cacheInvalidate"`
	AnalyticsEvents string `yaml:"analyticsEvents"`
}

// RedisConfig holds Redis connection and caching parameters.
type RedisConfig struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	PoolSize int           `yaml:"poolSize"`
	CacheTTL time.Duration `yaml:"cacheTTL"`
}

// IndexerConfig controls the indexing engine's memory thresholds, flush
// intervals, and segment merge policy.
type IndexerConfig struct {
	DataDir                string        `yaml:"dataDir"`
	SegmentMaxSize         int64         `yaml:"segmentMaxSize"`
	MergeInterval          time.Duration `yaml:"mergeInterval"`
	FlushInterval          time.Duration `yaml:"flushInterval"`
	MaxSegmentsBeforeMerge int           `yaml:"maxSegmentsBeforeMerge"`
}

// SearchConfig controls query execution limits and timeouts.
type SearchConfig struct {
	MaxResults           int           `yaml:"maxResults"`
	DefaultLimit         int           `yaml:"defaultLimit"`
	TimeoutPerShard      time.Duration `yaml:"timeoutPerShard"`
	MaxConcurrentQueries int           `yaml:"maxConcurrentQueries"`
}

// LoggingConfig controls structured logging level and output format.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// TracingConfig controls distributed tracing (sample rate, endpoint).
type TracingConfig struct {
	Enabled    bool    `yaml:"enabled"`
	Endpoint   string  `yaml:"endpoint"`
	SampleRate float64 `yaml:"sampleRate"`
}

// MetricsConfig controls the Prometheus metrics server.
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// GatewayConfig holds the API gateway port and upstream service URLs.
type GatewayConfig struct {
	Port         int    `yaml:"port"`
	IngestionURL string `yaml:"ingestionUrl"`
	SearcherURL  string `yaml:"searcherUrl"`
}

// Load reads a YAML config file (if provided) and applies environment-variable
// overrides. It returns a Config populated with sensible defaults for any
// missing values.
func Load(path string) (*Config, error) {
	cfg := defaultConfig()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", path, err)
		}
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

// defaultConfig returns a Config with production-ready defaults for local
// development.
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Postgres: PostgresConfig{
			Host:            "localhost",
			Port:            5432,
			Database:        "searchplatform",
			User:            "searchplatform",
			Password:        "localdev",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Kafka: KafkaConfig{
			Brokers:       []string{"localhost:9092"},
			ConsumerGroup: "searchplatform-group",
			Topics: KafkaTopics{
				DocumentIngest:  "document-ingest",
				IndexComplete:   "index.complete",
				CacheInvalidate: "cache-invalidate",
				AnalyticsEvents: "analytics-events",
			},
		},
		Redis: RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
			CacheTTL: 60 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    9090,
		},
		Gateway: GatewayConfig{
			Port:         8082,
			IngestionURL: "http://localhost:8081",
			SearcherURL:  "http://localhost:8080",
		},
	}
}

// applyEnvOverrides reads SP_* environment variables and overrides the
// corresponding config fields.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SP_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("SP_POSTGRES_HOST"); v != "" {
		cfg.Postgres.Host = v
	}
	if v := os.Getenv("SP_POSTGRES_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Postgres.Port = port
		}
	}
	if v := os.Getenv("SP_POSTGRES_DATABASE"); v != "" {
		cfg.Postgres.Database = v
	}
	if v := os.Getenv("SP_POSTGRES_USER"); v != "" {
		cfg.Postgres.User = v
	}
	if v := os.Getenv("SP_POSTGRES_PASSWORD"); v != "" {
		cfg.Postgres.Password = v
	}
	if v := os.Getenv("SP_POSTGRES_SSLMODE"); v != "" {
		cfg.Postgres.SSLMode = v
	}
	if v := os.Getenv("SP_KAFKA_BROKERS"); v != "" {
		cfg.Kafka.Brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("SP_REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("SP_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("SP_LOGGING_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("SP_LOGGING_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
	if v := os.Getenv("SP_GATEWAY_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Gateway.Port = port
		}
	}
	if v := os.Getenv("SP_GATEWAY_INGESTION_URL"); v != "" {
		cfg.Gateway.IngestionURL = v
	}
	if v := os.Getenv("SP_GATEWAY_SEARCHER_URL"); v != "" {
		cfg.Gateway.SearcherURL = v
	}
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Postgres PostgresConfig `yaml:"postgres"`
	Kafka    KafkaConfig    `yaml:"kafka"`
	Redis    RedisConfig    `yaml:"redis"`
	Indexer  IndexerConfig  `yaml:"indexer"`
	Search   SearchConfig   `yaml:"search"`
	Logging  LoggingConfig  `yaml:"logging"`
	Tracing  TracingConfig  `yaml:"tracing"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"readTimeout"`
	WriteTimeout    time.Duration `yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"`
}

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

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.Database, p.SSLMode,
	)
}

type KafkaConfig struct {
	Brokers       []string    `yaml:"brokers"`
	ConsumerGroup string      `yaml:"consumerGroup"`
	Topics        KafkaTopics `yaml:"topics"`
}

type KafkaTopics struct {
	DocumentIngest  string `yaml:"documentIngest"`
	IndexComplete   string `yaml:"indexComplete"`
	CacheInvalidate string `yaml:"cacheInvalidate"`
	AnalyticsEvents string `yaml:"analyticsEvents"`
}

type RedisConfig struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	PoolSize int           `yaml:"poolSize"`
	CacheTTL time.Duration `yaml:"cacheTTL"`
}

type IndexerConfig struct {
	DataDir                string        `yaml:"dataDir"`
	SEgmentMaxSize         int64         `yaml:"segmentMaxSize"`
	MergeInterval          time.Duration `yaml:"mergeInterval"`
	FlushInterval          time.Duration `yaml:"flushInterval"`
	MaxSegmentsBeforeMerge int           `yaml:"maxSegmentsBeforeMerge"`
}

type SearchConfig struct {
	MaxResults           int           `yaml:"maxResults"`
	DefaultLimit         int           `yaml:"defaultLimit"`
	TimeoutPerShard      time.Duration `yaml:"timeoutPerShard"`
	MaxConcurrentQueries int           `yaml:"maxConcurrentQueries"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type TracingConfig struct {
	Enabled    bool    `yaml:"enabled"`
	Endpoint   string  `yaml:"endpoint"`
	SampleRate float64 `yaml:"sampleRate"`
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

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
	}
}

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
}

package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Global     GlobalConfig     `yaml:"global"`
	Strategies []StrategyConfig `yaml:"strategies"`
}

// GlobalConfig contains default configurations for all strategies
type GlobalConfig struct {
	Slack       SlackConfig      `yaml:"slack"`
	LogLevel    string           `yaml:"log_level"`
	Schedule    string           `yaml:"schedule"`
	Retention   string           `yaml:"retention"`
	Timezone    string           `yaml:"timezone"`
	TempDir     string           `yaml:"temp_dir"`
	MaxParallel int              `yaml:"max_parallel_strategies"`
	Retry       RetryConfig      `yaml:"retry"`
	Timeout     TimeoutConfig    `yaml:"timeout"`
	S3          S3Config         `yaml:"s3"`
	Monitoring  MonitoringConfig `yaml:"monitoring"`
}

// SlackConfig contains Slack notification settings
type SlackConfig struct {
	BotToken  string `yaml:"bot_token"`
	ChannelID string `yaml:"channel_id"`
}

// RetryConfig contains retry settings
type RetryConfig struct {
	MaxAttempts int `yaml:"max_attempts"`
}

// TimeoutConfig contains timeout settings
type TimeoutConfig struct {
	Backup string `yaml:"backup"`
	Upload string `yaml:"upload"`
}

// S3Config contains S3 storage settings
type S3Config struct {
	Bucket      string        `yaml:"bucket"`
	BasePath    string        `yaml:"base_path"`
	Compression string        `yaml:"compression"`
	Endpoint    string        `yaml:"endpoint,omitempty"` // Custom endpoint for MinIO/S3-compatible storage
	Credentials S3Credentials `yaml:"credentials"`
}

// S3Credentials contains AWS credentials
type S3Credentials struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

// MonitoringConfig contains monitoring settings
type MonitoringConfig struct {
	Metrics     MetricsConfig     `yaml:"metrics"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// MetricsConfig contains Prometheus metrics settings
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// HealthCheckConfig contains health check settings
type HealthCheckConfig struct {
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

// StrategyConfig contains configuration for a specific backup strategy
type StrategyConfig struct {
	Name         string      `yaml:"name"`
	DatabaseType string      `yaml:"database_type"` // postgres, mysql, mongodb
	DatabaseURL  string      `yaml:"database_url"`
	Schedule     string      `yaml:"schedule,omitempty"`
	Retention    string      `yaml:"retention,omitempty"`
	Slack        SlackConfig `yaml:"slack,omitempty"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	configData := substituteEnvVars(string(data))

	var config Config
	if err := yaml.Unmarshal([]byte(configData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if err := setDefaults(&config); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	// Load Slack configuration from environment variables
	LoadSlackFromEnv(&config)

	return &config, nil
}

// LoadSlackFromEnv loads Slack configuration from environment variables
func LoadSlackFromEnv(config *Config) {
	// Load global Slack config from environment
	if botToken := os.Getenv("SLACK_BOT_TOKEN"); botToken != "" {
		config.Global.Slack.BotToken = botToken
	}
	if channelID := os.Getenv("SLACK_CHANNEL_ID"); channelID != "" {
		config.Global.Slack.ChannelID = channelID
	}

	// Override strategy-specific Slack config if environment variables are set
	for i := range config.Strategies {
		if config.Global.Slack.BotToken != "" && config.Strategies[i].Slack.BotToken == "" {
			config.Strategies[i].Slack.BotToken = config.Global.Slack.BotToken
		}
		if config.Global.Slack.ChannelID != "" && config.Strategies[i].Slack.ChannelID == "" {
			config.Strategies[i].Slack.ChannelID = config.Global.Slack.ChannelID
		}
	}
}

// setDefaults sets default values for missing configuration
func setDefaults(config *Config) error {
	// Set global defaults
	if config.Global.LogLevel == "" {
		config.Global.LogLevel = "info"
	}
	if config.Global.Schedule == "" {
		config.Global.Schedule = "1d"
	}
	if config.Global.Retention == "" {
		config.Global.Retention = "30d"
	}
	if config.Global.Timezone == "" {
		config.Global.Timezone = "UTC"
	}
	if config.Global.TempDir == "" {
		config.Global.TempDir = "/tmp/db-backup"
	}
	if config.Global.MaxParallel == 0 {
		config.Global.MaxParallel = 2
	}
	if config.Global.Retry.MaxAttempts == 0 {
		config.Global.Retry.MaxAttempts = 3
	}
	if config.Global.Timeout.Backup == "" {
		config.Global.Timeout.Backup = "30m"
	}
	if config.Global.Timeout.Upload == "" {
		config.Global.Timeout.Upload = "10m"
	}
	if config.Global.S3.Compression == "" {
		config.Global.S3.Compression = "gzip"
	}
	if config.Global.Monitoring.Metrics.Port == 0 {
		config.Global.Monitoring.Metrics.Port = 8080
	}
	if config.Global.Monitoring.Metrics.Path == "" {
		config.Global.Monitoring.Metrics.Path = "/metrics"
	}
	if config.Global.Monitoring.HealthCheck.Port == 0 {
		config.Global.Monitoring.HealthCheck.Port = 8080
	}
	if config.Global.Monitoring.HealthCheck.Path == "" {
		config.Global.Monitoring.HealthCheck.Path = "/health"
	}

	// Apply global defaults to strategies
	for i := range config.Strategies {
		strategy := &config.Strategies[i]
		if strategy.DatabaseType == "" {
			strategy.DatabaseType = "postgres" // Default to postgres for backward compatibility
		}
		// Validate database type
		switch strategy.DatabaseType {
		case "postgres", "mysql", "mariadb", "mongodb":
			// Valid database types
		default:
			return fmt.Errorf("unsupported database type '%s' for strategy '%s'. Supported types: postgres, mysql, mariadb, mongodb", strategy.DatabaseType, strategy.Name)
		}
		if strategy.Schedule == "" {
			strategy.Schedule = config.Global.Schedule
		}
		if strategy.Retention == "" {
			strategy.Retention = config.Global.Retention
		}
		if strategy.Slack.BotToken == "" {
			strategy.Slack.BotToken = config.Global.Slack.BotToken
		}
		if strategy.Slack.ChannelID == "" {
			strategy.Slack.ChannelID = config.Global.Slack.ChannelID
		}
	}

	return nil
}

// ParseDuration parses duration strings like "1h", "1d", "1w"
func ParseDuration(duration string) (time.Duration, error) {
	if len(duration) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", duration)
	}

	unit := duration[len(duration)-1:]
	value := duration[:len(duration)-1]

	var multiplier time.Duration
	switch unit {
	case "h":
		multiplier = time.Hour
	case "d":
		multiplier = 24 * time.Hour
	case "w":
		multiplier = 7 * 24 * time.Hour
	default:
		return time.ParseDuration(duration)
	}

	// Parse the numeric part
	var count int
	_, err := fmt.Sscanf(value, "%d", &count)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %s", duration)
	}

	return time.Duration(count) * multiplier, nil
}

// substituteEnvVars replaces ${VAR} patterns with environment variable values
func substituteEnvVars(input string) string {
	// Regular expression to match ${VAR} patterns
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")

		// Get environment variable value
		if value := os.Getenv(varName); value != "" {
			return value
		}

		// Return original if not found (keep ${VAR} format)
		return match
	})
}

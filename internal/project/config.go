package project

import "time"

const CurrentSchemaVersion = 1

type Config struct {
	SchemaVersion int                 `yaml:"schema_version"`
	Project       ProjectSection      `yaml:"project"`
	Runtime       RuntimeConfig       `yaml:"runtime"`
	Architecture  ArchitectureConfig  `yaml:"architecture"`
	API           APIConfig           `yaml:"api"`
	OpenAPI       OpenAPIConfig       `yaml:"openapi"`
	Database      DatabaseConfig      `yaml:"database"`
	Auth          AuthConfig          `yaml:"auth"`
	RateLimit     RateLimitConfig     `yaml:"rate_limit"`
	Cache         CacheConfig         `yaml:"cache"`
	Messaging     MessagingConfig     `yaml:"messaging"`
	Outbox        OutboxConfig        `yaml:"outbox"`
	Observability ObservabilityConfig `yaml:"observability"`
	Performance   PerformanceConfig   `yaml:"performance"`
	Deployment    DeploymentConfig    `yaml:"deployment"`
	Quality       QualityConfig       `yaml:"quality"`
}

type ProjectSection struct {
	Name          string `yaml:"name"`
	Module        string `yaml:"module"`
	Preset        string `yaml:"preset"`
	PresetVersion string `yaml:"preset_version,omitempty"`

	// GoVersion is kept for project.yaml v1 compatibility. New configurations
	// use runtime.go.language instead.
	GoVersion string `yaml:"go_version,omitempty"`
}

type RuntimeConfig struct {
	Go GoRuntimeConfig `yaml:"go"`
}

type GoRuntimeConfig struct {
	Language  string `yaml:"language"`
	Toolchain string `yaml:"toolchain"`
}

func (c Config) GoLanguageVersion() string {
	if c.Runtime.Go.Language != "" {
		return c.Runtime.Go.Language
	}
	return c.Project.GoVersion
}

func (c Config) GoToolchainVersion() string {
	if c.Runtime.Go.Toolchain != "" {
		return c.Runtime.Go.Toolchain
	}
	return PreferredToolchain(c.GoLanguageVersion())
}

type ArchitectureConfig struct {
	Type   string `yaml:"type"`
	Layout string `yaml:"layout"`
}

type APIConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Router          string        `yaml:"router"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	MaxBodySize     int64         `yaml:"max_body_size"`
}

type OpenAPIConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Source            string `yaml:"source"`
	StrictServer      bool   `yaml:"strict_server"`
	RequestValidation bool   `yaml:"request_validation"`
	Documentation     string `yaml:"documentation"`
}

type DatabaseConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Engine         string `yaml:"engine"`
	Driver         string `yaml:"driver"`
	Pool           string `yaml:"pool"`
	Migrations     string `yaml:"migrations"`
	CodeGeneration string `yaml:"code_generation"`
}

type AuthConfig struct {
	Enabled      bool               `yaml:"enabled"`
	Strategy     string             `yaml:"strategy"`
	AccessToken  AccessTokenConfig  `yaml:"access_token"`
	RefreshToken RefreshTokenConfig `yaml:"refresh_token"`
	RBAC         bool               `yaml:"rbac"`
	MFA          bool               `yaml:"mfa"`
}

type AccessTokenConfig struct {
	TTL        time.Duration `yaml:"ttl"`
	Algorithm  string        `yaml:"algorithm"`
	Issuer     string        `yaml:"issuer"`
	Audience   string        `yaml:"audience"`
	Revocation string        `yaml:"revocation"`
}

type RefreshTokenConfig struct {
	TTL            time.Duration `yaml:"ttl"`
	Storage        string        `yaml:"storage"`
	Rotation       bool          `yaml:"rotation"`
	ReuseDetection bool          `yaml:"reuse_detection"`
	Transport      string        `yaml:"transport"`
}

type RateLimitConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Strategy          string        `yaml:"strategy"`
	RequestsPerSecond int           `yaml:"requests_per_second"`
	Burst             int           `yaml:"burst"`
	Key               string        `yaml:"key"`
	EntryTTL          time.Duration `yaml:"entry_ttl"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval"`
}

type CacheConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Address  string `yaml:"address"`
	DB       int    `yaml:"db"`
}

type MessagingConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Provider      string        `yaml:"provider"`
	Brokers       string        `yaml:"brokers"`
	TopicPrefix   string        `yaml:"topic_prefix"`
	ConsumerGroup string        `yaml:"consumer_group"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryBackoff  time.Duration `yaml:"retry_backoff"`
	DLQSuffix     string        `yaml:"dlq_suffix"`
}

type OutboxConfig struct {
	Enabled      bool          `yaml:"enabled"`
	PollInterval time.Duration `yaml:"poll_interval"`
	BatchSize    int           `yaml:"batch_size"`
	MaxAttempts  int           `yaml:"max_attempts"`
}

type ObservabilityConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
	Tracing TracingConfig `yaml:"tracing"`
}

type LoggingConfig struct {
	Provider          string `yaml:"provider"`
	DevelopmentFormat string `yaml:"development_format"`
	ProductionFormat  string `yaml:"production_format"`
}

type MetricsConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Endpoint string `yaml:"endpoint"`
}

type TracingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Exporter string `yaml:"exporter"`
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
}

type PerformanceConfig struct {
	Pprof PprofConfig `yaml:"pprof"`
}

type PprofConfig struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
}

type DeploymentConfig struct {
	Docker       bool   `yaml:"docker"`
	Compose      bool   `yaml:"compose"`
	Kubernetes   bool   `yaml:"kubernetes"`
	Namespace    string `yaml:"namespace"`
	Replicas     int    `yaml:"replicas"`
	RuntimeImage string `yaml:"runtime_image"`
	NonRoot      bool   `yaml:"non_root"`
}

type QualityConfig struct {
	Coverage CoverageConfig `yaml:"coverage"`
}

type CoverageConfig struct {
	Minimum int `yaml:"minimum"`
}

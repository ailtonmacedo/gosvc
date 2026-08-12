package project

import (
	"time"

	presetpkg "github.com/ailtonmacedo/gosvc/internal/preset"
)

// DefaultConfig returns the defaults for the minimal-api preset. Required
// project identity fields are intentionally left empty.
func DefaultConfig() Config {
	return DefaultConfigForPreset("minimal-api")
}

// DefaultConfigForPreset returns defaults resolved for a built-in preset.
func DefaultConfigForPreset(name string) Config {
	presetVersion, _ := presetpkg.CurrentVersion(name)
	config := Config{
		SchemaVersion: CurrentSchemaVersion,
		Project:       ProjectSection{GoVersion: "auto", Preset: name, PresetVersion: presetVersion},
		Runtime:       RuntimeConfig{Go: GoRuntimeConfig{Language: "auto"}},
		Architecture:  ArchitectureConfig{Type: "clean", Layout: "layered"},
		API: APIConfig{
			Enabled: true, Router: "chi", Port: 8080,
			ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second,
			IdleTimeout: 60 * time.Second, ShutdownTimeout: 10 * time.Second,
			MaxBodySize: 1 << 20,
		},
		OpenAPI:  OpenAPIConfig{Source: "api/openapi.yaml", StrictServer: true, Documentation: "redoc"},
		Database: DatabaseConfig{Engine: "postgres", Driver: "pgx", Pool: "pgxpool", Migrations: "golang-migrate", CodeGeneration: "sqlc"},
		Auth: AuthConfig{
			Strategy:     "jwt",
			AccessToken:  AccessTokenConfig{TTL: 15 * time.Minute, Algorithm: "HS256", Issuer: "service", Audience: "service-api", Revocation: "none"},
			RefreshToken: RefreshTokenConfig{TTL: 7 * 24 * time.Hour, Storage: "postgres", Rotation: true, ReuseDetection: true, Transport: "http_only_cookie"},
		},
		RateLimit: RateLimitConfig{Strategy: "local", RequestsPerSecond: 10, Burst: 20, Key: "ip", EntryTTL: 10 * time.Minute, CleanupInterval: time.Minute},
		Cache:     CacheConfig{Provider: "redis", Address: "localhost:6379"},
		Messaging: MessagingConfig{Provider: "kafka", Brokers: "localhost:9092", TopicPrefix: "events", ConsumerGroup: "service-workers", MaxRetries: 5, RetryBackoff: time.Second, DLQSuffix: ".dlq"},
		Outbox:    OutboxConfig{PollInterval: time.Second, BatchSize: 100, MaxAttempts: 10},
		Observability: ObservabilityConfig{
			Logging: LoggingConfig{Provider: "slog", DevelopmentFormat: "text", ProductionFormat: "json"},
			Metrics: MetricsConfig{Provider: "prometheus", Endpoint: "/metrics"},
			Tracing: TracingConfig{Provider: "opentelemetry", Exporter: "otlp-grpc", Endpoint: "localhost:4317", Insecure: true},
		},
		Performance: PerformanceConfig{Pprof: PprofConfig{Address: "127.0.0.1:6060"}},
		Deployment:  DeploymentConfig{Docker: true, Namespace: "default", Replicas: 2, RuntimeImage: "distroless", NonRoot: true},
		Quality:     QualityConfig{Coverage: CoverageConfig{Minimum: 80}},
	}
	if name == "bare" || name == "worker" {
		config.API.Enabled = false
		config.OpenAPI.Enabled = false
		config.Database.Enabled = false
		config.Deployment.Compose = false
	}
	if name == "postgres-api" || name == "production-api" || name == "event-driven-api" {
		config.Project.GoVersion = "1.25.0"
		config.Runtime.Go.Language = "1.25.0"
		config.Runtime.Go.Toolchain = PreferredToolchain("1.25.0")
		config.OpenAPI.Enabled = true
		config.OpenAPI.RequestValidation = true
		config.Database.Enabled = true
		config.Deployment.Compose = true
	}
	if name == "production-api" || name == "event-driven-api" {
		config.Auth.Enabled = true
		config.Auth.RBAC = true
		config.Auth.AccessToken.Issuer = "production-api"
		config.Auth.AccessToken.Audience = "production-api"
		config.RateLimit.Enabled = true
		config.Observability.Metrics.Enabled = true
		config.Observability.Tracing.Enabled = true
	}
	if name == "event-driven-api" {
		config.Cache.Enabled = true
		config.Messaging.Enabled = true
		config.Outbox.Enabled = true
		config.Deployment.Kubernetes = true
		config.Deployment.Namespace = "default"
	}
	return config
}

// ApplyDefaults fills zero-valued scalar defaults. Callers constructing a
// Config programmatically should prefer DefaultConfigForPreset because
// booleans cannot distinguish omitted values from explicit false values.
func (c *Config) ApplyDefaults() {
	preset := c.Project.Preset
	if preset == "" {
		preset = "minimal-api"
	}
	defaults := DefaultConfigForPreset(preset)
	if c.SchemaVersion == 0 {
		c.SchemaVersion = defaults.SchemaVersion
	}
	if c.Project.GoVersion == "" {
		c.Project.GoVersion = defaults.Project.GoVersion
	}
	if c.Runtime.Go.Language == "" {
		if c.Project.GoVersion != "" {
			c.Runtime.Go.Language = c.Project.GoVersion
		} else {
			c.Runtime.Go.Language = defaults.Runtime.Go.Language
		}
	}
	if c.Project.GoVersion == "" || (c.Project.GoVersion == "auto" && c.Runtime.Go.Language != "auto") {
		c.Project.GoVersion = c.Runtime.Go.Language
	}
	if c.Runtime.Go.Toolchain == "" {
		if defaults.Runtime.Go.Toolchain != "" && c.Runtime.Go.Language == defaults.Runtime.Go.Language {
			c.Runtime.Go.Toolchain = defaults.Runtime.Go.Toolchain
		} else {
			c.Runtime.Go.Toolchain = PreferredToolchain(c.Runtime.Go.Language)
		}
	}
	if c.Project.Preset == "" {
		c.Project.Preset = defaults.Project.Preset
	}
	if c.Project.PresetVersion == "" {
		c.Project.PresetVersion = defaults.Project.PresetVersion
	}
	if c.Architecture.Type == "" {
		c.Architecture.Type = defaults.Architecture.Type
	}
	if c.Architecture.Layout == "" {
		c.Architecture.Layout = defaults.Architecture.Layout
	}
	if c.API.Router == "" {
		c.API.Router = defaults.API.Router
	}
	if c.API.Port == 0 {
		c.API.Port = defaults.API.Port
	}
	if c.API.ReadTimeout == 0 {
		c.API.ReadTimeout = defaults.API.ReadTimeout
	}
	if c.API.WriteTimeout == 0 {
		c.API.WriteTimeout = defaults.API.WriteTimeout
	}
	if c.API.IdleTimeout == 0 {
		c.API.IdleTimeout = defaults.API.IdleTimeout
	}
	if c.API.ShutdownTimeout == 0 {
		c.API.ShutdownTimeout = defaults.API.ShutdownTimeout
	}
	if c.API.MaxBodySize == 0 {
		c.API.MaxBodySize = defaults.API.MaxBodySize
	}
	if c.OpenAPI.Source == "" {
		c.OpenAPI.Source = defaults.OpenAPI.Source
	}
	if c.OpenAPI.Documentation == "" {
		c.OpenAPI.Documentation = defaults.OpenAPI.Documentation
	}
	if c.Database.Engine == "" {
		c.Database.Engine = defaults.Database.Engine
	}
	if c.Database.Driver == "" {
		c.Database.Driver = defaults.Database.Driver
	}
	if c.Database.Pool == "" {
		c.Database.Pool = defaults.Database.Pool
	}
	if c.Database.Migrations == "" {
		c.Database.Migrations = defaults.Database.Migrations
	}
	if c.Database.CodeGeneration == "" {
		c.Database.CodeGeneration = defaults.Database.CodeGeneration
	}
	if c.Auth.Strategy == "" {
		c.Auth.Strategy = defaults.Auth.Strategy
	}
	if c.Auth.AccessToken.TTL == 0 {
		c.Auth.AccessToken.TTL = defaults.Auth.AccessToken.TTL
	}
	if c.Auth.AccessToken.Algorithm == "" {
		c.Auth.AccessToken.Algorithm = defaults.Auth.AccessToken.Algorithm
	}
	if c.Auth.AccessToken.Issuer == "" {
		c.Auth.AccessToken.Issuer = defaults.Auth.AccessToken.Issuer
	}
	if c.Auth.AccessToken.Audience == "" {
		c.Auth.AccessToken.Audience = defaults.Auth.AccessToken.Audience
	}
	if c.Auth.AccessToken.Revocation == "" {
		c.Auth.AccessToken.Revocation = defaults.Auth.AccessToken.Revocation
	}
	if c.Auth.RefreshToken.TTL == 0 {
		c.Auth.RefreshToken.TTL = defaults.Auth.RefreshToken.TTL
	}
	if c.Auth.RefreshToken.Storage == "" {
		c.Auth.RefreshToken.Storage = defaults.Auth.RefreshToken.Storage
	}
	if c.Auth.RefreshToken.Transport == "" {
		c.Auth.RefreshToken.Transport = defaults.Auth.RefreshToken.Transport
	}
	if c.RateLimit.Strategy == "" {
		c.RateLimit.Strategy = defaults.RateLimit.Strategy
	}
	if c.RateLimit.RequestsPerSecond == 0 {
		c.RateLimit.RequestsPerSecond = defaults.RateLimit.RequestsPerSecond
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = defaults.RateLimit.Burst
	}
	if c.RateLimit.Key == "" {
		c.RateLimit.Key = defaults.RateLimit.Key
	}
	if c.RateLimit.EntryTTL == 0 {
		c.RateLimit.EntryTTL = defaults.RateLimit.EntryTTL
	}
	if c.RateLimit.CleanupInterval == 0 {
		c.RateLimit.CleanupInterval = defaults.RateLimit.CleanupInterval
	}
	if c.Cache.Provider == "" {
		c.Cache.Provider = defaults.Cache.Provider
	}
	if c.Cache.Address == "" {
		c.Cache.Address = defaults.Cache.Address
	}
	if c.Messaging.Provider == "" {
		c.Messaging.Provider = defaults.Messaging.Provider
	}
	if c.Messaging.Brokers == "" {
		c.Messaging.Brokers = defaults.Messaging.Brokers
	}
	if c.Messaging.TopicPrefix == "" {
		c.Messaging.TopicPrefix = defaults.Messaging.TopicPrefix
	}
	if c.Messaging.ConsumerGroup == "" {
		c.Messaging.ConsumerGroup = defaults.Messaging.ConsumerGroup
	}
	if c.Messaging.MaxRetries == 0 {
		c.Messaging.MaxRetries = defaults.Messaging.MaxRetries
	}
	if c.Messaging.RetryBackoff == 0 {
		c.Messaging.RetryBackoff = defaults.Messaging.RetryBackoff
	}
	if c.Messaging.DLQSuffix == "" {
		c.Messaging.DLQSuffix = defaults.Messaging.DLQSuffix
	}
	if c.Outbox.PollInterval == 0 {
		c.Outbox.PollInterval = defaults.Outbox.PollInterval
	}
	if c.Outbox.BatchSize == 0 {
		c.Outbox.BatchSize = defaults.Outbox.BatchSize
	}
	if c.Outbox.MaxAttempts == 0 {
		c.Outbox.MaxAttempts = defaults.Outbox.MaxAttempts
	}
	if c.Observability.Logging.Provider == "" {
		c.Observability.Logging.Provider = defaults.Observability.Logging.Provider
	}
	if c.Observability.Logging.DevelopmentFormat == "" {
		c.Observability.Logging.DevelopmentFormat = defaults.Observability.Logging.DevelopmentFormat
	}
	if c.Observability.Logging.ProductionFormat == "" {
		c.Observability.Logging.ProductionFormat = defaults.Observability.Logging.ProductionFormat
	}
	if c.Observability.Metrics.Provider == "" {
		c.Observability.Metrics.Provider = defaults.Observability.Metrics.Provider
	}
	if c.Observability.Metrics.Endpoint == "" {
		c.Observability.Metrics.Endpoint = defaults.Observability.Metrics.Endpoint
	}
	if c.Observability.Tracing.Provider == "" {
		c.Observability.Tracing.Provider = defaults.Observability.Tracing.Provider
	}
	if c.Observability.Tracing.Exporter == "" {
		c.Observability.Tracing.Exporter = defaults.Observability.Tracing.Exporter
	}
	if c.Observability.Tracing.Endpoint == "" {
		c.Observability.Tracing.Endpoint = defaults.Observability.Tracing.Endpoint
	}
	if c.Performance.Pprof.Address == "" {
		c.Performance.Pprof.Address = defaults.Performance.Pprof.Address
	}
	if c.Deployment.Namespace == "" {
		c.Deployment.Namespace = defaults.Deployment.Namespace
	}
	if c.Deployment.Replicas == 0 {
		c.Deployment.Replicas = defaults.Deployment.Replicas
	}
	if c.Deployment.RuntimeImage == "" {
		c.Deployment.RuntimeImage = defaults.Deployment.RuntimeImage
	}
	if c.Quality.Coverage.Minimum == 0 {
		c.Quality.Coverage.Minimum = defaults.Quality.Coverage.Minimum
	}
}

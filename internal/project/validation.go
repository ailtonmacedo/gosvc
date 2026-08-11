package project

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	projectNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	modulePartPattern  = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
)

type FieldError struct {
	Field   string
	Value   any
	Message string
}

func (e FieldError) Error() string {
	if e.Value == nil {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s (value: %v)", e.Field, e.Message, e.Value)
}

type ValidationErrors []error

func (e ValidationErrors) Error() string {
	messages := make([]string, 0, len(e))
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

func (e ValidationErrors) Unwrap() []error { return []error(e) }

func (c Config) Validate() error {
	var validationErrors ValidationErrors
	add := func(field string, value any, message string) {
		validationErrors = append(validationErrors, FieldError{Field: field, Value: value, Message: message})
	}

	if c.SchemaVersion != CurrentSchemaVersion {
		add("schema_version", c.SchemaVersion, fmt.Sprintf("unsupported schema version; expected %d", CurrentSchemaVersion))
	}
	if !projectNamePattern.MatchString(c.Project.Name) {
		add("project.name", c.Project.Name, "must use lowercase kebab-case, for example order-service")
	}
	if !validModulePath(c.Project.Module) {
		add("project.module", c.Project.Module, "must be a valid Go module path, for example github.com/acme/order-service")
	}
	if c.Project.Preset == "" {
		add("project.preset", nil, "is required")
	} else if c.Project.Preset != "minimal-api" && c.Project.Preset != "postgres-api" && c.Project.Preset != "production-api" && c.Project.Preset != "event-driven-api" {
		add("project.preset", c.Project.Preset, "unsupported preset; allowed values: minimal-api, postgres-api, production-api, event-driven-api")
	}
	if c.Architecture.Type != "clean" {
		add("architecture.type", c.Architecture.Type, "unsupported architecture; allowed value: clean")
	}
	if c.Architecture.Layout != "layered" {
		add("architecture.layout", c.Architecture.Layout, "unsupported layout; allowed value: layered")
	}
	if c.API.Enabled {
		if c.API.Router != "chi" {
			add("api.router", c.API.Router, "unsupported router; allowed value: chi")
		}
		if c.API.Port < 1 || c.API.Port > 65535 {
			add("api.port", c.API.Port, "must be between 1 and 65535")
		}
		if c.API.ReadTimeout <= 0 {
			add("api.read_timeout", c.API.ReadTimeout, "must be greater than zero")
		}
		if c.API.WriteTimeout <= 0 {
			add("api.write_timeout", c.API.WriteTimeout, "must be greater than zero")
		}
		if c.API.IdleTimeout <= 0 {
			add("api.idle_timeout", c.API.IdleTimeout, "must be greater than zero")
		}
		if c.API.ShutdownTimeout <= 0 {
			add("api.shutdown_timeout", c.API.ShutdownTimeout, "must be greater than zero")
		}
		if c.API.MaxBodySize <= 0 {
			add("api.max_body_size", c.API.MaxBodySize, "must be greater than zero")
		}
	}
	if c.OpenAPI.Enabled {
		if !c.API.Enabled {
			add("openapi.enabled", c.OpenAPI.Enabled, "requires api.enabled to be true")
		}
		if c.OpenAPI.Source != "api/openapi.yaml" {
			add("openapi.source", c.OpenAPI.Source, "unsupported source; allowed value: api/openapi.yaml")
		}
		if c.OpenAPI.Documentation != "redoc" {
			add("openapi.documentation", c.OpenAPI.Documentation, "unsupported documentation renderer; allowed value: redoc")
		}
	}
	if c.Database.Enabled {
		if c.Database.Engine != "postgres" {
			add("database.engine", c.Database.Engine, "unsupported engine; allowed value: postgres")
		}
		if c.Database.Driver != "pgx" {
			add("database.driver", c.Database.Driver, "unsupported driver; allowed value: pgx")
		}
		if c.Database.Pool != "pgxpool" {
			add("database.pool", c.Database.Pool, "unsupported pool; allowed value: pgxpool")
		}
		if c.Database.Migrations != "golang-migrate" {
			add("database.migrations", c.Database.Migrations, "unsupported migration tool; allowed value: golang-migrate")
		}
		if c.Database.CodeGeneration != "sqlc" {
			add("database.code_generation", c.Database.CodeGeneration, "unsupported code generator; allowed value: sqlc")
		}
	}
	if (c.Project.Preset == "postgres-api" || c.Project.Preset == "production-api" || c.Project.Preset == "event-driven-api") && !c.OpenAPI.Enabled {
		add("openapi.enabled", c.OpenAPI.Enabled, "must be true for database-backed presets")
	}
	if (c.Project.Preset == "postgres-api" || c.Project.Preset == "production-api" || c.Project.Preset == "event-driven-api") && !c.Database.Enabled {
		add("database.enabled", c.Database.Enabled, "must be true for database-backed presets")
	}

	if c.Auth.Enabled {
		if c.Auth.Strategy != "jwt" {
			add("auth.strategy", c.Auth.Strategy, "unsupported strategy; allowed value: jwt")
		}
		if c.Auth.AccessToken.TTL <= 0 {
			add("auth.access_token.ttl", c.Auth.AccessToken.TTL, "must be greater than zero")
		}
		if c.Auth.AccessToken.Algorithm != "HS256" {
			add("auth.access_token.algorithm", c.Auth.AccessToken.Algorithm, "unsupported algorithm; allowed value: HS256")
		}
		if strings.TrimSpace(c.Auth.AccessToken.Issuer) == "" {
			add("auth.access_token.issuer", c.Auth.AccessToken.Issuer, "must not be empty")
		}
		if strings.TrimSpace(c.Auth.AccessToken.Audience) == "" {
			add("auth.access_token.audience", c.Auth.AccessToken.Audience, "must not be empty")
		}
		if c.Auth.AccessToken.Revocation != "none" && c.Auth.AccessToken.Revocation != "redis-denylist" {
			add("auth.access_token.revocation", c.Auth.AccessToken.Revocation, "allowed values: none, redis-denylist")
		}
		if c.Auth.RefreshToken.TTL <= 0 {
			add("auth.refresh_token.ttl", c.Auth.RefreshToken.TTL, "must be greater than zero")
		}
		if c.Auth.RefreshToken.Storage != "postgres" {
			add("auth.refresh_token.storage", c.Auth.RefreshToken.Storage, "unsupported storage; allowed value: postgres")
		}
		if c.Auth.RefreshToken.Transport != "http_only_cookie" {
			add("auth.refresh_token.transport", c.Auth.RefreshToken.Transport, "unsupported transport; allowed value: http_only_cookie")
		}
		if !c.Database.Enabled {
			add("auth.enabled", c.Auth.Enabled, "requires database.enabled to be true")
		}
	}
	if c.RateLimit.Enabled {
		if c.RateLimit.Strategy != "local" {
			add("rate_limit.strategy", c.RateLimit.Strategy, "unsupported strategy; allowed value: local")
		}
		if c.RateLimit.RequestsPerSecond <= 0 {
			add("rate_limit.requests_per_second", c.RateLimit.RequestsPerSecond, "must be greater than zero")
		}
		if c.RateLimit.Burst <= 0 {
			add("rate_limit.burst", c.RateLimit.Burst, "must be greater than zero")
		}
		if c.RateLimit.Key != "ip" && c.RateLimit.Key != "user" {
			add("rate_limit.key", c.RateLimit.Key, "allowed values: ip, user")
		}
		if c.RateLimit.EntryTTL <= 0 {
			add("rate_limit.entry_ttl", c.RateLimit.EntryTTL, "must be greater than zero")
		}
		if c.RateLimit.CleanupInterval <= 0 {
			add("rate_limit.cleanup_interval", c.RateLimit.CleanupInterval, "must be greater than zero")
		}
	}
	if c.Cache.Enabled {
		if c.Cache.Provider != "redis" {
			add("cache.provider", c.Cache.Provider, "unsupported provider; allowed value: redis")
		}
		if strings.TrimSpace(c.Cache.Address) == "" {
			add("cache.address", c.Cache.Address, "must not be empty")
		}
		if c.Cache.DB < 0 {
			add("cache.db", c.Cache.DB, "must be zero or greater")
		}
	}
	if c.Messaging.Enabled {
		if c.Messaging.Provider != "kafka" {
			add("messaging.provider", c.Messaging.Provider, "unsupported provider; allowed value: kafka")
		}
		if strings.TrimSpace(c.Messaging.Brokers) == "" {
			add("messaging.brokers", c.Messaging.Brokers, "must not be empty")
		}
		if strings.TrimSpace(c.Messaging.TopicPrefix) == "" {
			add("messaging.topic_prefix", c.Messaging.TopicPrefix, "must not be empty")
		}
		if strings.TrimSpace(c.Messaging.ConsumerGroup) == "" {
			add("messaging.consumer_group", c.Messaging.ConsumerGroup, "must not be empty")
		}
		if c.Messaging.MaxRetries < 0 {
			add("messaging.max_retries", c.Messaging.MaxRetries, "must be zero or greater")
		}
		if c.Messaging.RetryBackoff <= 0 {
			add("messaging.retry_backoff", c.Messaging.RetryBackoff, "must be greater than zero")
		}
		if strings.TrimSpace(c.Messaging.DLQSuffix) == "" {
			add("messaging.dlq_suffix", c.Messaging.DLQSuffix, "must not be empty")
		}
	}
	if c.Outbox.Enabled {
		if !c.Database.Enabled {
			add("outbox.enabled", c.Outbox.Enabled, "requires database.enabled to be true")
		}
		if !c.Messaging.Enabled {
			add("outbox.enabled", c.Outbox.Enabled, "requires messaging.enabled to be true")
		}
		if c.Outbox.PollInterval <= 0 {
			add("outbox.poll_interval", c.Outbox.PollInterval, "must be greater than zero")
		}
		if c.Outbox.BatchSize <= 0 {
			add("outbox.batch_size", c.Outbox.BatchSize, "must be greater than zero")
		}
		if c.Outbox.MaxAttempts <= 0 {
			add("outbox.max_attempts", c.Outbox.MaxAttempts, "must be greater than zero")
		}
	}
	if c.Observability.Logging.Provider != "slog" {
		add("observability.logging.provider", c.Observability.Logging.Provider, "unsupported provider; allowed value: slog")
	}
	if c.Observability.Logging.DevelopmentFormat != "text" {
		add("observability.logging.development_format", c.Observability.Logging.DevelopmentFormat, "unsupported format; allowed value: text")
	}
	if c.Observability.Logging.ProductionFormat != "json" {
		add("observability.logging.production_format", c.Observability.Logging.ProductionFormat, "unsupported format; allowed value: json")
	}
	if c.Observability.Metrics.Enabled {
		if c.Observability.Metrics.Provider != "prometheus" {
			add("observability.metrics.provider", c.Observability.Metrics.Provider, "unsupported provider; allowed value: prometheus")
		}
		if !strings.HasPrefix(c.Observability.Metrics.Endpoint, "/") {
			add("observability.metrics.endpoint", c.Observability.Metrics.Endpoint, "must start with /")
		}
	}
	if c.Observability.Tracing.Enabled {
		if c.Observability.Tracing.Provider != "opentelemetry" {
			add("observability.tracing.provider", c.Observability.Tracing.Provider, "unsupported provider; allowed value: opentelemetry")
		}
		if c.Observability.Tracing.Exporter != "otlp-grpc" {
			add("observability.tracing.exporter", c.Observability.Tracing.Exporter, "unsupported exporter; allowed value: otlp-grpc")
		}
		if strings.TrimSpace(c.Observability.Tracing.Endpoint) == "" {
			add("observability.tracing.endpoint", c.Observability.Tracing.Endpoint, "must not be empty")
		}
	}
	if c.Performance.Pprof.Enabled && strings.TrimSpace(c.Performance.Pprof.Address) == "" {
		add("performance.pprof.address", c.Performance.Pprof.Address, "must not be empty when pprof is enabled")
	}
	if c.Project.Preset == "production-api" || c.Project.Preset == "event-driven-api" {
		if !c.Auth.Enabled {
			add("auth.enabled", c.Auth.Enabled, "must be true for production-api")
		}
		if !c.RateLimit.Enabled {
			add("rate_limit.enabled", c.RateLimit.Enabled, "must be true for production-api")
		}
		if !c.Observability.Metrics.Enabled {
			add("observability.metrics.enabled", c.Observability.Metrics.Enabled, "must be true for production-api")
		}
	}

	if c.Project.Preset == "event-driven-api" {
		if !c.Cache.Enabled {
			add("cache.enabled", c.Cache.Enabled, "must be true for event-driven-api")
		}
		if !c.Messaging.Enabled {
			add("messaging.enabled", c.Messaging.Enabled, "must be true for event-driven-api")
		}
		if !c.Outbox.Enabled {
			add("outbox.enabled", c.Outbox.Enabled, "must be true for event-driven-api")
		}
		if !c.Deployment.Kubernetes {
			add("deployment.kubernetes", c.Deployment.Kubernetes, "must be true for event-driven-api")
		}
	}
	if c.Deployment.Kubernetes {
		if strings.TrimSpace(c.Deployment.Namespace) == "" {
			add("deployment.namespace", c.Deployment.Namespace, "must not be empty")
		}
		if c.Deployment.Replicas <= 0 {
			add("deployment.replicas", c.Deployment.Replicas, "must be greater than zero")
		}
	}

	if c.Quality.Coverage.Minimum < 0 || c.Quality.Coverage.Minimum > 100 {
		add("quality.coverage.minimum", c.Quality.Coverage.Minimum, "must be between 0 and 100")
	}

	if len(validationErrors) > 0 {
		return validationErrors
	}
	return nil
}

func validModulePath(module string) bool {
	if module == "" || strings.ContainsAny(module, " \\") || strings.HasPrefix(module, "/") || strings.HasSuffix(module, "/") {
		return false
	}
	parts := strings.Split(module, "/")
	if len(parts) < 2 || !strings.Contains(parts[0], ".") {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || !modulePartPattern.MatchString(part) {
			return false
		}
	}
	return true
}

func IsValidationError(err error) bool {
	var target ValidationErrors
	return errors.As(err, &target)
}

package project

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type yamlContext struct {
	indent int
	values map[string]any
}

// parseSimpleYAML intentionally supports only the YAML subset used by
// project.yaml: nested mappings with scalar string, integer, and boolean
// values. Lists, anchors, aliases, tags, and multiline values are rejected.
// Keeping this parser local lets Sprint 1 remain dependency-free. It can be
// replaced behind Load when a fully featured YAML dependency is adopted.
func parseSimpleYAML(data []byte) (map[string]any, error) {
	root := make(map[string]any)
	stack := []yamlContext{{indent: 0, values: root}}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := strings.TrimRight(scanner.Text(), " \r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		if trimmed == "..." {
			continue
		}
		if strings.ContainsRune(raw, '\t') {
			return nil, fmt.Errorf("line %d: tabs are not allowed for indentation", lineNumber)
		}
		if strings.HasPrefix(trimmed, "-") {
			return nil, fmt.Errorf("line %d: lists are not supported in project.yaml", lineNumber)
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		if indent%2 != 0 {
			return nil, fmt.Errorf("line %d: indentation must use multiples of two spaces", lineNumber)
		}

		for len(stack) > 1 && stack[len(stack)-1].indent > indent {
			stack = stack[:len(stack)-1]
		}
		if stack[len(stack)-1].indent != indent {
			return nil, fmt.Errorf("line %d: unexpected indentation", lineNumber)
		}

		content := strings.TrimSpace(raw)
		key, rawValue, found := strings.Cut(content, ":")
		if !found {
			return nil, fmt.Errorf("line %d: expected 'key: value'", lineNumber)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: key cannot be empty", lineNumber)
		}
		if strings.ContainsAny(key, "{}[]&,*!|>'\"%@`") {
			return nil, fmt.Errorf("line %d: unsupported key syntax %q", lineNumber, key)
		}

		current := stack[len(stack)-1].values
		if _, exists := current[key]; exists {
			return nil, fmt.Errorf("line %d: duplicate field %q", lineNumber, key)
		}

		rawValue = strings.TrimSpace(rawValue)
		if rawValue == "" {
			child := make(map[string]any)
			current[key] = child
			stack = append(stack, yamlContext{indent: indent + 2, values: child})
			continue
		}
		if strings.Contains(rawValue, " #") {
			rawValue = strings.TrimSpace(strings.SplitN(rawValue, " #", 2)[0])
		}
		value, err := parseScalar(rawValue)
		if err != nil {
			return nil, fmt.Errorf("line %d, field %s: %w", lineNumber, key, err)
		}
		current[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read YAML: %w", err)
	}
	return root, nil
}

func parseScalar(value string) (any, error) {
	if strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") ||
		strings.Contains(value, "&") || strings.Contains(value, "*") {
		return nil, fmt.Errorf("unsupported YAML construct")
	}
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
		(value[0] == '\'' && value[len(value)-1] == '\'')) {
		if value[0] == '\'' {
			return strings.ReplaceAll(value[1:len(value)-1], "''", "'"), nil
		}
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return nil, fmt.Errorf("invalid quoted string: %w", err)
		}
		return decoded, nil
	}
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}
	if number, err := strconv.Atoi(value); err == nil {
		return number, nil
	}
	return value, nil
}

func decodeProjectConfig(root map[string]any) (Config, error) {
	if err := rejectUnknown(root, "", "schema_version", "project", "architecture", "api", "openapi", "database", "auth", "rate_limit", "cache", "messaging", "outbox", "observability", "performance", "deployment", "quality"); err != nil {
		return Config{}, err
	}

	presetName := "minimal-api"
	if section, exists, err := optionalMap(root, "project"); err != nil {
		return Config{}, err
	} else if exists {
		presetName, err = optionalString(section, "preset", presetName, "project.preset")
		if err != nil {
			return Config{}, err
		}
	}
	config := DefaultConfigForPreset(presetName)

	var err error
	if value, exists := root["schema_version"]; exists {
		config.SchemaVersion, err = requireInt(value, "schema_version")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "project"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "project", "name", "module", "go_version", "preset"); err != nil {
			return Config{}, err
		}
		config.Project.Name, err = optionalString(section, "name", config.Project.Name, "project.name")
		if err != nil {
			return Config{}, err
		}
		config.Project.Module, err = optionalString(section, "module", config.Project.Module, "project.module")
		if err != nil {
			return Config{}, err
		}
		config.Project.GoVersion, err = optionalString(section, "go_version", config.Project.GoVersion, "project.go_version")
		if err != nil {
			return Config{}, err
		}
		config.Project.Preset, err = optionalString(section, "preset", config.Project.Preset, "project.preset")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "architecture"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "architecture", "type", "layout"); err != nil {
			return Config{}, err
		}
		config.Architecture.Type, err = optionalString(section, "type", config.Architecture.Type, "architecture.type")
		if err != nil {
			return Config{}, err
		}
		config.Architecture.Layout, err = optionalString(section, "layout", config.Architecture.Layout, "architecture.layout")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "api"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "api", "enabled", "router", "port", "read_timeout", "write_timeout", "idle_timeout", "shutdown_timeout", "max_body_size"); err != nil {
			return Config{}, err
		}
		config.API.Enabled, err = optionalBool(section, "enabled", config.API.Enabled, "api.enabled")
		if err != nil {
			return Config{}, err
		}
		config.API.Router, err = optionalString(section, "router", config.API.Router, "api.router")
		if err != nil {
			return Config{}, err
		}
		config.API.Port, err = optionalInt(section, "port", config.API.Port, "api.port")
		if err != nil {
			return Config{}, err
		}
		config.API.ReadTimeout, err = optionalDuration(section, "read_timeout", config.API.ReadTimeout, "api.read_timeout")
		if err != nil {
			return Config{}, err
		}
		config.API.WriteTimeout, err = optionalDuration(section, "write_timeout", config.API.WriteTimeout, "api.write_timeout")
		if err != nil {
			return Config{}, err
		}
		config.API.IdleTimeout, err = optionalDuration(section, "idle_timeout", config.API.IdleTimeout, "api.idle_timeout")
		if err != nil {
			return Config{}, err
		}
		config.API.ShutdownTimeout, err = optionalDuration(section, "shutdown_timeout", config.API.ShutdownTimeout, "api.shutdown_timeout")
		if err != nil {
			return Config{}, err
		}
		maxBodySize, parseErr := optionalInt(section, "max_body_size", int(config.API.MaxBodySize), "api.max_body_size")
		if parseErr != nil {
			return Config{}, parseErr
		}
		config.API.MaxBodySize = int64(maxBodySize)
	}

	if section, exists, err := optionalMap(root, "openapi"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "openapi", "enabled", "source", "strict_server", "request_validation", "documentation"); err != nil {
			return Config{}, err
		}
		config.OpenAPI.Enabled, err = optionalBool(section, "enabled", config.OpenAPI.Enabled, "openapi.enabled")
		if err != nil {
			return Config{}, err
		}
		config.OpenAPI.Source, err = optionalString(section, "source", config.OpenAPI.Source, "openapi.source")
		if err != nil {
			return Config{}, err
		}
		config.OpenAPI.StrictServer, err = optionalBool(section, "strict_server", config.OpenAPI.StrictServer, "openapi.strict_server")
		if err != nil {
			return Config{}, err
		}
		config.OpenAPI.RequestValidation, err = optionalBool(section, "request_validation", config.OpenAPI.RequestValidation, "openapi.request_validation")
		if err != nil {
			return Config{}, err
		}
		config.OpenAPI.Documentation, err = optionalString(section, "documentation", config.OpenAPI.Documentation, "openapi.documentation")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "database"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "database", "enabled", "engine", "driver", "pool", "migrations", "code_generation"); err != nil {
			return Config{}, err
		}
		config.Database.Enabled, err = optionalBool(section, "enabled", config.Database.Enabled, "database.enabled")
		if err != nil {
			return Config{}, err
		}
		config.Database.Engine, err = optionalString(section, "engine", config.Database.Engine, "database.engine")
		if err != nil {
			return Config{}, err
		}
		config.Database.Driver, err = optionalString(section, "driver", config.Database.Driver, "database.driver")
		if err != nil {
			return Config{}, err
		}
		config.Database.Pool, err = optionalString(section, "pool", config.Database.Pool, "database.pool")
		if err != nil {
			return Config{}, err
		}
		config.Database.Migrations, err = optionalString(section, "migrations", config.Database.Migrations, "database.migrations")
		if err != nil {
			return Config{}, err
		}
		config.Database.CodeGeneration, err = optionalString(section, "code_generation", config.Database.CodeGeneration, "database.code_generation")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "auth"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "auth", "enabled", "strategy", "access_token", "refresh_token", "rbac", "mfa"); err != nil {
			return Config{}, err
		}
		config.Auth.Enabled, err = optionalBool(section, "enabled", config.Auth.Enabled, "auth.enabled")
		if err != nil {
			return Config{}, err
		}
		config.Auth.Strategy, err = optionalString(section, "strategy", config.Auth.Strategy, "auth.strategy")
		if err != nil {
			return Config{}, err
		}
		config.Auth.RBAC, err = optionalBool(section, "rbac", config.Auth.RBAC, "auth.rbac")
		if err != nil {
			return Config{}, err
		}
		config.Auth.MFA, err = optionalBool(section, "mfa", config.Auth.MFA, "auth.mfa")
		if err != nil {
			return Config{}, err
		}
		if access, ok, mapErr := optionalMap(section, "access_token"); mapErr != nil {
			return Config{}, mapErr
		} else if ok {
			if err := rejectUnknown(access, "auth.access_token", "ttl", "algorithm", "issuer", "audience", "revocation"); err != nil {
				return Config{}, err
			}
			config.Auth.AccessToken.TTL, err = optionalDuration(access, "ttl", config.Auth.AccessToken.TTL, "auth.access_token.ttl")
			if err != nil {
				return Config{}, err
			}
			config.Auth.AccessToken.Algorithm, err = optionalString(access, "algorithm", config.Auth.AccessToken.Algorithm, "auth.access_token.algorithm")
			if err != nil {
				return Config{}, err
			}
			config.Auth.AccessToken.Issuer, err = optionalString(access, "issuer", config.Auth.AccessToken.Issuer, "auth.access_token.issuer")
			if err != nil {
				return Config{}, err
			}
			config.Auth.AccessToken.Audience, err = optionalString(access, "audience", config.Auth.AccessToken.Audience, "auth.access_token.audience")
			if err != nil {
				return Config{}, err
			}
			config.Auth.AccessToken.Revocation, err = optionalString(access, "revocation", config.Auth.AccessToken.Revocation, "auth.access_token.revocation")
			if err != nil {
				return Config{}, err
			}
		}
		if refresh, ok, mapErr := optionalMap(section, "refresh_token"); mapErr != nil {
			return Config{}, mapErr
		} else if ok {
			if err := rejectUnknown(refresh, "auth.refresh_token", "ttl", "storage", "rotation", "reuse_detection", "transport"); err != nil {
				return Config{}, err
			}
			config.Auth.RefreshToken.TTL, err = optionalDuration(refresh, "ttl", config.Auth.RefreshToken.TTL, "auth.refresh_token.ttl")
			if err != nil {
				return Config{}, err
			}
			config.Auth.RefreshToken.Storage, err = optionalString(refresh, "storage", config.Auth.RefreshToken.Storage, "auth.refresh_token.storage")
			if err != nil {
				return Config{}, err
			}
			config.Auth.RefreshToken.Rotation, err = optionalBool(refresh, "rotation", config.Auth.RefreshToken.Rotation, "auth.refresh_token.rotation")
			if err != nil {
				return Config{}, err
			}
			config.Auth.RefreshToken.ReuseDetection, err = optionalBool(refresh, "reuse_detection", config.Auth.RefreshToken.ReuseDetection, "auth.refresh_token.reuse_detection")
			if err != nil {
				return Config{}, err
			}
			config.Auth.RefreshToken.Transport, err = optionalString(refresh, "transport", config.Auth.RefreshToken.Transport, "auth.refresh_token.transport")
			if err != nil {
				return Config{}, err
			}
		}
	}

	if section, exists, err := optionalMap(root, "rate_limit"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "rate_limit", "enabled", "strategy", "requests_per_second", "burst", "key", "entry_ttl", "cleanup_interval"); err != nil {
			return Config{}, err
		}
		config.RateLimit.Enabled, err = optionalBool(section, "enabled", config.RateLimit.Enabled, "rate_limit.enabled")
		if err != nil {
			return Config{}, err
		}
		config.RateLimit.Strategy, err = optionalString(section, "strategy", config.RateLimit.Strategy, "rate_limit.strategy")
		if err != nil {
			return Config{}, err
		}
		config.RateLimit.RequestsPerSecond, err = optionalInt(section, "requests_per_second", config.RateLimit.RequestsPerSecond, "rate_limit.requests_per_second")
		if err != nil {
			return Config{}, err
		}
		config.RateLimit.Burst, err = optionalInt(section, "burst", config.RateLimit.Burst, "rate_limit.burst")
		if err != nil {
			return Config{}, err
		}
		config.RateLimit.Key, err = optionalString(section, "key", config.RateLimit.Key, "rate_limit.key")
		if err != nil {
			return Config{}, err
		}
		config.RateLimit.EntryTTL, err = optionalDuration(section, "entry_ttl", config.RateLimit.EntryTTL, "rate_limit.entry_ttl")
		if err != nil {
			return Config{}, err
		}
		config.RateLimit.CleanupInterval, err = optionalDuration(section, "cleanup_interval", config.RateLimit.CleanupInterval, "rate_limit.cleanup_interval")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "cache"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "cache", "enabled", "provider", "address", "db"); err != nil {
			return Config{}, err
		}
		config.Cache.Enabled, err = optionalBool(section, "enabled", config.Cache.Enabled, "cache.enabled")
		if err != nil {
			return Config{}, err
		}
		config.Cache.Provider, err = optionalString(section, "provider", config.Cache.Provider, "cache.provider")
		if err != nil {
			return Config{}, err
		}
		config.Cache.Address, err = optionalString(section, "address", config.Cache.Address, "cache.address")
		if err != nil {
			return Config{}, err
		}
		config.Cache.DB, err = optionalInt(section, "db", config.Cache.DB, "cache.db")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "messaging"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "messaging", "enabled", "provider", "brokers", "topic_prefix", "consumer_group", "max_retries", "retry_backoff", "dlq_suffix"); err != nil {
			return Config{}, err
		}
		config.Messaging.Enabled, err = optionalBool(section, "enabled", config.Messaging.Enabled, "messaging.enabled")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.Provider, err = optionalString(section, "provider", config.Messaging.Provider, "messaging.provider")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.Brokers, err = optionalString(section, "brokers", config.Messaging.Brokers, "messaging.brokers")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.TopicPrefix, err = optionalString(section, "topic_prefix", config.Messaging.TopicPrefix, "messaging.topic_prefix")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.ConsumerGroup, err = optionalString(section, "consumer_group", config.Messaging.ConsumerGroup, "messaging.consumer_group")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.MaxRetries, err = optionalInt(section, "max_retries", config.Messaging.MaxRetries, "messaging.max_retries")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.RetryBackoff, err = optionalDuration(section, "retry_backoff", config.Messaging.RetryBackoff, "messaging.retry_backoff")
		if err != nil {
			return Config{}, err
		}
		config.Messaging.DLQSuffix, err = optionalString(section, "dlq_suffix", config.Messaging.DLQSuffix, "messaging.dlq_suffix")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "outbox"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "outbox", "enabled", "poll_interval", "batch_size", "max_attempts"); err != nil {
			return Config{}, err
		}
		config.Outbox.Enabled, err = optionalBool(section, "enabled", config.Outbox.Enabled, "outbox.enabled")
		if err != nil {
			return Config{}, err
		}
		config.Outbox.PollInterval, err = optionalDuration(section, "poll_interval", config.Outbox.PollInterval, "outbox.poll_interval")
		if err != nil {
			return Config{}, err
		}
		config.Outbox.BatchSize, err = optionalInt(section, "batch_size", config.Outbox.BatchSize, "outbox.batch_size")
		if err != nil {
			return Config{}, err
		}
		config.Outbox.MaxAttempts, err = optionalInt(section, "max_attempts", config.Outbox.MaxAttempts, "outbox.max_attempts")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "observability"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "observability", "logging", "metrics", "tracing"); err != nil {
			return Config{}, err
		}
		if logging, ok, mapErr := optionalMap(section, "logging"); mapErr != nil {
			return Config{}, mapErr
		} else if ok {
			if err := rejectUnknown(logging, "observability.logging", "provider", "development_format", "production_format"); err != nil {
				return Config{}, err
			}
			config.Observability.Logging.Provider, err = optionalString(logging, "provider", config.Observability.Logging.Provider, "observability.logging.provider")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Logging.DevelopmentFormat, err = optionalString(logging, "development_format", config.Observability.Logging.DevelopmentFormat, "observability.logging.development_format")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Logging.ProductionFormat, err = optionalString(logging, "production_format", config.Observability.Logging.ProductionFormat, "observability.logging.production_format")
			if err != nil {
				return Config{}, err
			}
		}
		if metrics, ok, mapErr := optionalMap(section, "metrics"); mapErr != nil {
			return Config{}, mapErr
		} else if ok {
			if err := rejectUnknown(metrics, "observability.metrics", "enabled", "provider", "endpoint"); err != nil {
				return Config{}, err
			}
			config.Observability.Metrics.Enabled, err = optionalBool(metrics, "enabled", config.Observability.Metrics.Enabled, "observability.metrics.enabled")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Metrics.Provider, err = optionalString(metrics, "provider", config.Observability.Metrics.Provider, "observability.metrics.provider")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Metrics.Endpoint, err = optionalString(metrics, "endpoint", config.Observability.Metrics.Endpoint, "observability.metrics.endpoint")
			if err != nil {
				return Config{}, err
			}
		}
		if tracing, ok, mapErr := optionalMap(section, "tracing"); mapErr != nil {
			return Config{}, mapErr
		} else if ok {
			if err := rejectUnknown(tracing, "observability.tracing", "enabled", "provider", "exporter", "endpoint", "insecure"); err != nil {
				return Config{}, err
			}
			config.Observability.Tracing.Enabled, err = optionalBool(tracing, "enabled", config.Observability.Tracing.Enabled, "observability.tracing.enabled")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Tracing.Provider, err = optionalString(tracing, "provider", config.Observability.Tracing.Provider, "observability.tracing.provider")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Tracing.Exporter, err = optionalString(tracing, "exporter", config.Observability.Tracing.Exporter, "observability.tracing.exporter")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Tracing.Endpoint, err = optionalString(tracing, "endpoint", config.Observability.Tracing.Endpoint, "observability.tracing.endpoint")
			if err != nil {
				return Config{}, err
			}
			config.Observability.Tracing.Insecure, err = optionalBool(tracing, "insecure", config.Observability.Tracing.Insecure, "observability.tracing.insecure")
			if err != nil {
				return Config{}, err
			}
		}
	}

	if section, exists, err := optionalMap(root, "performance"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "performance", "pprof"); err != nil {
			return Config{}, err
		}
		if pprof, ok, mapErr := optionalMap(section, "pprof"); mapErr != nil {
			return Config{}, mapErr
		} else if ok {
			if err := rejectUnknown(pprof, "performance.pprof", "enabled", "address"); err != nil {
				return Config{}, err
			}
			config.Performance.Pprof.Enabled, err = optionalBool(pprof, "enabled", config.Performance.Pprof.Enabled, "performance.pprof.enabled")
			if err != nil {
				return Config{}, err
			}
			config.Performance.Pprof.Address, err = optionalString(pprof, "address", config.Performance.Pprof.Address, "performance.pprof.address")
			if err != nil {
				return Config{}, err
			}
		}
	}

	if section, exists, err := optionalMap(root, "deployment"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "deployment", "docker", "compose", "kubernetes", "namespace", "replicas", "runtime_image", "non_root"); err != nil {
			return Config{}, err
		}
		config.Deployment.Docker, err = optionalBool(section, "docker", config.Deployment.Docker, "deployment.docker")
		if err != nil {
			return Config{}, err
		}
		config.Deployment.Compose, err = optionalBool(section, "compose", config.Deployment.Compose, "deployment.compose")
		if err != nil {
			return Config{}, err
		}
		config.Deployment.Kubernetes, err = optionalBool(section, "kubernetes", config.Deployment.Kubernetes, "deployment.kubernetes")
		if err != nil {
			return Config{}, err
		}
		config.Deployment.Namespace, err = optionalString(section, "namespace", config.Deployment.Namespace, "deployment.namespace")
		if err != nil {
			return Config{}, err
		}
		config.Deployment.Replicas, err = optionalInt(section, "replicas", config.Deployment.Replicas, "deployment.replicas")
		if err != nil {
			return Config{}, err
		}
		config.Deployment.RuntimeImage, err = optionalString(section, "runtime_image", config.Deployment.RuntimeImage, "deployment.runtime_image")
		if err != nil {
			return Config{}, err
		}
		config.Deployment.NonRoot, err = optionalBool(section, "non_root", config.Deployment.NonRoot, "deployment.non_root")
		if err != nil {
			return Config{}, err
		}
	}

	if section, exists, err := optionalMap(root, "quality"); err != nil {
		return Config{}, err
	} else if exists {
		if err := rejectUnknown(section, "quality", "coverage"); err != nil {
			return Config{}, err
		}
		if coverage, exists, err := optionalMap(section, "coverage"); err != nil {
			return Config{}, err
		} else if exists {
			if err := rejectUnknown(coverage, "quality.coverage", "minimum"); err != nil {
				return Config{}, err
			}
			config.Quality.Coverage.Minimum, err = optionalInt(coverage, "minimum", config.Quality.Coverage.Minimum, "quality.coverage.minimum")
			if err != nil {
				return Config{}, err
			}
		}
	}

	return config, nil
}

func rejectUnknown(values map[string]any, prefix string, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	for field := range values {
		if _, ok := allowedSet[field]; !ok {
			path := field
			if prefix != "" {
				path = prefix + "." + field
			}
			return fmt.Errorf("field %s not found in project schema", path)
		}
	}
	return nil
}

func optionalMap(values map[string]any, key string) (map[string]any, bool, error) {
	value, exists := values[key]
	if !exists {
		return nil, false, nil
	}
	section, ok := value.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("%s: expected mapping", key)
	}
	return section, true, nil
}

func optionalString(values map[string]any, key, fallback, path string) (string, error) {
	value, exists := values[key]
	if !exists {
		return fallback, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected string", path)
	}
	return text, nil
}

func optionalBool(values map[string]any, key string, fallback bool, path string) (bool, error) {
	value, exists := values[key]
	if !exists {
		return fallback, nil
	}
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s: expected boolean", path)
	}
	return boolean, nil
}

func optionalInt(values map[string]any, key string, fallback int, path string) (int, error) {
	value, exists := values[key]
	if !exists {
		return fallback, nil
	}
	integer, err := requireInt(value, path)
	if err != nil {
		return 0, err
	}
	return integer, nil
}

func requireInt(value any, path string) (int, error) {
	integer, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("%s: expected integer", path)
	}
	return integer, nil
}

func optionalDuration(values map[string]any, key string, fallback time.Duration, path string) (time.Duration, error) {
	value, exists := values[key]
	if !exists {
		return fallback, nil
	}
	text, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("%s: expected duration string", path)
	}
	duration, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", path, text, err)
	}
	return duration, nil
}

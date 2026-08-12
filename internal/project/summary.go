package project

import "fmt"

func (c Config) Summary() string {
	return fmt.Sprintf(
		`project: %s
module: %s
preset: %s
go: language=%s toolchain=%s
architecture: %s/%s
api: enabled=%t router=%s port=%d
database: enabled=%t engine=%s driver=%s pool=%s
auth: enabled=%t strategy=%s rbac=%t
rate-limit: enabled=%t strategy=%s rps=%d burst=%d
cache: enabled=%t provider=%s address=%s db=%d
messaging: enabled=%t provider=%s brokers=%s group=%s retries=%d
outbox: enabled=%t interval=%s batch=%d attempts=%d
observability: metrics=%t tracing=%t
pprof: enabled=%t address=%s
deployment: docker=%t compose=%t kubernetes=%t namespace=%s replicas=%d runtime=%s non-root=%t
coverage minimum: %d%%`,
		c.Project.Name, c.Project.Module, c.Project.Preset,
		c.GoLanguageVersion(), c.GoToolchainVersion(),
		c.Architecture.Type, c.Architecture.Layout,
		c.API.Enabled, c.API.Router, c.API.Port,
		c.Database.Enabled, c.Database.Engine, c.Database.Driver, c.Database.Pool,
		c.Auth.Enabled, c.Auth.Strategy, c.Auth.RBAC,
		c.RateLimit.Enabled, c.RateLimit.Strategy, c.RateLimit.RequestsPerSecond, c.RateLimit.Burst,
		c.Cache.Enabled, c.Cache.Provider, c.Cache.Address, c.Cache.DB,
		c.Messaging.Enabled, c.Messaging.Provider, c.Messaging.Brokers, c.Messaging.ConsumerGroup, c.Messaging.MaxRetries,
		c.Outbox.Enabled, c.Outbox.PollInterval, c.Outbox.BatchSize, c.Outbox.MaxAttempts,
		c.Observability.Metrics.Enabled, c.Observability.Tracing.Enabled,
		c.Performance.Pprof.Enabled, c.Performance.Pprof.Address,
		c.Deployment.Docker, c.Deployment.Compose, c.Deployment.Kubernetes, c.Deployment.Namespace, c.Deployment.Replicas, c.Deployment.RuntimeImage, c.Deployment.NonRoot,
		c.Quality.Coverage.Minimum,
	)
}

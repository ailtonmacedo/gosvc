# Sprint 6 — Security and Observability

## Goal

Add the `production-api` preset on top of the PostgreSQL/OpenAPI foundation and
generate the security and observability capabilities required by the framework
specification.

## Delivered scope

### Production preset

A third built-in preset is now available:

```bash
gosvc new order-service \
  --module github.com/acme/order-service \
  --preset production-api
```

The preset composes:

```text
base
config
chi
health
docker
postgres
migrations
sqlc
openapi
request-validation
jwt
auth-refresh
rbac
rate-limit-local
logging
prometheus
opentelemetry
pprof
github-actions
```

The project configuration gained typed sections for authentication, rate
limiting, logging, metrics, tracing, and pprof. Database-backed presets now
default to Go 1.25; the minimal preset continues to resolve the local Go
version.

### JWT access tokens

The generated JWT manager:

- requires an HS256 secret containing at least 32 bytes;
- emits a unique token ID;
- includes subject, issuer, audience, issued-at, not-before, and expiration;
- explicitly restricts accepted signing methods;
- requires expiration;
- validates issuer and audience;
- allows a 30-second clock skew;
- maps all parse failures to the domain-level unauthenticated error.

The access-token lifetime defaults to 15 minutes.

### Opaque refresh tokens

Refresh tokens are independent random 32-byte values encoded with URL-safe
base64. Only SHA-256 hashes are persisted.

The generated authentication session workflow supports:

- seven-day refresh lifetime;
- session creation on login;
- row locking during rotation;
- one-time refresh-token rotation;
- revocation of the previous session;
- detection of already rotated or revoked tokens;
- revocation of all active sessions for the user when reuse is detected;
- explicit logout;
- cascade deletion when a user is removed.

The refresh token is never included in the JSON response. It is sent as an
HttpOnly, SameSite=Strict cookie and can require `Secure` in production.

### Password handling

Passwords use bcrypt. The generated project contains:

```bash
go run ./cmd/hash-password '<password>'
```

This helper is intended for bootstrapping a development or administrative user
without storing plaintext passwords in migrations or source code.

### Ports and Clean Architecture

Authentication infrastructure remains outside the domain and application
layers.

The application service depends on:

```go
type AuthRepository interface { /* ... */ }
type PasswordVerifier interface { /* ... */ }
type TokenManager interface { /* ... */ }
```

The JWT library, bcrypt, pgxpool, cookies, and HTTP details are implemented only
by infrastructure adapters.

### HTTP authentication and RBAC

Generated public routes:

```text
POST /auth/login
POST /auth/refresh
POST /auth/logout
```

Generated protected example:

```text
GET /admin/ping
```

The protected router group performs authentication before business CRUD routes.
The admin subgroup additionally requires the `admin` role.

Authentication middleware:

- requires the Bearer scheme;
- parses the access token once;
- stores a typed principal in request context;
- returns a standardized 401 response.

RBAC middleware distinguishes unauthenticated access from authenticated access
without sufficient permissions and returns 403 for the latter.

### OpenAPI contract

The generated contract now contains:

- login, refresh, logout, and admin operations;
- login and token response schemas;
- a Bearer JWT security scheme;
- global authentication requirements;
- explicit public overrides for authentication endpoints;
- existing CRUD, health, documentation, and error contracts.

This prevents the request-validation middleware from rejecting security routes
that exist at runtime.

### Local rate limiting

The generated limiter uses `golang.org/x/time/rate` and supports:

```text
key: ip
key: user
```

Features:

- configurable requests per second;
- configurable burst;
- HTTP 429 response;
- `Retry-After` and limit headers;
- stale key eviction;
- configurable entry TTL and cleanup interval;
- IP limiting before authentication;
- user limiting after authentication, ensuring the authenticated user ID is
  actually available.

The limiter is intentionally local to one process. A Redis or gateway strategy
remains future work for globally shared limits across replicas.

### Structured logging

The generated project configures `log/slog` with:

- text output for development;
- JSON output for production;
- service and environment attributes;
- request ID;
- trace ID when a valid span exists;
- method, path, status, response bytes, and duration.

Tokens, cookies, passwords, and request bodies are not logged.

### Prometheus metrics

The generated project exposes:

```text
GET /metrics
```

Metrics include:

```text
http_requests_total{method,status}
http_request_duration_seconds{method}
http_requests_in_flight
```

The label set intentionally avoids URL values and user identifiers to prevent
unbounded cardinality.

### OpenTelemetry tracing

The generated project supports OTLP gRPC tracing with:

- configurable enable/disable flag;
- configurable collector endpoint;
- configurable insecure transport for local development;
- resource attributes for service name and environment;
- W3C Trace Context and Baggage propagation;
- HTTP server instrumentation;
- provider shutdown during graceful shutdown.

Docker Compose includes an OpenTelemetry Collector using the debug exporter.
That configuration verifies export and propagation but does not provide durable
trace storage.

### pprof isolation

`pprof` is disabled by default. When enabled, it runs on a separate
administration server, defaulting to:

```text
127.0.0.1:6060
```

The pprof endpoints are not added to the public Chi router. The administration
server participates in graceful shutdown.

### Development observability stack

The generated production Compose file includes:

```text
postgres
api
prometheus
otel-collector
```

Prometheus scrapes the application metrics endpoint. The Collector receives
OTLP gRPC traces and writes them through a debug exporter.

### Dependency and tool updates

The generated production dependency set was updated together rather than
mixing incompatible generations:

```text
github.com/jackc/pgx/v5                         v5.10.0
github.com/golang-jwt/jwt/v5                   v5.3.1
github.com/prometheus/client_golang            v1.23.2
go.opentelemetry.io/otel                       v1.45.0
go.opentelemetry.io/otel/sdk                   v1.45.0
go.opentelemetry.io/otel/.../otlptracegrpc     v1.45.0
go.opentelemetry.io/contrib/.../otelhttp       v0.70.0
golang.org/x/crypto                            v0.54.0
golang.org/x/time                              v0.12.0
```

Generated tool versions now include oapi-codegen v2.8.0, GolangCI-Lint v2.12.2,
govulncheck v1.6.0, golang-migrate v4.19.1, and sqlc v1.31.0. GolangCI-Lint
configuration was migrated to schema version 2.

## Generated files

The production preset adds, among others:

```text
cmd/hash-password/main.go
internal/domain/auth.go
internal/ports/auth.go
internal/application/auth_service.go
internal/application/auth_service_test.go
internal/infrastructure/auth/jwt.go
internal/infrastructure/auth/jwt_test.go
internal/infrastructure/auth/password.go
internal/infrastructure/http/auth_handler.go
internal/infrastructure/http/auth_handler_test.go
internal/infrastructure/http/auth_middleware.go
internal/infrastructure/http/rate_limit.go
internal/infrastructure/http/logging.go
internal/infrastructure/http/metrics.go
internal/infrastructure/http/security_observability_test.go
internal/infrastructure/persistence/postgres/auth_repository.go
internal/observability/observability.go
internal/bootstrap/auth.go
db/migrations/900001_create_auth.up.sql
db/migrations/900001_create_auth.down.sql
deployments/observability/prometheus.yml
deployments/observability/otel-collector.yaml
```

Authentication migrations use a reserved high sequence (`900001`) so they do
not collide with sequential business-resource migrations.

## Tests added

Generated tests cover:

- invalid and valid login workflows;
- inactive users and password failure;
- repository and token-manager errors;
- session creation;
- refresh rotation;
- invalid refresh tokens and logout behavior;
- weak JWT secrets;
- access-token issuance;
- refresh-token generation and deterministic hashing;
- invalid access and empty refresh tokens;
- bcrypt hashing and comparison;
- login/refresh/logout HTTP behavior;
- refresh cookie attributes;
- Bearer authentication middleware;
- RBAC success and rejection;
- IP and user rate limiting;
- Prometheus endpoint and middleware;
- request logging;
- documentation, health, and protected routes.

The production preset, including an added UUID resource, passes the generated
80% coverage gate in the generator test suite.

## Corrections made during the sprint

### OpenAPI and runtime mismatch

The first production implementation added authentication routes to the router
but not to OpenAPI. Because validation executes before routing, those requests
would have been rejected. The authentication and admin paths are now generated
as part of the contract.

### User rate-limit placement

The initial user strategy ran before authentication and therefore fell back to
IP. The router composition now places the user limiter after authentication.

### Go toolchain compatibility in offline tests

Current database and telemetry dependencies require a newer Go line than the
isolated test runner. Production output correctly targets Go 1.25, while the
generator's offline compilation harness temporarily substitutes a local Go
1.23 module file and API-compatible local stubs. The original generated
`go.mod` is restored after each test.

## Validation performed

The Sprint 6 repository passes:

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
```

The generator suite also:

- creates all three presets;
- creates a production API with security and observability;
- adds an additional UUID resource;
- verifies stable business and authentication migrations;
- parses generated YAML files;
- compiles generated projects against API-compatible local stubs;
- runs generated tests and the 80% coverage gate;
- verifies idempotent regeneration;
- validates the generated project architecture and manifest.

## Environment limitations

This execution environment cannot reach the public Go module proxy and does not
provide Docker or PostgreSQL. As a result, the following were not executed
against their real binaries and services here:

```text
go mod download for the generated application
sqlc
oapi-codegen
golang-migrate
golangci-lint
govulncheck
Docker build
Docker Compose
PostgreSQL integration tests
live OTLP export
live Prometheus scraping
```

The generated CI workflow is the required connected-environment validation for
those steps. `gosvc doctor` reports missing tools explicitly rather than
silently skipping them.

## Remaining production work

The following capabilities remain intentionally outside this sprint:

- asymmetric JWT signing and JWKS distribution;
- MFA;
- Redis-backed distributed rate limiting;
- access-token denylist;
- user-management endpoints;
- password reset and email verification;
- audit-event storage;
- durable trace backend and dashboards;
- alert rules and Alertmanager;
- database instrumentation spans;
- Kafka, Outbox Pattern, and Kubernetes.

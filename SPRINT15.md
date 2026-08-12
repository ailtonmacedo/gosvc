# Sprint 15 — Real Integration & E2E Certification

## Objective

Turn the remaining live-integration uncertainty into an explicit, machine-readable release gate. The generator must distinguish product failures from infrastructure that is unavailable on the current host and must be able to exercise generated projects with real dependencies on a connected CI runner.

## Delivered

### `gosvc certify`

A new certification command supports two modes:

```bash
gosvc certify --mode static
gosvc certify --mode real --require-real
```

Static mode generates every built-in preset, adds a representative UUID CRUD resource to database-backed presets, validates structure and Clean Architecture rules, and proves generator idempotency.

Real mode additionally executes the generated projects with their real toolchain and infrastructure. It reports each preset as `PASS`, `FAIL`, `BLOCKED`, or `SKIPPED` and can emit a versioned JSON report.

### Real certification coverage

The real path is designed to verify:

- `go mod download`, `go test`, `go vet`, `go build`;
- real `oapi-codegen` and `sqlc generate`;
- `golangci-lint` and `govulncheck`;
- Docker image builds;
- PostgreSQL startup and migration `up/down/up`;
- generated integration tests;
- live health checks and CRUD;
- JWT login, protected CRUD, RBAC, refresh, logout, and Prometheus metrics;
- Redis idempotency;
- Kafka/Redpanda publish and consume;
- Transactional Outbox publication through the generated worker.

### CI certification

`.github/workflows/certification.yml` runs on a connected GitHub Actions runner with Go 1.25 and pinned versions of sqlc, migrate, oapi-codegen, golangci-lint, and govulncheck. The real certification report is uploaded as CI evidence.

The release workflow now also uses Go 1.25 and requires real certification before deterministic release assets are built.

### Report contract

Added:

- `schema/certification-report.schema.json`;
- `docs/CERTIFICATION.md`;
- compatibility contract entry for certification report schema v1.

The schema validates both the static and real reports produced during this sprint.

## Distributed correctness fixes found by certification work

### Kafka dual listeners

The generated Docker Compose configuration previously advertised `kafka:9092` to all clients. Host-side clients connecting through `127.0.0.1:9092` could receive broker metadata pointing at a Docker-only hostname.

The generated broker now uses:

```text
internal listener: kafka:29092
external listener: 127.0.0.1:9092
```

API and worker containers use `kafka:29092`; host-side development and certification keep using `127.0.0.1:9092`.

### Idempotency release on DLQ failure

A generated Kafka consumer could previously acquire the Redis idempotency key, fail business processing, fail to publish to the DLQ, and retain the idempotency reservation. A redelivered message could then be treated as already processed.

The `IdempotencyStore` contract now includes `Release`, and the consumer releases its reservation when processing cannot be safely completed or handed to the DLQ.

Generated tests cover:

- normal success;
- duplicate delivery;
- handler failure followed by successful DLQ publication;
- DLQ publication failure followed by idempotency release.

A generated real integration test also covers Redis acquire/release/reacquire and Kafka publish/consume when run with the `integration` build tag.

## Certification result on this host

### Static mode

All four presets passed:

```text
PASS  event-driven-api
PASS  minimal-api
PASS  postgres-api
PASS  production-api

passed=4 failed=0 blocked=0
```

### Real mode

All four presets are correctly reported as `BLOCKED`, not `PASS` and not `FAIL`.

The current host provides Go 1.23.2 and Git, but does not provide the real certification infrastructure:

- Go 1.25 for database-backed presets;
- Docker / Docker Compose;
- sqlc;
- oapi-codegen;
- golang-migrate;
- golangci-lint;
- govulncheck;
- resolvable access to `proxy.golang.org`.

The reports are stored in:

```text
artifacts/certification-static.json
artifacts/certification-real.json
```

`--require-real` converts the `BLOCKED` state into a failing CLI exit status, which is what the GitHub workflow uses.

## Quality gates executed

```bash
make verify
go test -race ./...
go vet ./...
go build ./...
gosvc acceptance
gosvc certify --mode static
gosvc release check --version 1.1.0 --allow-placeholder
```

Results:

- generator test suite: PASS;
- race detector: PASS;
- vet: PASS;
- build: PASS;
- built-in preset acceptance: 4/4 PASS;
- static certification: 4/4 PASS;
- release preflight: PASS with the documented placeholder override.

## Remaining external proof

No local workaround is treated as equivalent to a real E2E run. The remaining proof is to execute `.github/workflows/certification.yml` on a connected GitHub runner with Docker and module access. The workflow is configured to fail if any required real integration check is blocked or fails.

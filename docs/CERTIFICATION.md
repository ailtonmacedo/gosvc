# Real Integration & E2E Certification

`gosvc certify` is the release-level verification command for generated projects.
It is intentionally different from `gosvc acceptance`:

- `acceptance` proves that the generator can render, validate, and regenerate all built-in presets deterministically.
- `certify --mode real` executes the generated projects with the real Go toolchain, code generators, Docker, PostgreSQL, Redis, and Kafka.

## Static certification

Static mode requires only the `gosvc` build environment:

```bash
gosvc certify --mode static
```

It generates every built-in preset, adds a representative UUID resource to database-backed presets, validates the manifest and architecture, and verifies idempotency.

JSON output:

```bash
gosvc certify --mode static --json > certification-static.json
```

## Real certification

Real mode additionally requires:

- Go compatible with the generated preset (Go 1.25+ for database-backed presets);
- Git;
- Docker with Docker Compose;
- sqlc;
- oapi-codegen;
- golang-migrate;
- golangci-lint;
- govulncheck;
- network access to the Go module proxy or another configured module source.

Run it with:

```bash
gosvc certify --mode real --require-real
```

`--require-real` makes missing infrastructure a failing exit status. Without it,
missing infrastructure is reported as `BLOCKED` instead of being confused with a product failure.

## Real checks by preset

### minimal-api

- `go mod download`;
- `go mod tidy` after code generation, so `go.sum` contains the checksums required by compilation;
- unit tests;
- `go vet`;
- build;
- golangci-lint;
- govulncheck;
- Docker image build;
- start the generated API;
- `/health/live`;
- `/health/ready`.

### postgres-api

Everything in `minimal-api`, plus:

- real oapi-codegen generation;
- real sqlc generation;
- PostgreSQL through Docker Compose;
- Docker Compose `up --wait` readiness, requiring healthchecked services to report `healthy`;
- strict verification of Compose `State`/`Health` before database access;
- migration `up`, with bounded retry only for transient PostgreSQL startup connection errors;
- integration tests;
- live CRUD over HTTP;
- migration `down 1` followed by `up` again.

### production-api

Everything in `postgres-api`, plus:

- create a real bcrypt administrator;
- login;
- signed JWT access token;
- protected CRUD;
- RBAC admin endpoint;
- refresh-token rotation path;
- logout;
- Prometheus metrics endpoint.

Tracing is disabled during the API smoke test so an unavailable external collector
cannot make application correctness depend on telemetry infrastructure.

### event-driven-api

Everything in `production-api`, plus:

- Redis container health;
- Kafka/Redpanda container health;
- real Outbox row insertion;
- real worker process;
- Outbox row marked as published;
- Kafka message observed with `rpk`.

## Report statuses

- `PASS`: the check executed and succeeded.
- `FAIL`: the check executed and failed.
- `BLOCKED`: required infrastructure is unavailable, so the check could not execute.
- `SKIPPED`: explicitly omitted by policy or platform.

The JSON contract is documented in `schema/certification-report.schema.json`.

If migration startup still fails, the report automatically captures `docker compose ps` and the last PostgreSQL container logs so the CI artifact contains the operational cause.

## CI

The repository workflow `.github/workflows/certification.yml` installs the exact
external tools used by generated projects and runs:

```bash
gosvc certify --mode real --require-real --json
```

This workflow is the authoritative environment for release certification when a
local development machine does not provide Docker or the external services.

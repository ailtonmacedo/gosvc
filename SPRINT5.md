# Sprint 5 — Production Quality and Diagnostics

## Scope

Sprint 5 implements the backlog items for generated tests, coverage, quality
tooling, CI, project validation, environment diagnosis, and architectural
boundary checks.

## Delivered

### Generated tests

- comprehensive `OrderService` tests covering all CRUD methods, domain
  validation, and repository errors;
- comprehensive Order HTTP handler table covering success, invalid input,
  not-found, validation, and internal errors;
- comprehensive application and HTTP tests for every resource added through
  `gosvc add resource`;
- one repository integration test per resource behind `//go:build integration`;
- cleanup and cancellation through `t.Cleanup`.

### Coverage

Generated projects include:

```bash
make coverage
make coverage-check
```

The default threshold is 80%. The coverage script measures domain,
application, and HTTP delivery code while excluding generated adapters. The
generator's own tests execute this gate against:

1. `minimal-api`;
2. `postgres-api` with the default Order resource;
3. `postgres-api` after adding a UUID Product resource.

The first PostgreSQL implementation measured 50.6%. Tests were expanded until
both PostgreSQL scenarios passed the 80% gate.

### Quality tooling

Generated projects now include:

- `.golangci.yml`;
- locally pinned `golangci-lint` installation;
- locally pinned `govulncheck` installation;
- format, tidy, generation, vet, lint, test, coverage, race, vulnerability,
  build, and Docker targets;
- executable `scripts/check-coverage.sh`.

### CI

Pull Request job:

- format check;
- module consistency;
- generated-code consistency;
- vet;
- lint;
- unit coverage gate;
- build.

Main branch job:

- race detector;
- vulnerability scan;
- PostgreSQL service;
- migrations;
- integration tests;
- Docker build.

### `gosvc validate`

Checks:

- project configuration;
- `go.mod` module path;
- manifest schema, preset, features, and tracked files;
- checksums and ownership;
- resource registry;
- migration and query artifacts;
- OpenAPI resource paths;
- architecture violations.

User-owned file changes are warnings. Generated-file changes are errors.

### `gosvc doctor`

Checks project-local `bin` before `PATH` for:

- Go;
- Git;
- Docker;
- Docker Compose;
- sqlc;
- oapi-codegen;
- golang-migrate;
- golangci-lint;
- govulncheck.

Missing required tools return exit code 4.

### `gosvc check architecture`

Enforces:

- domain cannot import application or infrastructure;
- domain cannot import Chi, pgx, JWT, OpenTelemetry, or Prometheus;
- application cannot import infrastructure, Chi, or pgx.

Violations identify the source file, forbidden import, and rule.

### `gosvc verify`

```bash
gosvc verify --project . --static
```

Static mode validates structure and architecture. Full mode additionally runs:

```text
go test ./...
go test -race ./...
go vet ./...
go build ./...
golangci-lint run
govulncheck ./...
```

## Validation executed

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
```

Manual CLI validation:

```bash
gosvc new catalog-service --preset postgres-api ...
gosvc add resource product ...
gosvc validate --project ./catalog-service
gosvc check architecture --project ./catalog-service
gosvc verify --project ./catalog-service --static
gosvc doctor --project ./catalog-service
```

The first four commands passed. `doctor` correctly returned exit code 4 because
Docker and external project tools are unavailable in the delivery environment.

## Environment limitation

The environment cannot execute live Docker/PostgreSQL or download the external
quality and generation tools. Generated projects are compiled and coverage
checked with local API-compatible stubs. Connected CI must still execute:

- `make tools`;
- `make generate`;
- migrations;
- integration tests against PostgreSQL;
- Docker build;
- golangci-lint;
- govulncheck.

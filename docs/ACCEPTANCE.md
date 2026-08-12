# Acceptance Matrix

The built-in acceptance matrix is the final offline quality gate for the project generator. It exercises every preset through the public generator APIs without downloading external Go modules or starting infrastructure services.

## Run the matrix

```bash
gosvc acceptance
```

Example output:

```text
PASS    bare               files=... resources=0 architecture=... duration=...ms
PASS    event-driven-api   files=... resources=2 architecture=... duration=...ms
PASS    minimal-api        files=... resources=0 architecture=... duration=...ms
PASS    postgres-api       files=... resources=2 architecture=... duration=...ms
PASS    production-api     files=... resources=2 architecture=... duration=...ms
PASS    worker             files=... resources=0 architecture=... duration=...ms
Acceptance matrix: passed=6 failed=0 duration=...ms
All built-in presets passed acceptance: 6/6.
```

Machine-readable output (schema: `schema/acceptance-report.schema.json`):

```bash
gosvc acceptance --json > acceptance-report.json
```

Keep the generated projects for inspection:

```bash
gosvc acceptance --keep
```

Use an explicit empty workspace:

```bash
gosvc acceptance --workdir ./tmp/acceptance
```

## Checks performed

For every built-in preset, the matrix:

1. resolves the preset defaults;
2. generates a new project;
3. reloads `project.yaml` and verifies the canonical preset ID and preset version;
4. verifies the generated Go runtime policy (`runtime.go.language/toolchain`);
5. validates the manifest and tracked-file checksums;
6. validates the Clean Architecture import boundaries;
7. repeats generation and requires zero writes;
8. counts generated files and records execution time.

For `postgres-api`, `production-api`, and `event-driven-api`, it also:

1. adds a representative `product` resource with UUID, string, decimal, boolean, and datetime fields;
2. validates migrations, SQL queries, OpenAPI paths, ports, use cases, handlers, and repositories;
3. repeats resource generation and requires zero writes.

The acceptance matrix intentionally does not download modules, run Docker, or connect to PostgreSQL, Redis, Kafka, or Kubernetes. Those checks remain in connected CI and deployment environments.

## Release integration

`gosvc release check` runs the acceptance matrix automatically. A release cannot pass preflight if any built-in preset fails generation, structural validation, or idempotency.

## CI

The repository includes `.github/workflows/acceptance.yml`, which runs the matrix, uploads the JSON report, and executes the generator benchmarks on supported Go versions.


See `docs/TEMPLATE_REGRESSION.md` for the complete static/real regression model.

The controlled Chi/Echo + sqlc/GORM combinations are additionally compiled by the component regression suite documented in `docs/TEMPLATE_REGRESSION.md`.

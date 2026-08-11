# Sprint 12 — Acceptance and Compatibility Hardening

## Objective

Close the v1.0 quality loop with an offline, deterministic acceptance matrix for every built-in preset, measurable generator benchmarks, compatibility documentation, and release-preflight enforcement.

## Delivered

### `gosvc acceptance`

The new command generates and validates:

- `minimal-api`;
- `postgres-api`;
- `production-api`;
- `event-driven-api`.

For database-backed presets, it also adds a representative `product` CRUD resource containing:

- UUID identifier;
- string;
- decimal/integer representation;
- boolean;
- datetime.

Each scenario validates:

- initial generation;
- manifest and checksum consistency;
- required resource artifacts;
- OpenAPI paths;
- Clean Architecture imports;
- project regeneration with zero writes;
- resource regeneration with zero writes.

### JSON report

```bash
gosvc acceptance --json > acceptance-report.json
```

The report uses schema version 1 and records file counts, resource counts, architecture files, write counts, checks performed, status, and duration per preset.

### Release integration

`gosvc release check` now runs the acceptance matrix. A release preflight fails when any built-in preset fails generation, validation, or idempotency.

### Benchmarks

Added benchmarks for:

- minimal preset rendering;
- event-driven preset rendering;
- complete minimal project generation.

Run them with:

```bash
make benchmark
```

### Compatibility policy

Added:

- `docs/ACCEPTANCE.md`;
- `docs/COMPATIBILITY.md`;
- `schema/compatibility-matrix.json`;
- `schema/acceptance-report.schema.json`;
- `.github/workflows/acceptance.yml`.

The compatibility policy versions project config, manifest, plugin manifest, plugin protocol, resource registry, and acceptance report contracts.

## Acceptance results

```text
PASS event-driven-api
PASS minimal-api
PASS postgres-api
PASS production-api
```

Observed generated file counts in this delivery:

| Preset | Files | Resources | Architecture files |
|---|---:|---:|---:|
| `minimal-api` | 21 | 0 | 2 |
| `postgres-api` | 59 | 2 | 10 |
| `production-api` | 81 | 2 | 13 |
| `event-driven-api` | 103 | 2 | 16 |

## Quality gates executed

```bash
gofmt
make verify
go test -race ./internal/acceptance ./internal/cli ./internal/releasecheck ./internal/completion
go vet ./...
go build ./...
gosvc acceptance
gosvc release check --version 1.0.0 --allow-placeholder
```

The complete non-race test suite passed. Targeted race tests cover the new acceptance command, CLI integration, release preflight, and completion generation. The repository's complete race suite remains suitable for connected CI because its generated-project compilation fixtures are substantially heavier.

## Benchmark baseline

The exact values depend on CPU and filesystem. A one-iteration local baseline on Linux amd64 was recorded in `benchmarks.txt`.

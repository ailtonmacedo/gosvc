# Real Integration & E2E Certification

`gosvc certify` is the release-level verification command for generated projects.

- `acceptance` proves deterministic rendering/validation/regeneration.
- `certify --mode static` checks every preset without external infrastructure.
- `certify --mode real` executes the effective capabilities with real tools and Docker infrastructure.

## Capability-driven certification

Real certification does **not** infer every check from a preset name. It derives capabilities from the generated configuration:

```text
openapi.enabled                  -> oapi-codegen-real
database.code_generation=sqlc   -> sqlc-real
database.enabled                 -> PostgreSQL + migrate + integration tests
api.enabled                      -> HTTP smoke
cache.enabled                    -> Redis health
messaging.enabled                -> Kafka health/topics
outbox.enabled                   -> Outbox/worker/Kafka smoke
project.preset=worker            -> standalone worker lifecycle smoke
```

A non-applicable check is recorded as `SKIPPED` with a `not applicable: ...` detail. It is not a PASS and not a failure.

This fixes two important cases:

- `bare`/`worker`: no OpenAPI/sqlc files are expected;
- `postgres-api + GORM`: OpenAPI still runs, but `sqlc-real` is N/A.

## Static certification

```bash
gosvc certify --mode static
gosvc certify --mode static --json > certification-static.json
```

Expected human summary:

```text
Presets:
  PASS     bare
  PASS     event-driven-api
  PASS     minimal-api
  PASS     postgres-api
  PASS     production-api
  PASS     worker
Certification: passed=6 failed=0 blocked=0 skipped=0
```

## Real certification prerequisites

- secure effective Go toolchain (`go1.25.12` floor for generated database APIs);
- Git;
- Docker; Docker Compose when the generated project enables Compose;
- oapi-codegen only when OpenAPI is enabled;
- sqlc only when `database.code_generation: sqlc`;
- golang-migrate only when database is enabled;
- golangci-lint and govulncheck;
- module-source network access when dependencies need download.

The Go security floor is compared using the complete `major.minor.patch` tuple. For example, `go1.25.4` does **not** satisfy a `go1.25.12` requirement, while `go1.25.12`, `go1.25.13`, or a newer Go minor release do.

```bash
gosvc certify --mode real --require-real --timeout 4m
```

`--require-real` turns environmental `BLOCKED` results into a failing exit status.

## Real check matrix

| Check | bare | worker | minimal-api | postgres-api sqlc | postgres-api GORM | production | event-driven |
|---|---:|---:|---:|---:|---:|---:|---:|
| Go quality gates | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Docker build | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| OpenAPI generation | N/A | N/A | N/A | ✅ | ✅ | ✅ | ✅ |
| sqlc generation | N/A | N/A | N/A | ✅ | N/A | ✅ | ✅ |
| app/worker/API smoke | app | worker | API | API | API | API | API |
| PostgreSQL/migrations | N/A | N/A | N/A | ✅ | ✅ | ✅ | ✅ |
| Redis | N/A | N/A | N/A | N/A | N/A | N/A | ✅ |
| Kafka/Outbox | N/A | N/A | N/A | N/A | N/A | N/A | ✅ |

## Status semantics

- `PASS`: check executed successfully.
- `FAIL`: check executed and failed.
- `BLOCKED`: required environment/tooling prevented execution.
- `SKIPPED`: policy says the check is not applicable to this effective project configuration.

The JSON contract is `schema/certification-report.schema.json`; schema v1 now enumerates all six built-in presets.

## CI

`.github/workflows/certification.yml` installs the exact external tooling and runs the real certification. The JSON artifact is uploaded even on failure, preserving the failing check and diagnostics.

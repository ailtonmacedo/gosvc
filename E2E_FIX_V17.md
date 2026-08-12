# E2E v17 — Capability Certification & Contract Hardening

## Real CI regression fixed

The v16 real certification loop unconditionally invoked OpenAPI/sqlc for every preset except `minimal-api`. The new non-HTTP `bare` and `worker` presets therefore failed with:

```text
error reading config file 'api/oapi-codegen.yaml': no such file or directory
```

The GitHub Actions evidence showed static certification passing 6/6 while real certification finished 4 passed / 2 failed. The failure was in the certification policy, not in the generated non-HTTP projects.

## Capability-driven model

v17 derives real checks from effective `project.Config`:

- `openapi.enabled` -> oapi-codegen;
- `database.code_generation == sqlc` -> sqlc;
- `database.enabled` -> migrations/PostgreSQL/integration;
- `api.enabled` -> HTTP smoke;
- `cache.enabled` -> Redis;
- `messaging.enabled` -> Kafka;
- `outbox.enabled` -> Outbox smoke;
- `worker` -> standalone worker lifecycle smoke.

Non-applicable checks are emitted as `SKIPPED` with an explicit `not applicable` detail.

## Go security floor comparison

Real certification now compares the complete Go version tuple (major, minor, and patch). A host running `go1.25.4` no longer satisfies a required security floor of `go1.25.12`; `go1.25.12` and newer patch/minor releases do. This closes a certification gap where only major/minor values were previously compared.

## Preset upgrade

`gosvc upgrade` now accepts:

```bash
gosvc upgrade --project . --preset-version 1.1.0 --dry-run
gosvc upgrade --project . --preset-version 1.1.0
```

Legacy `minimal-api@1.0.0` and `postgres-api@1.0.0` remain resolvable, enabling deterministic upgrade tests to current `1.1.0`.

## Documentation hardening

Added:

- `docs/CONTRACTS.md`;
- `docs/MIGRATING_FROM_V15.md`;
- plugin protocol request/response examples;
- exact driver/pool matrix;
- exact coverage N/A rule;
- `gosvc verify` vs `make verify` / `make verify-strict` guidance;
- capability-driven certification matrix.

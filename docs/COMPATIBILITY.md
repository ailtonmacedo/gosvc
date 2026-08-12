# Compatibility Policy

## Versioned contracts

| Contract | Current | Role |
|---|---:|---|
| `project.yaml` | 1 | user-owned desired project configuration |
| `.gosvc/manifest.json` | 4 | generator-owned state, ownership/checksums, compatibility and resolved preset version |
| Plugin manifest | 3 | native/Docker execution declaration |
| Plugin protocol | 1 | JSON stdin/stdout |
| Resource registry | 1 | CRUD/resource definitions |
| Acceptance report | 1 | deterministic generator evidence |
| Certification report | 1 | real/static certification evidence; six built-in presets |

The configuration schema and manifest schema are intentionally independent. `schema_version: 1` at the top of `project.yaml` does **not** mean the internal manifest is schema 1. See `docs/CONTRACTS.md`.

## Preset compatibility

Preset versions are separate from the gosvc CLI version.

Current defaults:

- `bare@1.0.0`;
- `worker@1.0.0`;
- `minimal-api@1.1.0` (legacy `1.0.0` retained);
- `postgres-api@1.1.0` (legacy `1.0.0` retained);
- `production-api@1.0.0`;
- `event-driven-api@1.0.0`.

Normal regeneration does not silently switch versions. Use:

```bash
gosvc upgrade --project . --preset-version 1.1.0 --dry-run
```

## Component compatibility

| Code generation | Driver | Pool |
|---|---|---|
| `sqlc` | `pgx` | `pgxpool` |
| `gorm` | `gorm-postgres` | `database/sql` |

`production-api` and `event-driven-api` remain Chi + sqlc only.

## Go compatibility

| Layer | Policy | Meaning |
|---|---|---|
| gosvc CLI | repository `go 1.23` | generator build compatibility |
| generated DB/API language | `runtime.go.language: 1.25.0` | source-language/module contract |
| secure generated toolchain | `runtime.go.toolchain: go1.25.12` | compiler + standard-library security floor |

## Generated-project invariants

- regeneration is idempotent;
- user-owned files are protected except explicit configuration migrations requested by `upgrade`;
- generated-file changes obey manifest checksum ownership;
- migration numbers are not reassigned;
- plugin artifacts preserve producer/ownership metadata;
- unsupported component combinations fail before generation;
- real certification executes only applicable capabilities.

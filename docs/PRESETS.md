# Presets and controlled composition

Presets have an **ID and version independent from the CLI**. `gosvc new --preset postgres-api` resolves the **current/default version** of that preset; pin a historical version only when reproducibility requires it.

```bash
gosvc presets list
gosvc presets show postgres-api
gosvc presets show postgres-api 1.0.0
gosvc presets show postgres-api 1.1.0
```

## Current presets

| Preset | Current | Type | Router | Persistence | Use |
|---|---:|---|---|---|---|
| `bare` | `1.0.0` | app | — | — | minimal Clean Architecture without HTTP/database |
| `worker` | `1.0.0` | worker | — | — | long-running background process without HTTP/database |
| `minimal-api` | `1.1.0` | API | Chi / Echo | — | small HTTP API |
| `postgres-api` | `1.1.0` | API | Chi / Echo | sqlc / GORM | PostgreSQL + OpenAPI/CRUD |
| `production-api` | `1.0.0` | API | Chi | sqlc | auth/observability golden path |
| `event-driven-api` | `1.0.0` | API | Chi | sqlc | Redis/Kafka/Outbox golden path |

Legacy versions retained for explicit upgrade/testing:

- `minimal-api@1.0.0`: Chi only;
- `postgres-api@1.0.0`: Chi + sqlc only.

## Component contracts

| Persistence | `database.code_generation` | `database.driver` | `database.pool` | Generated query layer |
|---|---|---|---|---|
| sqlc | `sqlc` | `pgx` | `pgxpool` | `sqlc.yaml`, `db/queries`, `internal/generated/sqlc` |
| GORM | `gorm` | `gorm-postgres` | `database/sql` | GORM repository; no sqlc files |

The combination is validated as one contract. Mixing `gorm` with `pgxpool`, or `sqlc` with `database/sql`, fails before generation.

## Golden path and alternatives

```text
minimal-api@1.1.0
  ├─ chi   (default)
  └─ echo

postgres-api@1.1.0
  ├─ router: chi (default) | echo
  └─ persistence: sqlc (default) | gorm

production-api@1.0.0 / event-driven-api@1.0.0
  └─ chi + sqlc only
```

## No implicit version upgrade

This command:

```bash
gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api
```

uses the current `postgres-api` version for a **new** project. Once the project records `preset_version`, regeneration keeps that exact version.

Move versions explicitly:

```bash
gosvc upgrade --project . --preset-version 1.1.0 --dry-run
gosvc upgrade --project . --preset-version 1.1.0
```

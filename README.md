# gosvc

`gosvc` is an opinionated generator for production-oriented Go projects. Generated applications are ordinary Go modules and do **not** depend on the generator at runtime.

## E2E v17 baseline

v17 keeps controlled composition and makes real certification capability-driven, so non-HTTP presets and GORM variants run only the checks that actually apply.

### Presets

| Preset | Version | Purpose |
|---|---:|---|
| `bare` | `1.0.0` | Clean Architecture scaffold without HTTP/database adapters |
| `worker` | `1.0.0` | long-running background worker without HTTP/database adapters |
| `minimal-api` | `1.1.0` | HTTP API; Chi by default, Echo optional |
| `postgres-api` | `1.1.0` | PostgreSQL/OpenAPI/CRUD; Chi/Echo + sqlc/GORM |
| `production-api` | `1.0.0` | secure observable API on the certified Chi + sqlc path |
| `event-driven-api` | `1.0.0` | production API + Redis/Kafka/Outbox/worker/Kubernetes |

Preset versions are independent from the CLI version and are persisted in `project.preset_version` and the gosvc manifest.

```bash
gosvc presets list
gosvc presets show postgres-api
```

## Controlled component composition

```text
minimal-api
  router: chi | echo

postgres-api
  router:      chi | echo
  persistence: sqlc | gorm

production-api / event-driven-api
  router:      chi
  persistence: sqlc
```

The default/golden path remains Chi + pgx/pgxpool + sqlc. Unsupported combinations fail before files are generated.

## Quickstart

Installed usage:

```bash
gosvc doctor --preset postgres-api

gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api

# Follow the exact `Next: cd ...` path emitted by gosvc.
cd catalog-service
make bootstrap
gosvc doctor --project .

docker compose up -d postgres
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/catalog-service?sslmode=disable'
make migrate-up
make verify
make run
```

When developing the generator itself, build and use `./bin/gosvc`; after entering the sibling generated project, use `../gosvc/bin/gosvc`. See `docs/QUICKSTART.md`.

## Other starting points

```bash
gosvc new batch-tool \
  --module github.com/acme/batch-tool \
  --preset bare

gosvc new cleanup-worker \
  --module github.com/acme/cleanup-worker \
  --preset worker

gosvc new edge-api \
  --module github.com/acme/edge-api \
  --preset minimal-api \
  --router echo

gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api \
  --router echo \
  --persistence gorm
```

## Go version policy

The generator and database-backed generated services intentionally have different contracts:

```text
gosvc CLI repository
  └─ go 1.23

Generated database-backed project
  ├─ runtime.go.language: 1.25.0
  ├─ runtime.go.toolchain: go1.25.12
  ├─ go.mod: go 1.25.0 + toolchain go1.25.12
  └─ Docker/CI/security floor: Go 1.25.12
```

## Quality model

Generated projects include gofmt, tidy/codegen drift checks, `go vet`, GolangCI-Lint, unit tests, coverage, race detector, govulncheck and build gates. v17 keeps the component regression matrix that generates and compiles every supported Chi/Echo + sqlc/GORM combination offline with deterministic stubs.

```bash
gosvc acceptance
gosvc certify --mode static
gosvc certify --mode real --require-real --timeout 4m
```

## Plugin execution

Plugins use the versioned JSON protocol. There are now two execution modes:

- **native** — backward-compatible trusted executable with checksum/snapshot protections;
- **docker** — schema-3 isolated execution with read-only project mount, dropped capabilities, no-new-privileges, resource limits, non-root user and network disabled by default.

Require isolation explicitly:

```bash
gosvc plugins run audit --project . --require-sandbox --dry-run
```

Network-enabled Docker plugins require a second explicit approval with `--allow-network`.

See `docs/PLUGIN_SECURITY.md` and `docs/PLUGINS.md` for copyable protocol request/response examples.

## Documentation

- `docs/gosvc-documentacao.html` — standalone visual guide;
- `docs/QUICKSTART.md` — first project;
- `docs/CONTRACTS.md` — project config vs manifest/plugin/report contracts;
- `docs/MIGRATING_FROM_V15.md` — migration and explicit preset-version upgrade;
- `docs/PRESETS.md` — versioned presets/component matrix;
- `docs/ARCHITECTURE.md` — generated architecture;
- `docs/COMMANDS.md` — command cookbook;
- `docs/PLUGIN_SECURITY.md` — native vs Docker threat model;
- `docs/TEMPLATE_REGRESSION.md` — regression/certification matrix;
- `docs/TROUBLESHOOTING.md` — operational fixes;
- `CHANGELOG.md` — release/history notes.

## Release

Repository release state marker: `release-evidence-ready`. The release preflight verifies this marker together with governance, acceptance, certification, and release-evidence contracts.

```bash
gosvc release check --version 1.1.0
gosvc release snapshot --version 1.1.0 --output dist --parallel 3
gosvc release verify --dist dist
```

Do not publish a release when required real certification fails or is blocked.

# E2E Fix v12 — First-run module reconciliation and unambiguous project navigation

## Problem observed

When `gosvc new catalog-service` is executed from the `gosvc` source checkout, v11
correctly creates the new service at `../catalog-service`. A shell chain that still uses
`cd ./catalog-service` can therefore enter an old nested project (or fail if it does not
exist).

The reported `make verify` failure at generated `Makefile:51` was the `tidy-check` Git
drift gate. `go mod tidy` legitimately normalized `go.mod` and populated/updated
`go.sum` with indirect dependencies and checksums. When the generated service already
had its own Git baseline, the v11 gate treated that first local normalization exactly
like CI drift and failed.

A separate usability risk is invoking `gosvc` from PATH after building a newer checkout:
the command may still resolve to an older installed binary.

## Changes

### 1. Exact next-directory instruction

After successful generation the CLI now prints the resolved destination and the exact
next command, for example:

```text
Project generated at ../catalog-service using preset postgres-api.
Next: cd ../catalog-service
Then: make bootstrap
```

If an older `gosvc/catalog-service` directory still exists while the safe destination is
`../catalog-service`, the CLI emits an explicit warning not to enter the legacy path.

### 2. First-run bootstrap

Generated projects now expose:

```bash
make bootstrap
```

It runs `go mod tidy` and, for database-backed presets, code generation. This is the
recommended initialization step before the first verification/commit.

### 3. Developer verification versus strict CI drift

Local:

```bash
make verify
```

continues after `go mod tidy` or code generation changes tracked files, but prints an
`UPDATE` message requiring review and commit. This lets a freshly generated or upgraded
project reconcile its module graph without a false-negative first run.

Strict mode:

```bash
make verify-strict
# or
make tidy-check STRICT_GIT_DRIFT=1
make generate-check STRICT_GIT_DRIFT=1
```

fails on any tracked drift. Generated GitHub Actions explicitly uses
`STRICT_GIT_DRIFT=1`, so CI remains fail-closed.

### 4. Source-checkout binary identity

The root project now provides:

```bash
make install VERSION=1.1.0
```

`gosvc version` also prints the resolved executable path. For source validation,
`./bin/gosvc` remains the safest command because it unambiguously uses the just-built
checkout.

## Correct workflow from inside gosvc

```bash
make build VERSION=1.1.0
./bin/gosvc version

./bin/gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api

cd ../catalog-service
docker compose up -d postgres
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/catalog-service?sslmode=disable'
make bootstrap
make migrate-up
make verify
make run
```

Do not use `cd ./catalog-service` from the `gosvc` checkout after safe sibling generation.

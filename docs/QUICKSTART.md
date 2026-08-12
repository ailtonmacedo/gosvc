# Quickstart

## Installed gosvc (recommended)

A preset version is optional when creating a **new** project. Omitting it selects the current version; the resolved value is persisted and future regeneration keeps it.

```bash
gosvc doctor --preset postgres-api

gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api

cd ../catalog-service
make bootstrap
gosvc doctor --project .
docker compose up -d postgres
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/catalog-service?sslmode=disable'
make migrate-up
make verify
make run
```

Pin when reproducibility requires it:

```bash
gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api \
  --preset-version 1.1.0
```

## Developing gosvc from source

```bash
cd framework-go/gosvc
make build VERSION=dev
./bin/gosvc doctor --preset postgres-api
./bin/gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api

cd ../catalog-service
make bootstrap
../gosvc/bin/gosvc doctor --project .
```

Always follow the exact `Next: cd ...` printed by the CLI.

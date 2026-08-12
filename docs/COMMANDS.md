# Command cookbook

## Discover presets

```bash
gosvc presets list
gosvc presets show postgres-api
gosvc presets show postgres-api 1.1.0
```

Omitting the version means **current version for new projects**. Existing projects keep the `preset_version` recorded in `project.yaml`.

## Preflight and generation

```bash
gosvc doctor --preset postgres-api

gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api
```

Variants:

```bash
gosvc new edge-api --module github.com/acme/edge-api --preset minimal-api --router echo

gosvc new catalog-service \
  --module github.com/acme/catalog-service \
  --preset postgres-api \
  --router echo \
  --persistence gorm
```

## Project validation: CLI vs Makefile

```bash
gosvc validate --project .
gosvc doctor --project .
gosvc check architecture --project .
gosvc verify --project . --static
gosvc verify --project .
```

`gosvc verify --static` validates gosvc structure/manifest/architecture only. `gosvc verify` adds Go tests, race, vet, build, golangci-lint and govulncheck.

Generated projects also expose:

```bash
make verify
make verify-strict
```

`make verify` is the project-native developer gate and includes format/tidy/codegen/coverage. It may reconcile local module/generated metadata and warn about drift. `make verify-strict` runs the same Makefile gate with `STRICT_GIT_DRIFT=1`, which is the CI policy for zero tracked drift.

Use the CLI verify for a tool-driven cross-project check; use the generated Makefile as the canonical local/CI project workflow.

## CRUD

```bash
gosvc add resource product \
  --fields "id:uuid,name:string,price:decimal,active:bool" \
  --crud \
  --project .
```

## Plugins

```bash
gosvc plugins list --project .
gosvc plugins validate --project .
gosvc plugins run audit --project . --dry-run
gosvc plugins run audit --project . --require-sandbox --dry-run
gosvc plugins run audit --project . --require-sandbox --allow-network
```

## Acceptance / certification

```bash
gosvc acceptance
gosvc acceptance --json
gosvc certify --mode static
gosvc certify --mode real --require-real --timeout 4m
```

## Framework and preset upgrades

```bash
gosvc upgrade --project . --dry-run

gosvc upgrade --project . --preset-version 1.1.0 --dry-run
gosvc upgrade --project . --preset-version 1.1.0

gosvc upgrade backups --project .
gosvc upgrade rollback --project . --backup latest --dry-run
```

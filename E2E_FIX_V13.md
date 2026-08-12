# E2E Fix v13 — golangci-lint cache isolation and PostgreSQL timestamp helper

## Symptom

A `postgres-api` service generated outside the gosvc source repository could fail `make verify` with two related diagnostics:

1. golangci-lint reported a stale source path that still pointed to the previous nested location such as `gosvc/catalog-service/...`;
2. the base PostgreSQL template reported `timeToDB` as unused before a resource with a `datetime` field had been added.

## Root causes

### Shared global golangci-lint cache

The default golangci-lint cache is user-global. Reusing the same Go module after moving the generated project could reuse cached analyzer results containing the old absolute path.

### Intentional helper seen as dead code

`timeToDB` is required by CRUD resources that declare `datetime` fields, but the initial `Order` resource does not call it. The `unused` linter therefore treated the extension helper as dead code in the base preset.

## Fix

Generated Makefiles now define:

```make
GOLANGCI_LINT_CACHE := $(CURDIR)/.cache/golangci-lint

lint: $(GOLANGCI_LINT)
	mkdir -p $(GOLANGCI_LINT_CACHE)
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(GOLANGCI_LINT) run
```

The generated `.gitignore` ignores `/.cache/`, and `make clean` removes the project-local cache. A `.project-root` marker records the absolute project root; if the whole service directory is moved later, `make lint` detects the path change and resets the stale cache automatically.

The PostgreSQL timestamp helper keeps `timeToDB`, but documents its deliberate deferred use:

```go
//nolint:unused // Used by generated resources that declare datetime fields.
func timeToDB(value time.Time) pgtype.Timestamptz {
    return pgtype.Timestamptz{Time: value, Valid: true}
}
```

This suppresses only the intentional helper; the `unused` linter remains enabled for the rest of the project.

## Regression coverage

- generated Makefile must define a project-local `GOLANGCI_LINT_CACHE`;
- lint invocation must pass that absolute project-local cache to golangci-lint;
- `.gitignore` must exclude `/.cache/`;
- `make clean` must remove `.cache`;
- generated PostgreSQL timestamp helper must retain `timeToDB` and its targeted `nolint:unused` reason;
- all framework tests, vet, build, race tests, acceptance matrix and static certification must remain green.

## Migration from v12

For an already generated v12 service, either regenerate/upgrade the generated Makefile and timestamp helper or temporarily clear the old global cache before linting:

```bash
rm -rf "$(go env GOCACHE 2>/dev/null)/../golangci-lint" 2>/dev/null || true
# Preferred after v13 regeneration: the project uses ./.cache/golangci-lint automatically.
```

The safest one-time fallback with the existing golangci-lint binary is:

```bash
./bin/golangci-lint cache clean
```

After v13 generation this global cleanup is no longer required for normal project linting.

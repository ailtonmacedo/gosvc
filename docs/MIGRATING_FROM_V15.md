# Migrating projects from v15/v16 to v17

This guide separates **CLI/framework upgrade** from **preset-version upgrade**.

## 1. Back up / commit first

Commit or stash local work. `gosvc upgrade` creates its own rollback backup unless `--no-backup` is explicitly supplied.

```bash
git status
gosvc upgrade backups --project .
```

## 2. Preview framework/manifest migration

```bash
gosvc upgrade --project . --dry-run
```

Review every `CREATE`, `UPDATE`, `PROTECT` and manifest change before applying.

## 3. Projects created before preset versioning

v15-era projects may not have `project.preset_version`. Loading them applies the compatible default and an explicit upgrade persists current metadata.

```bash
gosvc upgrade --project . --dry-run
gosvc upgrade --project .
```

## 4. Upgrade a preset version explicitly

`minimal-api` and `postgres-api` keep their legacy `1.0.0` contracts and current `1.1.0` contracts. Regeneration never changes versions implicitly.

Preview:

```bash
gosvc upgrade \
  --project . \
  --preset-version 1.1.0 \
  --dry-run
```

Apply:

```bash
gosvc upgrade \
  --project . \
  --preset-version 1.1.0
```

The explicit preset migration canonicalizes the `project.yaml` preset version, updates managed templates/manifest metadata, and creates a rollback backup first.

## 5. Verify

```bash
make bootstrap
gosvc doctor --project .
make verify
```

CI should continue to use the strict drift policy:

```bash
make verify-strict
```

## 6. Roll back if needed

```bash
gosvc upgrade backups --project .
gosvc upgrade rollback --project . --backup latest --dry-run
gosvc upgrade rollback --project . --backup latest
```

## v16 -> v17 note

The most important certification change is internal: Real E2E is now capability-driven. `bare` and `worker` no longer attempt OpenAPI/sqlc generation, and a GORM project never requires `sqlc-real`.

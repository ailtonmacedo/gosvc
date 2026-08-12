# Project and preset upgrades

`gosvc upgrade` is the explicit migration boundary. Normal regeneration does **not** switch framework schemas or preset versions silently.

## Two independent targets

- `--to <version>` targets the **gosvc framework/binary version** used to validate and regenerate managed artifacts.
- `--preset-version <version>` targets the **existing preset's version**. It never changes the preset ID itself.

## Preview a framework upgrade

```bash
gosvc upgrade --project ./order-service --to 1.1.0 --dry-run
```

## Preview a preset upgrade

For example, migrate `postgres-api@1.0.0` to `postgres-api@1.1.0`:

```bash
gosvc upgrade \
  --project ./catalog-service \
  --preset-version 1.1.0 \
  --dry-run
```

The plan reports framework, preset and manifest-schema transitions plus every file action. A dry run creates no backup and writes nothing.

## Apply

```bash
gosvc upgrade --project ./catalog-service --preset-version 1.1.0
```

Before writing files, the command creates:

```text
.gosvc/backups/<timestamp>-<version>-schema<version>.zip
```

An explicit preset-version migration is allowed to canonicalize `project.yaml` because the user requested a configuration contract change. Other user-owned files remain protected.

The update is staged in a sibling temporary directory and activated atomically. Manifest metadata is written only after the staged project is ready.

`--force` only changes managed generated files that otherwise conflict. It is not a global permission to overwrite user-owned code.

For disposable CI environments only:

```bash
gosvc upgrade --project . --no-backup
```

## Backups and rollback

```bash
gosvc upgrade backups --project .

gosvc upgrade rollback \
  --project . \
  --backup latest \
  --dry-run

gosvc upgrade rollback --project . --backup latest
```

Rollback restores the complete project snapshot and validates project identity before activation.

## Upgrade notes

```bash
gosvc upgrade notes --from 1.0.0 --to 1.1.0
```

## Compatibility policy

- no implicit preset upgrade during `new` regeneration;
- framework downgrades are rejected; use a rollback backup;
- preset target must exist in the built-in version registry;
- component configuration must be supported by the target preset version;
- development binaries may target explicit framework versions for testing;
- release binaries only upgrade framework metadata to their own version;
- unknown future schemas are rejected rather than guessed.

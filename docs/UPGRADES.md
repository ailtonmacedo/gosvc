# Project upgrades

`gosvc upgrade` is the only command that may migrate generator metadata between
schema versions. Normal regeneration deliberately rejects legacy manifest
schemas so schema changes remain explicit and reviewable.

## Preview an upgrade

```bash
gosvc upgrade --project ./order-service --to 1.1.0 --dry-run
```

The plan reports:

- source and target framework versions;
- source and target manifest schemas;
- generated files that will be created or updated;
- user-owned files that will be protected;
- conflicts that require manual resolution or `--force`.

A dry run does not create a backup and does not modify the project.

## Apply an upgrade

```bash
gosvc upgrade --project ./order-service --to 1.1.0
```

Before writing files, the command creates a ZIP archive under:

```text
.gosvc/backups/<timestamp>-<version>-schema<version>.zip
```

The archive excludes previous backups, includes metadata about the project and
source framework, and is preserved through later upgrades and rollbacks.

The update is staged in a sibling temporary directory and activated through an
atomic directory swap. An upgrade record is appended to
`.gosvc/manifest.json` only after the swap succeeds.

`--force` only overwrites managed generated files. User-owned files remain
protected by the ownership rules used by the generator.

For exceptional CI or disposable environments, backup creation may be disabled
explicitly:

```bash
gosvc upgrade --project . --no-backup
```

Disabling backups is not recommended for normal project upgrades.

## List backups

```bash
gosvc upgrade backups --project .
```

The output includes creation time, source framework version, archive path, and
archive size.

## Preview a rollback

```bash
gosvc upgrade rollback \
  --project . \
  --backup latest \
  --dry-run
```

A rollback restores the complete project snapshot, including user-owned files.
Review and commit or stash local changes before applying it.

## Apply a rollback

```bash
gosvc upgrade rollback --project . --backup latest
```

The selected archive is extracted into a temporary sibling directory, validated
against the current project identity, and activated atomically. The backup
catalog is copied into the restored project before the swap, so older backups
remain available. A rollback record is appended to manifest schema v3.

## Upgrade notes

```bash
gosvc upgrade notes --from 1.0.0 --to 1.1.0
```

The command prints registered migration notes for the requested semantic-version
range, including schema changes, backup behavior, and rollback support.

## Compatibility policy

- Downgrades through `gosvc upgrade` are rejected; use an explicit rollback backup.
- A release binary upgrades only to its own version.
- Development binaries may target an explicit semantic version for testing.
- Unknown future schemas are rejected rather than guessed.
- Schema migrations are sequential and deterministic.
- Backups are validated against project name and module before restoration.

# Sprint 14 — Upgrade safety, backups, and rollback

## Objective

Close the post-release upgrade lifecycle by making upgrades recoverable and
compatibility metadata explicit.

## Delivered

### Manifest schema v3

The generated project manifest now records:

- project compatibility metadata;
- the minimum `gosvc` version expected to understand the project;
- the last `gosvc` version that validated or regenerated the project;
- the backup associated with each upgrade record;
- rollback history.

Legacy manifest schemas 1 and 2 remain readable by `gosvc upgrade` and migrate
explicitly to schema 3.

### Automatic pre-upgrade backups

Unless `--no-backup` is supplied, `gosvc upgrade` creates a ZIP archive under:

```text
.gosvc/backups/
```

The backup:

- is created before any project write;
- excludes previous backup archives;
- preserves regular-file permissions;
- rejects symbolic links and unsupported file types;
- includes versioned project metadata;
- remains available after later upgrades and rollbacks.

### Backup catalog

```bash
gosvc upgrade backups --project .
```

The command prints timestamp, source framework version, archive path, and size.

### Atomic rollback

```bash
gosvc upgrade rollback --project . --backup latest --dry-run
gosvc upgrade rollback --project . --backup latest
```

Rollback:

- validates the archive against project name and Go module;
- blocks absolute paths and path traversal;
- extracts into a temporary sibling directory;
- preserves the complete backup catalog;
- activates the restored tree atomically;
- records rollback history in the manifest.

Rollback restores the complete snapshot, including user-owned files.

### Upgrade notes

```bash
gosvc upgrade notes --from 1.0.0 --to 1.1.0
```

The command prints registered migration guidance for a semantic-version range.

### Plugin preservation fix

Core upgrades now retain plugin-produced artifacts, producer metadata, plugin
references, and namespaced plugin features.

### Documentation and contracts

Added or updated:

- `docs/UPGRADES.md`;
- `docs/BACKUPS.md`;
- `docs/DEPRECATIONS.md`;
- `docs/COMPATIBILITY.md`;
- `schema/manifest.schema.json` v3;
- `schema/upgrade-backup.schema.json` v1;
- `schema/compatibility-matrix.json`;
- shell completions;
- README and changelog.

## Validation

Executed successfully:

```bash
make verify
go test -race ./...
go vet ./...
go build ./...
```

A complete demonstration also validated:

1. a simulated v1.0.0 project with manifest schema 2;
2. upgrade dry-run to 1.1.0;
3. automatic backup creation;
4. preservation of a customized README during upgrade;
5. project validation after upgrade;
6. rollback dry-run;
7. atomic rollback;
8. removal of a file created after the backup;
9. restoration of the customized README;
10. rollback history in the restored manifest.

The v1.1.0 release assets were generated for Linux, macOS, and Windows on amd64
and arm64 and passed offline release verification, including host-binary
execution.

## Operational note

The release candidate is rendered for `acme/gosvc` while the source module still
uses `github.com/ailtonmacedo/gosvc`. Before official publication, run:

```bash
gosvc release prepare --repository ailtonmacedo/gosvc
```

Then regenerate the release without `--allow-placeholder`.

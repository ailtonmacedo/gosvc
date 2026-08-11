# Upgrade Backups

`gosvc upgrade` creates a local ZIP snapshot before applying an upgrade unless
`--no-backup` is explicitly provided.

## Location

```text
.gosvc/backups/
```

Backups are intentionally stored inside the project so they remain available
when the project directory is moved. Teams may exclude this directory from
source control and copy archives to their normal backup storage.

## Archive contents

Each archive contains:

- every regular project file at the time of the upgrade;
- the source `.gosvc/manifest.json`;
- `.gosvc-backup.json` metadata;
- file permissions required by scripts and executables.

The existing `.gosvc/backups` directory is excluded to prevent recursive archive
growth.

## Security properties

Creation and restoration reject:

- symbolic links;
- unsupported file types;
- absolute archive paths;
- `..` path traversal;
- mismatched project name or Go module identity;
- unknown backup metadata schemas.

Rollback extraction occurs in a sibling temporary directory. The restored tree
is activated only after extraction and manifest migration succeed.

## Operational guidance

Before a rollback:

1. Commit or stash local changes.
2. Run `gosvc upgrade rollback --dry-run`.
3. Confirm the selected project name, module, framework version, and timestamp.
4. Apply the rollback.
5. Run `gosvc validate` and the project's normal test suite.

A rollback restores user-owned files too. It is a full project snapshot, not
only a generated-file rollback.

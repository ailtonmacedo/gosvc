# Deprecation Policy

The `gosvc` v1 release line follows a predictable deprecation process.

## Lifecycle

1. A feature is documented as deprecated in a minor release.
2. The CLI emits a warning when the deprecated feature is detected.
3. A supported replacement and migration path are documented.
4. Removal occurs no earlier than the next major release, except for urgent
   security fixes where preserving compatibility would keep a known vulnerability.

## Contracts covered

The policy applies to:

- CLI commands and flags;
- `project.yaml` fields;
- manifest fields and schema versions;
- plugin manifest and execution protocol fields;
- generated directory and ownership conventions;
- built-in preset names and feature identifiers.

## Compatibility metadata

Manifest schema v3 records:

```json
{
  "compatibility": {
    "minimum_gosvc_version": "1.1.0",
    "last_validated_gosvc_version": "1.1.0"
  }
}
```

`minimum_gosvc_version` indicates the oldest CLI version expected to understand
the project metadata. `last_validated_gosvc_version` records the CLI version that
most recently regenerated or upgraded the project.

## Current deprecations

There are no deprecated public contracts in `gosvc` 1.1.0.

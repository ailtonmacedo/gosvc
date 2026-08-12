# Internal and user-facing contracts

`gosvc` has several independently versioned contracts. They serve different purposes and must not be confused with each other.

| Contract | Current | Location | Written by | Read by | Purpose |
|---|---:|---|---|---|---|
| Project configuration | v1 | `project.yaml` | generator + explicit upgrade | `new`, `validate-config`, `doctor`, `validate`, `upgrade`, `add resource` | user-editable desired configuration |
| Generator manifest | v4 | `.gosvc/manifest.json` | generator / upgrade / plugin apply | regeneration, upgrade, rollback, project validation | internal ownership, checksums, preset version and compatibility metadata |
| Resource registry | v1 | `.gosvc/resources.json` | `add resource` | regeneration / resource rendering | stable resource definitions and migration allocation |
| Plugin manifest | v3 | `.gosvc/plugins/<name>/plugin.json` | plugin author | plugin loader/runtime | execution mode, capabilities, digest/checksum and minimum gosvc version |
| Plugin request protocol | v1 | JSON on stdin | gosvc | plugin process/container | invocation context |
| Plugin response protocol | v1 | JSON on stdout | plugin | gosvc | diagnostics and artifact contribution |
| Acceptance report | v1 | JSON output | `gosvc acceptance` | CI / release evidence | deterministic generator contract |
| Certification report | v1 | JSON output | `gosvc certify` | CI / release evidence | static/real environment certification |

## `project.yaml` is not the manifest

`project.yaml` is the **user configuration**. It is intended to be readable, reviewable and version-controlled.

```yaml
schema_version: 1
project:
  name: catalog-service
  module: github.com/acme/catalog-service
  preset: postgres-api
  preset_version: 1.1.0
```

`.gosvc/manifest.json` is **generator state**. It records which files are managed, their checksums, producer/ownership metadata, compatibility and the resolved preset version. Editing it manually is unsupported.

A `project.yaml` schema change and a manifest schema change are independent events. For example, `project.yaml` can remain schema v1 while the internal manifest evolves from v3 to v4.

## Contract ownership

- normal regeneration may update only generator-owned artifacts according to manifest checksums;
- user-owned artifacts are protected;
- explicit `gosvc upgrade` may migrate configuration/metadata when the user requests a schema or preset-version transition;
- plugins can contribute artifacts only through protocol v1; direct writes are never the artifact contract.

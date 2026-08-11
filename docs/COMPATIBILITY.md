# Compatibility Policy

## Stable contracts in v1.x

The following contracts are versioned and compatibility-sensitive:

| Contract | Current version | Compatibility rule |
|---|---:|---|
| `project.yaml` schema | 1 | New optional fields may be added in v1.x; removals require a major release |
| `.gosvc/manifest.json` schema | 3 | Older supported schemas require explicit migration through `gosvc upgrade` |
| Plugin manifest schema | 2 | Unknown fields are rejected; schema upgrades require plugin migration |
| Plugin execution protocol | 1 | Request and response shapes remain backward compatible in v1.x |
| Resource registry schema | 1 | Existing resource definitions and migration numbers remain stable |
| Acceptance report schema | 1 | Existing JSON fields remain available throughout v1.x |
| Certification report schema | 1 | PASS/FAIL/BLOCKED semantics remain backward compatible throughout v1.x |

## Go compatibility

| Component | Requirement |
|---|---|
| `gosvc` CLI | Go version declared by the repository `go.mod` |
| `minimal-api` output | Resolves the local supported Go version |
| Database-backed presets | Go 1.25 or newer |

Database-backed presets use current pgx and OpenTelemetry dependency lines and therefore intentionally target Go 1.25.

## Generated project compatibility

Within the v1 release line:

- regeneration must remain idempotent;
- user-owned files must never be overwritten automatically;
- generated-file changes require checksum validation;
- existing migration numbers must not be reassigned;
- plugin-produced files retain their producer and ownership metadata;
- a project is never upgraded implicitly during normal regeneration.

Breaking template changes must be delivered through `gosvc upgrade`, with a dry-run plan, automatic backup, manifest history, and an explicit rollback path.

## Supported release platforms

Official archives are produced for:

| Operating system | Architectures |
|---|---|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |

Shell completion is generated for Bash, Zsh, Fish, and PowerShell. Homebrew metadata supports Linux and macOS; Scoop metadata supports Windows.

## External infrastructure

Generated projects may target PostgreSQL, Redis, Kafka, Prometheus, OpenTelemetry Collector, Docker, and Kubernetes. Exact server-version support should be pinned and validated by each generated project's connected CI environment. The offline acceptance matrix verifies configuration and generated code structure. `gosvc certify --mode real` is the authoritative live interoperability gate for the Go toolchain, Docker, PostgreSQL, Redis, Kafka, code generators, migrations, HTTP flows, authentication, and Outbox publication.

## Deprecation policy

A feature scheduled for removal should be documented as deprecated for at least one minor release before removal. Security fixes may require faster changes when compatibility would preserve a known vulnerability.

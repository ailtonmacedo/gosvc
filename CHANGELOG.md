# Changelog

All notable changes to `gosvc` are documented here. The project follows Semantic Versioning.

## [1.1.0] - 2026-08-06

### Added

- `gosvc certify` with static and real integration/E2E modes, PASS/FAIL/BLOCKED reporting, and JSON evidence.
- Real certification workflow covering code generation, PostgreSQL migrations, HTTP CRUD, JWT/RBAC/refresh, Redis, Kafka, Outbox publication, lint, vulnerability scan, and Docker builds.
- Certification report JSON Schema and compatibility contract.
- Manifest schema v3 with compatibility metadata and rollback history.
- Automatic ZIP backups before project upgrades.
- `gosvc upgrade backups` for backup catalog inspection.
- `gosvc upgrade rollback` with dry-run and atomic project restoration.
- `gosvc upgrade notes` for version-range migration guidance.
- A formal deprecation policy for CLI, project, manifest, plugin, and preset contracts.

### Publication rehearsal

- Added `gosvc release github-plan` with read-only GitHub publication readiness checks and machine-readable JSON output.
- Added module/origin/workflow/changelog/governance cross-checks to the release preflight once the final module identity is prepared.
- Added a formal GitHub publication guide, recommended branch protection, and publication-plan JSON Schema.

### Fixed

- Released Kafka idempotency reservations when DLQ publication fails, preventing a failed message from being silently treated as already processed.
- Added distributed integration coverage for Redis idempotency and Kafka publish/consume paths.
- Preserved plugin-produced artifacts and producer metadata during core upgrades.
- Validated backup project identity before rollback.
- Preserved the complete backup catalog after a rollback.
- Completed generated module metadata with `go mod tidy` before real E2E compilation.
- Aligned generated sqlc configuration with Go 1.25 and mapped non-null `timestamptz` columns to `time.Time`.
- Removed/conditioned unused HTTP helpers and normalized Kafka validation errors so generated presets pass strict linting.

## [1.0.0] - 2026-08-05

### Added

- Four project presets: `minimal-api`, `postgres-api`, `production-api`, and `event-driven-api`.
- Clean Architecture scaffolding, Chi HTTP server, graceful shutdown, health checks, Docker, and CI.
- PostgreSQL with pgxpool, migrations, sqlc, OpenAPI, ReDoc, request validation, and CRUD generation.
- JWT authentication, refresh-token rotation, RBAC, rate limiting, Prometheus, OpenTelemetry, and private pprof.
- Redis idempotency, Kafka adapters, Transactional Outbox, worker process, DLQ handling, and Kubernetes manifests.
- Manifest schema migrations, project upgrades, executable plugins, checksums, ownership, and atomic application.
- Shell completion for Bash, Zsh, Fish, and PowerShell.
- Deterministic cross-platform release archives, checksums, SPDX SBOM, and release manifest.
- Release preflight and snapshot commands.

### Publication hardening

- Added transactional module-path preparation with `gosvc release prepare`.
- Added offline release verification with host-binary execution.
- Added repository-aware installer rendering and local mirror support.
- Added generated Homebrew formula and Scoop manifest using release archive hashes.
- Added module, repository, and Git-origin consistency checks.

### Acceptance hardening

- Added `gosvc acceptance` with human-readable and JSON reports.
- Added generation and idempotency checks for all built-in presets.
- Added representative UUID CRUD generation for database-backed presets.
- Added automatic acceptance execution to release preflight.
- Added generator benchmarks and a documented v1 compatibility policy.

### Release evidence hardening

- Added bounded parallel cross-platform builds through `release snapshot --parallel`.
- Added deterministic `RELEASE_NOTES.md` extraction from the matching changelog section.
- Added `release-evidence.json` with stable acceptance results and quality gates.
- Added strict verification for all required platform archives and publication assets.
- Added a public JSON Schema and documentation for release evidence.

### Security

- Managed-file checksums and conflict detection.
- Plugin executable checksums, timeouts, output limits, path validation, and temporary workspaces.
- Non-root container defaults, private profiling endpoint, strong JWT configuration, and secret handling guidance.

[1.1.0]: https://github.com/ailtonmacedo/gosvc/releases/tag/v1.1.0
[1.0.0]: https://github.com/ailtonmacedo/gosvc/releases/tag/v1.0.0

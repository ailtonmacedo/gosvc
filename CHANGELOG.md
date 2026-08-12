# Changelog

All notable changes to `gosvc` are documented here. The project follows Semantic Versioning.

## [Unreleased] — E2E v17

### Fixed — capability-driven real certification

- Real E2E now derives OpenAPI, sqlc, database, Compose, Redis, Kafka, Outbox, HTTP and worker checks from the effective generated configuration instead of assuming every preset is an HTTP/database API.
- `bare` and `worker` now record OpenAPI/sqlc checks as `SKIPPED`/not applicable instead of failing on missing `api/oapi-codegen.yaml`.
- `postgres-api` with GORM no longer requires `sqlc-real`.
- Added a standalone worker lifecycle smoke and a non-HTTP app smoke to real certification.
- Updated certification report schema v1 to enumerate all six built-in presets.
- Real certification now compares the full Go major/minor/patch tuple, so an insecure `go1.25.4` host cannot satisfy the `go1.25.12` security floor.

### Added — contracts and upgrade usability

- Added explicit legacy `minimal-api@1.0.0` and `postgres-api@1.0.0` definitions alongside current `1.1.0` versions.
- Added `gosvc upgrade --preset-version <version>` with dry-run, backup, explicit `project.yaml` migration and rollback compatibility.
- Added contract documentation distinguishing `project.yaml` from `.gosvc/manifest.json` and mapping plugin/report contracts to their readers/writers.
- Added copyable plugin protocol request/response examples and migration guidance from v15/v16.
- Documented the exact coverage N/A detection rule, CLI-vs-Makefile verification roles and sqlc/GORM driver/pool contracts.

### Added

- Added versioned `bare` and `worker` presets for non-HTTP workloads.
- Added controlled component selection: Echo for `minimal-api`/`postgres-api`, and GORM for `postgres-api`, while production/distributed presets remain on Chi + sqlc.
- Added `gosvc presets list/show` and `--preset-version`, making preset versions an explicit contract independent from the CLI version.
- Added plugin manifest schema v3 with Docker-isolated execution, digest-pinned images, read-only mounts, dropped capabilities, no-new-privileges, resource limits and network-off-by-default.
- Added `--require-sandbox` and explicit `--allow-network` plugin execution policy flags.
- Added an offline component regression matrix covering bare, worker, Chi/Echo and sqlc/GORM supported combinations.

### Changed

- Centralized supported component combinations inside the versioned preset registry so CLI inspection and project validation share one contract.
- Made generated Makefiles install sqlc only when the database is enabled and sqlc is actually selected.
- Made the empty `bare` scaffold coverage gate report N/A until business-layer statements exist instead of failing with a misleading 0%.
- Updated generated README and user documentation for the six presets, controlled composition, preset versions and plugin isolation.

## [1.1.0] - 2026-08-06

### Changed — E2E v15 consistency hardening

- Documentation revision 15.1 separates installed-CLI and source-checkout Quickstarts, fixes the onboarding order to bootstrap before project-level doctor, and adds explicit PATH/binary guidance.
- Standardized the four canonical preset IDs across CLI, project contracts, reports and documentation: `minimal-api`, `postgres-api`, `production-api`, and `event-driven-api`.
- Moved current usage documentation away from historical E2E fix narratives; the visual guide now focuses on present behavior and links back to this changelog for history.
- Added explicit generated-runtime policy in `project.yaml` under `runtime.go.language` and `runtime.go.toolchain`, while preserving read compatibility with legacy `project.go_version`.
- Added `gosvc doctor --preset <name>` so preset requirements can be checked before a project exists.
- Added generated-code architecture examples, a command cookbook, a plugin threat model, troubleshooting guidance, and a documented template regression matrix.
- Strengthened offline acceptance to reload every generated `project.yaml` and verify the canonical preset ID plus Go runtime policy before idempotency checks.

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

- Added a Go 1.25 security floor: database-backed presets keep `go 1.25.0` language compatibility but generate `toolchain go1.25.12`, Docker uses `golang:1.25.12-bookworm`, and doctor/real certification reject older Go 1.25 patch levels.
- Isolated `golangci-lint` cache per generated project via `GOLANGCI_LINT_CACHE=$(CURDIR)/.cache/golangci-lint`, preventing stale absolute paths when a service is regenerated or moved outside the gosvc source repository.
- Marked the PostgreSQL `timeToDB` template helper as an intentional generated-resource extension hook, preventing the base `postgres-api` preset from failing the `unused` linter before any `datetime` resource is added.
- Made `gosvc new` print the exact `cd` command for the resolved destination and warn when a stale legacy `gosvc/<project>` directory could be mistaken for the new sibling project.
- Added `make bootstrap` plus developer/CI drift modes: local `make verify` reconciles and reports module/generated drift, while generated CI and `make verify-strict` enforce zero tracked drift with `STRICT_GIT_DRIFT=1`.
- Added `make install` for source checkouts and made `gosvc version` print the resolved executable path, preventing an older PATH binary from being confused with the freshly built generator.
- Made `gosvc new` place projects beside the gosvc source repository when invoked from inside that checkout, preventing generated projects from inheriting the framework repository's Git state.
- Made generated `tidy-check` and `generate-check` run Git drift enforcement only when the generated project is the Git worktree root, so first-run verification works before `git init` and inside a parent repository.
- Clarified installation guidance so `@main` is used before the `v1.1.0` tag exists.
- Released Kafka idempotency reservations when DLQ publication fails, preventing a failed message from being silently treated as already processed.
- Added distributed integration coverage for Redis idempotency and Kafka publish/consume paths.
- Preserved plugin-produced artifacts and producer metadata during core upgrades.
- Validated backup project identity before rollback.
- Preserved the complete backup catalog after a rollback.
- Completed generated module metadata with `go mod tidy` before real E2E compilation.
- Aligned generated sqlc configuration with Go 1.25 and mapped non-null `timestamptz` columns to `time.Time`.
- Removed/conditioned unused HTTP helpers and normalized Kafka validation errors so generated presets pass strict linting.
- Provisioned Redpanda topics explicitly before distributed integration tests instead of relying on broker auto-creation.
- Configured OpenAPI request validation with an explicit Bearer authentication function for protected routes while keeping JWT signature/claims validation in the application authentication middleware.

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

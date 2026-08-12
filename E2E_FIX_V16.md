# E2E_FIX_V16 — Composable Presets, Versioned Contracts, and Plugin Isolation

## Scope

E2E v16 implements the functional improvements deliberately deferred from v15 while preserving the existing golden path. The release adds versioned preset contracts, two non-HTTP starting points, controlled router/persistence alternatives, a Docker-isolated plugin execution mode, and a stronger component regression matrix.

## 1. Versioned preset contracts

Preset versions are now independent from the gosvc CLI version and are persisted in both `project.yaml` and the gosvc manifest.

Current built-in contracts:

| Preset | Version | Kind |
|---|---:|---|
| `bare` | `1.0.0` | app |
| `worker` | `1.0.0` | worker |
| `minimal-api` | `1.1.0` | api |
| `postgres-api` | `1.1.0` | api |
| `production-api` | `1.0.0` | api |
| `event-driven-api` | `1.0.0` | api |

The CLI exposes the registry through `gosvc presets list` and `gosvc presets show <name> [version]`. Regeneration rejects a preset-version mismatch instead of silently applying templates from a different contract.

## 2. Controlled component composition

The preset registry is the source of truth for supported component combinations.

- `minimal-api`: `chi` (default) or `echo`.
- `postgres-api`: `chi` or `echo`; `sqlc` (default) or `gorm`.
- `production-api`: certified `chi + sqlc` path only.
- `event-driven-api`: certified `chi + sqlc` path only.

Unsupported combinations fail during configuration validation, before project files are written. This intentionally avoids an unbounded cross-product of untested framework choices.

## 3. New `bare` and `worker` presets

`bare` generates a Clean Architecture application scaffold without HTTP/database adapters. `worker` generates a long-running background-worker scaffold with context cancellation and tests, also without HTTP/database adapters.

The generated Makefile is component-aware, so non-database projects do not install sqlc/migrate and non-OpenAPI projects do not install oapi-codegen.

A fresh `bare` project has no business-layer statements yet; its coverage gate reports `n/a` rather than treating an empty coverage domain as `0%`. Once business statements exist, the normal coverage minimum is enforced.

## 4. Echo and GORM adapters

Echo templates cover routing, OpenAPI server generation, request handling and CRUD resource generation. GORM templates cover PostgreSQL connection management, transactions, repositories and integration-test scaffolding while preserving the same application/domain ports.

The golden path remains Chi + pgx/pgxpool + sqlc.

## 5. Plugin schema v3 and Docker isolation

Plugin schema v3 adds an explicit execution policy:

- `native`: backward-compatible trusted executable with checksum/snapshot controls;
- `docker`: digest-pinned container execution.

Docker execution uses a temporary project snapshot and applies a hardened runtime policy including read-only root filesystem, read-only project bind mount, dropped Linux capabilities, `no-new-privileges`, non-root execution, PID/memory/CPU limits, tmpfs for `/tmp`, and network disabled by default.

`--require-sandbox` rejects native plugins. A Docker plugin that declares network access requires the caller to also pass `--allow-network`.

This is process/container isolation, not a claim that arbitrary third-party code is risk-free. Residual Docker/host/runtime risks are documented in `docs/PLUGIN_SECURITY.md`.

## 6. Manifest/schema compatibility

The gosvc manifest schema advances to v4 and records `preset_version`. Migration from the previous manifest schema is supported. Plugin schema advances to v3 while schema-2 native plugins remain compatible.

## 7. Regression matrix

In addition to the built-in preset acceptance matrix, v16 compiles/tests the supported component combinations deterministically:

1. bare
2. worker
3. minimal-api + Chi
4. minimal-api + Echo
5. postgres-api + Chi + sqlc
6. postgres-api + Echo + sqlc
7. postgres-api + Chi + GORM
8. postgres-api + Echo + GORM

The offline compile harness uses deterministic local dependency stubs so template compatibility can be checked without relying on network downloads. Real certification remains a separate Docker-backed gate.

## 8. Documentation

The visual guide, README, quickstart, preset reference, commands, plugin security guide, compatibility guide, acceptance guide and template-regression guide were updated for the v16 contracts.

## Validation status

The v16 package is considered complete only after the framework gates, acceptance matrix, static certification, HTML integrity checks and archive integrity checks pass. Real Docker E2E certification is reported separately and must not be marked PASS unless it is actually executed successfully.

## Final validation evidence

Executed successfully against the v16 source tree:

- `gofmt` cleanliness — PASS.
- `go test ./...` — PASS.
- `go vet ./...` — PASS.
- `go build ./...` — PASS.
- `make verify` — PASS.
- `go test -race ./...` — PASS.
- built-in acceptance — **6 passed / 0 failed**.
- static certification — **6 passed / 0 failed / 0 blocked / 0 skipped**.
- component regression matrix — **8/8 supported component variants PASS**.
- JSON schemas/examples syntax validation — PASS.
- release preflight for `1.1.0` — PASS.
- visual HTML integrity — 26 sections, 5 inline SVG diagrams, no broken internal anchors.

Real Docker E2E certification was **not executed in this validation environment** because the Docker command is unavailable. This status is intentionally not promoted to PASS.

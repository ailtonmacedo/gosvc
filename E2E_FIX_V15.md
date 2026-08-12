# E2E v15 — Consistency & Documentation Hardening

## Goal

Consolidate the behavior accumulated through the previous E2E fixes into one coherent public contract instead of adding another major feature family.

## Changes

### Canonical preset naming

The public preset IDs are now documented and regression-tested as:

```text
minimal-api
postgres-api
production-api
event-driven-api
```

Acceptance reloads every generated `project.yaml` and fails when the persisted ID differs from the preset being certified.

### Explicit Go runtime policy

New generated configurations use:

```yaml
runtime:
  go:
    language: 1.25.0
    toolchain: go1.25.12
```

The generator still reads the legacy `project.go_version` field for v1 compatibility. Declaring both formats with conflicting language versions is rejected.

The generated `go.mod` continues to emit:

```go
go 1.25.0

toolchain go1.25.12
```

Doctor and real certification calculate the effective host requirement from both the language contract and the explicit toolchain.

### Pre-generation doctor

`gosvc doctor` now accepts:

```bash
gosvc doctor --preset postgres-api
```

This mode can run before `project.yaml` exists. Host/runtime prerequisites are enforced; project-managed local tools are reported but may be installed later by `make bootstrap`.

After generation, the strict project-aware check remains:

```bash
gosvc doctor --project .
```

### Template regression hardening

Acceptance now validates, per preset:

- canonical preset ID;
- Go runtime language/toolchain policy;
- manifest and architecture;
- representative resource generation for database-backed presets;
- project and resource idempotency.

Real certification remains the authoritative connected regression gate for actual toolchains, code generators, Docker, PostgreSQL, auth, Redis, Kafka, Outbox and worker flows.

### Documentation architecture

The standalone HTML guide was rewritten around current behavior. Historical E2E fix chapters were removed from the user guide and remain available through `CHANGELOG.md` and `E2E_FIX_*.md`.

New focused documents:

- `docs/QUICKSTART.md`
- `docs/PRESETS.md`
- `docs/ARCHITECTURE.md`
- `docs/COMMANDS.md`
- `docs/PLUGIN_SECURITY.md`
- `docs/TEMPLATE_REGRESSION.md`
- `docs/TROUBLESHOOTING.md`

The visual guide now includes current architecture, Go policy, generated-code examples, canonical preset matrix, auth separation, Outbox flow, template regression, CI/CD, and plugin threat-model visuals.

# E2E Fix v14 — Go 1.25 security floor

## Symptom

`make verify` reached `govulncheck` after formatting, tidy, code generation, vet, lint, unit tests, coverage and race detection had passed. The scan reported 15 reachable vulnerabilities from the Go standard library while the host was running Go 1.25.4. The highest fixed patch in the report was Go 1.25.12.

## Root cause

Database-backed presets declared the Go 1.25 language line but did not pin a patched build toolchain. A developer could therefore build with an older Go 1.25 patch whose standard library still contained known vulnerabilities. Docker inherited the same unpatched version selector.

## Fix

- Keep `go 1.25.0` as the language compatibility contract.
- Generate `toolchain go1.25.12` for Go 1.25 database-backed presets.
- Use `golang:1.25.12-bookworm` in generated Dockerfiles.
- `doctor` and real certification enforce the effective runtime security floor.
- Generated GitHub Actions uses `actions/setup-go@v7` with `go-version-file: go.mod`; setup-go reads the toolchain directive when present.
- Offline generator tests use `GOTOOLCHAIN=local` only for their isolated stub compilation so framework tests do not need network access.

## Security invariant

A project may retain Go 1.25.0 language semantics while builds and security gates execute with Go 1.25.12 or newer. The vulnerability gate remains fail-closed; no `govulncheck` finding is suppressed.

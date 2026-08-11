# Contributing

## Development setup

```bash
make tidy
make verify
make build
```

Changes should include tests. Generator changes must also cover idempotency, ownership, and compilation of generated projects.

## Pull requests

1. Keep each pull request focused.
2. Explain architectural or compatibility implications.
3. Add or update tests and documentation.
4. Run `make verify` and `go test -race ./...`.
5. Do not edit generated fixtures without regenerating and reviewing them.

## Commit guidance

Use clear imperative subjects, such as `add release preflight` or `fix plugin ownership preservation`.

## Reporting security issues

Do not open public issues for vulnerabilities. Follow `SECURITY.md`.

## Preset acceptance

Changes to templates, presets, manifests, resources, or project validation must pass:

```bash
make acceptance
make benchmark
```

Attach `gosvc acceptance --json` output to changes that intentionally alter generated file counts or compatibility contracts.

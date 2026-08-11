# Real E2E hardening

This revision addresses the failures observed in the GitHub Actions real-certification run for commit `d337d6d`.

## Generator fixes

- Removed `chi/middleware.RealIP`. The generated application now uses the connection peer from `http.Request.RemoteAddr` unless an application-specific trusted-proxy strategy is added explicitly.
- Database presets initialize readiness directly from `pgxpool.Pool.Ping` instead of assigning a placeholder and overwriting it.
- Route registrars are initialized at declaration time; the minimal preset passes `nil` directly.
- Authentication logout reports `domain.ErrInvalidRefreshToken` when refresh-token hashing fails while the HTTP logout endpoint remains idempotent and returns 204.
- Chi remains pinned to `v5.3.0`.

## Certification behavior

The real certification now runs independent quality gates as a diagnostic batch:

1. `go test ./...`
2. `go vet ./...`
3. `go build ./...`
4. `golangci-lint run`
5. `govulncheck ./...`

If the project still builds, Docker/runtime checks may continue even when lint or vulnerability checks fail. The preset still fails overall, but the report contains more actionable information from one CI run.

Generation, code-generation (`oapi-codegen`, `sqlc`) and `go mod tidy` remain fail-fast because later checks are not reliable when these stages fail.

## Local evidence

The repository was validated with:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./...`
- `make verify`
- `gosvc acceptance --json` (4/4 presets pass)
- `gosvc certify --mode static --require-real --json` (4/4 presets pass)
- regression tests that render all presets and reject the lint patterns observed in the real CI run.

The container used to prepare this revision cannot resolve external module hosts and does not provide Docker or the real CI tools, so the authoritative real-infrastructure result remains the GitHub Actions `Real E2E Certification` workflow.

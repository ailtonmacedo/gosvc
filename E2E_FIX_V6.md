# Real E2E v6 — PostgreSQL readiness hardening

## Evidence from the previous GitHub Actions run

The previous run proved that generation, code generation, Go quality gates,
vulnerability scanning, and Docker builds are working. `minimal-api` passed end
to end. The three database-backed presets failed only at the first migration,
immediately after the old readiness check incorrectly accepted PostgreSQL as
ready.

Observed failure in all three database-backed presets:

```text
postgres-health: PASS (0 ms)
migrate-up: FAIL (4 ms)
connection reset by peer
```

## Root cause

The old certification logic treated either `running` OR `healthy` in
`docker compose ps --format json` output as ready. A PostgreSQL container can
be in `running` state while its healthcheck still reports `starting`, so the
host-side migration raced database initialization.

## Fix

1. Start dependencies with Docker Compose `up -d --wait --wait-timeout`.
2. Parse Compose JSON status and require `State=running` AND `Health=healthy`
   for PostgreSQL, Redis, and Kafka.
3. Add `start_period` and faster healthcheck cadence to generated Compose files.
4. Retry `migrate up` for at most 15 seconds only for transient startup/network
   failures such as connection reset/refused; SQL/migration errors are not
   retried.
5. If migration still fails, automatically capture Compose status and the last
   200 PostgreSQL log lines in the certification report.

## Local validation

```text
go test ./...          PASS
go test -race ./...    PASS
go vet ./...           PASS
go build ./...         PASS
make verify             PASS
acceptance              4/4 PASS
static certification    4/4 PASS
```

The real Docker/PostgreSQL phase remains authoritative in GitHub Actions.

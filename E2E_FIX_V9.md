# E2E Fix v9 — Worker-scoped configuration

## Failure observed in GitHub Actions

The Real E2E Certification reached the final event-driven Outbox smoke test, but
`cmd/worker` exited during startup because the shared runtime configuration
validator required `JWT_SECRET` even though the worker does not use HTTP/JWT
authentication.

Observed failure:

```text
outbox event was not marked published
worker stopped error="load configuration: JWT_SECRET must contain at least 32 bytes"
```

At that point the other three presets already passed, and event-driven had
already passed compilation, lint, vulnerability scanning, Docker, PostgreSQL,
Redis, Kafka topic provisioning, integration tests, and the authenticated API
smoke suite.

## Root cause

The API and the asynchronous worker both called `config.Load()`. `Load()` runs
full API configuration validation, so API-only capabilities (JWT, HTTP, rate
limiting, cache, pprof, metrics) accidentally became startup dependencies of
the Outbox worker.

## Fix

- Added `config.LoadWorker()` and `Config.ValidateWorker()`.
- Worker validation is limited to the capabilities it actually consumes:
  application identity, PostgreSQL, Kafka, and Outbox settings.
- `cmd/worker` now uses `config.LoadWorker()`.
- Removed unused `JWT_SECRET` and `REDIS_ADDRESS` from the worker service in
  generated Compose configuration.
- The E2E Outbox smoke now fails fast if the worker process exits before marking
  the event as published, including the worker logs in a dedicated diagnostic
  check instead of waiting for the full command timeout.
- Added a generator regression test that verifies the worker-scoped contract.

## Local gates

```text
go test ./...          PASS
go test -race ./...    PASS
go vet ./...           PASS
go build ./...         PASS
make verify             PASS
Acceptance              4/4 PASS
Static Certification    4/4 PASS
```

The remaining proof is the Real E2E run on GitHub Actions, where Docker,
PostgreSQL, Redis, and Redpanda are available.

# E2E Fix v7 — Kafka topic provisioning and public auth OpenAPI contract

This update addresses the two remaining failures observed in the GitHub Actions Real E2E Certification run on 2026-08-10.

## 1. Kafka integration topic provisioning

The event-driven integration test publishes to `certification.integration`. The broker was healthy, but the topic did not exist, causing `UNKNOWN_TOPIC_OR_PARTITION`.

The certification runner now creates all certification topics explicitly after Redpanda is healthy and before integration tests run:

- `certification.integration`
- `certification.integration.dlq`
- `<topic_prefix>.certification`
- `<topic_prefix>.certification.dlq`

Topics are created with one partition and one replica, matching the single-broker certification environment.

## 2. Public authentication operations in OpenAPI

The generated OpenAPI document defines Bearer authentication globally for protected APIs. Login, refresh, and logout are public entry points and must override the global security requirement.

The generated operations now include:

```yaml
security: []
```

for:

- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`

Protected resource routes and `/admin/ping` continue to require `bearerAuth`.

## Regression coverage

Tests assert that:

- all three public authentication operations explicitly override global security;
- certification Kafka topic names are deterministic and include the main and DLQ topics;
- all existing generator, acceptance, certification, race, vet, and build gates continue to pass locally.

## Local gates executed

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./...`
- `make verify`
- `gosvc acceptance` — 4/4 PASS
- `gosvc certify --mode static --require-real` — 4/4 PASS

The Docker/Kafka/OpenAPI request-validation E2E itself requires the connected GitHub Actions runner and is therefore verified by the next Real E2E Certification run.

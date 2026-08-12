# E2E Fix v10 — Semantic Outbox Payload Validation

## Symptom

The real certification reached the final event-driven Outbox smoke test and failed with:

```text
Kafka payload not observed: {"ok": true}
```

At the same time, Kafka topic provisioning, Kafka integration tests, the HTTP/JWT smoke suite, migrations, lint, vulnerability scanning, and Docker builds had already passed.

## Root cause

The certification inserted `{"ok":true}` into a PostgreSQL `JSONB` column. PostgreSQL is free to normalize JSONB textual representation. The worker then published the normalized JSON payload, observed by the runner as:

```json
{"ok": true}
```

The certification incorrectly compared the Kafka message using exact string fragments that only accepted the compact representation without whitespace.

This was a false negative: the correct message had reached Kafka.

## Fix

The Outbox Kafka smoke check now parses the consumed record as JSON and validates the semantic invariant:

```text
payload.ok == true
```

JSON whitespace and formatting no longer affect certification correctness. Invalid JSON, a missing `ok` field, non-boolean values, or `ok: false` still fail the check.

The failure path now also records an explicit `outbox-kafka-smoke` failed check with the observed payload.

## Regression tests

Added cases for:

- compact JSON (`{"ok":true}`)
- PostgreSQL JSONB formatting (`{"ok": true}`)
- surrounding whitespace/newlines
- `ok: false`
- missing `ok`
- string `"true"` instead of boolean
- invalid JSON

# Real E2E Fix v8 — OpenAPI Bearer validation

## Evidence from the GitHub runner

The v7 runner proves the previous Kafka correction is effective:

- `kafka-topics`: PASS for `certification.integration`, its DLQ, `events.certification`, and its DLQ.
- `event-driven-api` integration tests: PASS.
- `minimal-api`: PASS.
- `postgres-api`: PASS including migrations, integration tests, CRUD smoke, and migration down/up.

The only remaining failures were `production-api` and `event-driven-api`, both while creating an order after a successful login:

```text
create order status=401
code=openapi_validation_failed
```

## Root cause

The generated OpenAPI contract correctly uses global `bearerAuth` security for protected operations and `security: []` for the public authentication operations. However, the request-validator middleware was instantiated without `openapi3filter.Options.AuthenticationFunc`.

That means a protected request was rejected by the OpenAPI validation layer before the existing JWT middleware (`AuthMiddleware.Authenticate`) could validate the access token.

## Fix

Protected presets now configure:

```go
Options: openapi3filter.Options{
    AuthenticationFunc: openAPIBearerAuthentication,
}
```

The contract-level authentication function validates only the OpenAPI requirement:

1. the security scheme must be `bearerAuth`;
2. an `Authorization` header must exist;
3. it must have a non-empty `Bearer <token>` shape.

Cryptographic JWT validation remains in `AuthMiddleware.Authenticate`, which continues to validate signature, expiry, issuer/audience, claims, and principal context. This avoids duplicating JWT policy inside the OpenAPI layer while still satisfying the OpenAPI security contract.

## Certification hardening

The real E2E smoke now proves all three states for production/event-driven presets:

1. missing Bearer token -> 401;
2. malformed/invalid JWT with Bearer shape -> 401 from the real JWT middleware;
3. valid login access token -> protected CRUD succeeds.

## Local gates

Before packaging v8:

```text
go test ./...          PASS
go test -race ./...    PASS
go vet ./...           PASS
go build ./...         PASS
make verify             PASS
acceptance 4/4          PASS
static certification    PASS 4/4
```

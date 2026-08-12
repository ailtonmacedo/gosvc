# Template Regression Matrix

Template changes are product changes. v17 verifies them at three levels.

## 1. Six canonical preset contracts

`gosvc acceptance` generates all six current presets and validates identity/version, Go runtime policy, manifest, Clean Architecture, representative CRUD where applicable and idempotency.

## 2. Eight supported component variants

| Variant | Generation | compile/tests/coverage |
|---|---:|---:|
| bare | ✅ | ✅ |
| worker | ✅ | ✅ |
| minimal + Chi | ✅ | ✅ |
| minimal + Echo | ✅ | ✅ |
| postgres + Chi + sqlc | ✅ | ✅ |
| postgres + Echo + sqlc | ✅ | ✅ |
| postgres + Chi + GORM | ✅ | ✅ |
| postgres + Echo + GORM | ✅ | ✅ |

Regression also asserts that GORM variants do not generate `sqlc.yaml`, and Echo variants do not retain Chi as a runtime dependency.

## 3. Real capability certification

Real E2E derives checks from effective configuration, not just preset ID. This prevents false failures for `bare`/`worker` and prevents sqlc from being required by GORM variants.

## Coverage N/A rule

The generated coverage script filters the Go coverage profile to statements whose package path matches:

```text
/internal/domain/
/internal/application/
/internal/infrastructure/http/
```

If the filtered profile contains only the coverage header and **zero monitored statements**, coverage is `N/A`. This is the expected state for a fresh `bare` scaffold.

The moment any monitored package contains executable statements, `go tool cover` computes the percentage and `quality.coverage.minimum` (80 by default) becomes mandatory.

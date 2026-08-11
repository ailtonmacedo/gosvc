# Sprint 16 — GitHub Publication Rehearsal

## Objective

Close the gap between local E2E certification and the first public GitHub tag without performing irreversible publication implicitly.

## Delivered

- `gosvc release github-plan` in human and JSON formats.
- Cross-checks for module path, repository identity, origin remote, branch, changelog, governance files, and publication workflows.
- A deterministic sequence of Git/GitHub CLI commands for the first publication.
- JSON Schema for machine-readable publication plans.
- GitHub publication documentation and branch-protection recommendations.
- Release preflight integration: after the module is prepared, `gosvc release check` also consumes the GitHub publication checks.
- Shell completions updated with `release github-plan`.

## Safety properties

- Publication planning is read-only.
- No repository is created automatically.
- No remote is modified automatically.
- No commit, push, tag, workflow dispatch, or GitHub Release is executed by the planner.
- A module/repository mismatch is a blocking failure.
- Missing critical workflows or governance files are blocking failures.
- Missing Git initialization/origin is a warning so a fresh source bundle can still produce a valid plan.

## Validation

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

The release-specific and high-cost race suites were also executed independently to avoid terminal timeout ambiguity.

## GitHub context

The authenticated GitHub account available during this sprint is `alfredovaras`. No accessible repository named `gosvc` was returned by the installed GitHub connector, so no remote mutation or publication was attempted.

A local publication rehearsal is generated separately using `github.com/ailtonmacedo/gosvc` as the candidate identity. This does not create the GitHub repository.

## Concrete publication rehearsal

Using the authenticated GitHub identity available in this environment, a local copy was prepared as:

```text
github.com/ailtonmacedo/gosvc
```

The prepared copy passed:

```bash
go test ./...
go vet ./...
go build ./...
gosvc release github-plan --repository ailtonmacedo/gosvc --version 1.1.0
gosvc release check --version 1.1.0
gosvc release snapshot --version 1.1.0 --repository ailtonmacedo/gosvc --parallel 3
gosvc release verify --dist dist
```

Publication readiness result:

```text
passed=7 warnings=1 failed=0
```

The single warning is expected: the local publication copy intentionally has no `.git` directory, so no external Git state was mutated during the rehearsal.

The final release candidate contains all six platform archives and passed host-binary execution during offline verification.

# E2E Fix v11 — Safe project destination and Git-independent first verification

## Problem observed

A project created with `gosvc new catalog-service` while the current directory was the
`gosvc` source repository was written to `gosvc/catalog-service`. The generated
`Makefile` then executed Git drift checks against the parent `gosvc` worktree, so
untracked `catalog-service/go.mod` / `go.sum` caused `make verify` to fail even though
the generated project itself was valid.

The same log also showed `go install ...@v1.1.0` returning `unknown revision` because the
`v1.1.0` tag had not yet been published.

## Changes

### 1. Safe default output for `gosvc new`

If the current directory is anywhere inside the source checkout whose `go.mod` declares:

```text
module github.com/ailtonmacedo/gosvc
```

and no `--output` is supplied, `gosvc new NAME` resolves the destination beside the
framework repository.

From:

```text
/work/framework-go/gosvc
```

this command:

```bash
gosvc new catalog-service --module github.com/acme/catalog-service --preset postgres-api
```

creates:

```text
/work/framework-go/catalog-service
```

instead of:

```text
/work/framework-go/gosvc/catalog-service
```

Outside the gosvc source checkout the historical default remains `./NAME`. An explicit
`--output` always wins.

### 2. Generated Makefile no longer trusts a parent Git repository

`tidy-check` and `generate-check` now enforce Git drift only when:

```bash
git rev-parse --show-toplevel
```

matches the generated project's own physical working directory. Before `git init`, or
when the project sits inside some unrelated parent worktree, the Git-specific drift step
is skipped with an explicit message. `go mod tidy` and code generation still run.

In CI, where the generated project is checked out as its own repository root, the strict
Git drift checks remain active.

### 3. Installation documentation

Before a tag exists:

```bash
go install github.com/ailtonmacedo/gosvc/cmd/gosvc@main
```

After `v1.1.0` is published:

```bash
go install github.com/ailtonmacedo/gosvc/cmd/gosvc@v1.1.0
```

## Compatibility

No CLI flag was removed or renamed. `--output` behavior is unchanged when explicitly
provided. Projects created from normal workspaces keep the old `./<project-name>`
default.

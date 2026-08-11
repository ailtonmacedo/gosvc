# Sprint 8 — Upgrade lifecycle and plugin contract

## Objective

Establish the `v1.0` lifecycle foundation so projects created by older releases
can be upgraded explicitly and extension metadata can be validated safely.

## Delivered

### Manifest schema v2

The manifest now records:

- framework version;
- project name and module;
- `project.yaml` schema version;
- selected preset and features;
- plugin references;
- upgrade history;
- managed files, ownership, and SHA-256 checksums.

Schema-v1 manifests remain readable through `manifest.LoadDocument`, but normal
regeneration rejects them and instructs the user to run `gosvc upgrade`. This
prevents silent metadata migrations.

### Upgrade command

```bash
gosvc upgrade --project . --to 1.0.0 --dry-run
gosvc upgrade --project . --to 1.0.0
```

The command:

1. loads and validates `project.yaml`;
2. reads legacy or current manifest schemas;
3. rejects downgrades;
4. renders the target release artifacts;
5. produces the normal ownership-aware change plan;
6. preserves user-owned files;
7. applies updates through an atomic directory swap;
8. records the transition only after success.

Running the same upgrade again returns `Project is already up to date` and does
not append another history record.

### Plugin metadata contract

A stable `Plugin` interface and metadata schema were added. Plugin manifests
are discovered under:

```text
.gosvc/plugins/<name>/plugin.json
```

Commands:

```bash
gosvc plugins list --project .
gosvc plugins validate --project .
```

Validation covers:

- schema version;
- lowercase kebab-case names;
- semantic versions;
- minimum gosvc version;
- supported and non-duplicated capabilities;
- duplicate plugin names;
- relative entrypoints that cannot escape the plugin directory;
- unknown JSON fields.

External plugin entrypoints are not executed in Sprint 8.

### Stable schemas

Added:

```text
schema/manifest.schema.json
schema/plugin.schema.json
```

Operational documentation:

```text
docs/UPGRADES.md
docs/PLUGINS.md
```

### Validation hardening

`gosvc validate` now cross-checks the manifest project name, module, and
configuration schema against `project.yaml`.

## Real migration validation

A copy of the Sprint 7 `event-driven-api` demo was upgraded from:

```text
framework: dev -> 1.0.0
manifest schema: 1 -> 2
```

The existing README was identified as user-owned and protected. All managed
artifacts remained unchanged, the upgraded project passed `gosvc validate`, and
a second upgrade was idempotent.

## Quality gates

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
```

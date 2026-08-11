# Sprint 9 — Executable Plugin Protocol

## Goal

Complete the `v1.0` extension boundary by allowing reviewed external plugins to
validate projects and contribute artifacts without linking plugin code into the
`gosvc` process.

## Delivered

### Plugin manifest v2

- `schema_version: 2`;
- `protocol_version: 1`;
- SHA-256 executable checksum;
- strict unknown-field rejection;
- compatibility checks against the running `gosvc` version;
- legacy schema-v1 discovery without execution.

### Controlled execution

- JSON request on stdin;
- JSON response on stdout;
- separate `validate` and `contribute` actions;
- reduced environment;
- configurable timeout with a maximum CLI value of one minute;
- 4 MiB stdout/stderr cap;
- non-zero exit handling;
- strict response decoding;
- diagnostic validation;
- copied plugin workspace;
- copied project snapshot, excluding `.git` and installed plugin binaries.

The real project path is not supplied to the plugin. Writes made through
`GOSVC_PROJECT_DIR` affect only the temporary snapshot.

### Artifact lifecycle

- clean relative path validation;
- `.gosvc/` and `.git/` protection;
- core and cross-plugin collision detection;
- untracked-file collision detection;
- immutable producer identity;
- immutable ownership for existing plugin artifacts;
- `generated` and `user` ownership semantics;
- atomic staging and directory swap;
- dry-run plans;
- idempotent reexecution;
- plugin file preservation during normal project regeneration.

### Manifest integration

Plugin references record:

- name;
- version;
- source manifest;
- executable checksum;
- protocol version.

File records may contain a `producer` field. Plugin features are stored as:

```text
plugin:<plugin-name>:<feature>
```

### CLI

```bash
gosvc plugins list --project .
gosvc plugins validate --project .
gosvc plugins checksum <entrypoint>
gosvc plugins run <name> --project . --dry-run
gosvc plugins run <name> --project . --timeout 10s
gosvc plugins run <name> --project . --force
```

`--force` may replace only modified `generated` artifacts belonging to that
same plugin. It never overwrites `user` artifacts.

### Validation

`gosvc validate` now checks:

- plugin source paths remain inside the project;
- plugin identity matches the installed reference;
- version, checksum, and protocol match;
- executable checksum still matches `plugin.json`;
- each produced file refers to an installed producer;
- plugin feature namespaces have an installed owner.

### Machine-readable schemas

```text
schema/plugin.schema.json
schema/plugin-request.schema.json
schema/plugin-response.schema.json
schema/manifest.schema.json
```

## Tests

The suite covers:

- protocol execution;
- validation and contribution actions;
- checksum mismatch;
- timeout;
- error diagnostics;
- project snapshot isolation;
- dry-run;
- atomic apply;
- idempotency;
- core collision rejection;
- producer preservation;
- core regeneration after plugin application;
- manual modification of generated plugin artifacts;
- CLI option ordering and end-to-end execution.

## Security boundary

The workspace design prevents ordinary plugin code from modifying the real
project through the paths supplied by `gosvc`. It does not provide kernel-level
isolation. A process running under the current user may access other filesystem
locations allowed by the OS. Only trusted plugins should be executed unless an
external sandbox is used.

## Deferred

- custom plugin commands;
- network denial;
- signed publisher identities;
- remote plugin registry;
- platform-specific sandboxing;
- plugin uninstall and rollback commands.

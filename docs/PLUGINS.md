# External plugin protocol

`gosvc` executes external plugins through a versioned JSON protocol. Plugins live under:

```text
.gosvc/plugins/<plugin-name>/
└── plugin.json
```

Native plugins also contain their entrypoint; Docker plugins reference an immutable image digest.

## Schema v3 execution modes

### Native

Backward-compatible trusted executable:

```json
{
  "schema_version": 3,
  "protocol_version": 1,
  "name": "audit",
  "version": "1.0.0",
  "description": "Adds audit validation",
  "minimum_gosvc_version": "1.1.0",
  "capabilities": ["validation", "artifacts"],
  "entrypoint": "bin/audit",
  "checksum": "sha256:<64-hex>",
  "execution": {"mode": "native"}
}
```

Schema v2 plugins without `execution` remain executable as native plugins for compatibility. Schema v1 remains metadata-only legacy.

### Docker sandbox

```json
{
  "schema_version": 3,
  "protocol_version": 1,
  "name": "audit",
  "version": "1.0.0",
  "description": "Sandboxed audit validation",
  "minimum_gosvc_version": "1.1.0",
  "capabilities": ["validation", "artifacts"],
  "execution": {
    "mode": "docker",
    "docker": {
      "image": "ghcr.io/acme/gosvc-audit@sha256:<64-hex>",
      "command": ["/plugin"],
      "network": false
    }
  }
}
```

Docker images must be pinned by digest. `entrypoint`/`checksum` belong to native execution and must not be declared for Docker plugins.

## Security policy

```bash
gosvc plugins validate --project .
gosvc plugins run audit --project . --dry-run
```

Require a sandboxed plugin:

```bash
gosvc plugins run audit --project . --require-sandbox --dry-run
```

Network is denied by default. If a Docker plugin explicitly declares network access, the operator must also opt in:

```bash
gosvc plugins run audit --project . --require-sandbox --allow-network
```

See `docs/PLUGIN_SECURITY.md` for the exact Docker restrictions and remaining threat model. A ready-to-edit Docker manifest is in `examples/plugins/sandboxed-audit/plugin.json`; replace its placeholder digest with the immutable digest of the real plugin image.

## Capabilities

| Capability | Meaning |
|---|---|
| `validation` | receives `validate` and returns diagnostics |
| `artifacts` | receives `contribute` and returns features/files |
| `commands` | reserved metadata for a future custom-command protocol |

## Protocol request

Each invocation receives one JSON document on stdin. A complete copyable example is also available at `examples/plugins/protocol-request.json`:

```json
{
  "protocol_version": 1,
  "action": "validate",
  "plugin": {"name": "audit", "version": "1.0.0"},
  "project": {
    "root": "/workspace/project",
    "name": "order-service",
    "module": "github.com/acme/order-service",
    "preset": "production-api",
    "features": ["base", "security"],
    "dry_run": true
  }
}
```

For native execution, `root` points to the temporary snapshot. For Docker execution, it is `/workspace/project`, mounted read-only.

Environment contract:

```text
GOSVC_PLUGIN_PROTOCOL=1
GOSVC_PLUGIN_ACTION=validate|contribute
GOSVC_PROJECT_DIR=<snapshot>
```

## Protocol response

A complete example is available at `examples/plugins/protocol-response.json`. The stdout contract is:

```json
{
  "protocol_version": 1,
  "diagnostics": [
    {"severity": "info", "path": "project.yaml", "message": "audit configuration is valid"}
  ],
  "contribution": {
    "features": ["trail"],
    "artifacts": [
      {
        "path": "internal/audit/audit.gen.go",
        "content": "package audit\n",
        "mode": 420,
        "ownership": "generated"
      }
    ]
  }
}
```

Only returned artifacts can be applied to the real project. Direct writes inside a native snapshot or Docker read-only mount are not the artifact API.

## Artifact rules

Artifacts must use clean relative paths and cannot target `.gosvc/` or `.git/`, collide with core/other-plugin ownership, claim existing untracked files, or change an existing ownership class. Application remains atomic through the generator plan/apply path.

## Machine-readable contracts

```text
schema/plugin.schema.json
schema/plugin-request.schema.json
schema/plugin-response.schema.json
schema/manifest.schema.json
```

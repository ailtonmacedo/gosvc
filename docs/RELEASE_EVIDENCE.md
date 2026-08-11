# Release Evidence

Every `gosvc release snapshot` now produces two human- and machine-readable publication records:

- `RELEASE_NOTES.md`, extracted from the exact `CHANGELOG.md` section for the release version;
- `release-evidence.json`, a deterministic summary of identity, acceptance results, snapshot gates, and reproducibility intent.

## Evidence contents

`release-evidence.json` records:

- release version, module, repository, commit, build timestamp, and Go builder version;
- the result of all built-in preset acceptance checks;
- stable file, resource, and architecture counts per preset;
- the deterministic gates executed by `release snapshot`;
- `reproducible: true` for builds driven by `SOURCE_DATE_EPOCH`.

Volatile acceptance values such as runtime duration and temporary workspace paths are deliberately excluded. This keeps the evidence file reproducible when the same source, version, and `SOURCE_DATE_EPOCH` are used.

## Verification

```bash
gosvc release verify --dist dist
```

The verifier rejects a release when:

- release notes do not match the requested version;
- evidence identity differs from `release-manifest.json`;
- any preset failed acceptance;
- snapshot gates are absent;
- a required platform archive or supporting asset is missing;
- checksums or sizes differ.

The public JSON contract is available at `schema/release-evidence.schema.json`.

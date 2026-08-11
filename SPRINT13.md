# Sprint 13 — Release Evidence and Parallel Packaging

## Goal

Close the last local publication gap for `v1.0.0`: produce all six platform archives efficiently and attach deterministic, auditable release evidence.

## Delivered

- Bounded parallel cross-platform compilation with `--parallel`.
- Default concurrency capped at three workers.
- Cancellation of remaining builds after the first target failure.
- Deterministic release notes extracted from the matching changelog section.
- Deterministic `release-evidence.json` with stable acceptance results.
- Strict verification of all six platform archives and supporting assets.
- Evidence identity cross-checked against `release-manifest.json`.
- Release workflow publishing `RELEASE_NOTES.md` through `--notes-file`.
- Public release-evidence JSON Schema and operational documentation.

## Release assets added

```text
RELEASE_NOTES.md
release-evidence.json
```

The evidence intentionally excludes temporary paths, runtime durations, and other volatile acceptance fields.

## Commands

```bash
SOURCE_DATE_EPOCH="$(git show -s --format=%ct HEAD)" \
  gosvc release snapshot \
  --version 1.0.0 \
  --repository ailtonmacedo/gosvc \
  --output dist \
  --parallel 3

gosvc release verify --dist dist
```

## Quality gates

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
```

## Results in this environment

The six-platform snapshot was generated and verified successfully.

```text
Sequential build (--parallel 1): 5.68s
Parallel build   (--parallel 3): 2.57s
Speedup:                         2.21x
Elapsed-time reduction:         54.8%
```

Two complete snapshots created with the same `SOURCE_DATE_EPOCH` produced identical SHA-256 hashes for every file, including archives, notes, evidence, SBOM, installers, and package-manager manifests.

The POSIX installer was also exercised against a local HTTP mirror and the installed binary reported:

```text
gosvc version 1.0.0
commit: unknown
built: 2023-11-14T22:13:20Z
```

The publication candidate uses `acme/gosvc` for rendered download metadata while the source module remains the local placeholder `github.com/ailtonmacedo/gosvc`. An official release must run `gosvc release prepare` first, removing the need for `--allow-placeholder`.

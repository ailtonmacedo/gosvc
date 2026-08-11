# Release Process

## 1. Prepare the repository identity

Preview the module and import rewrite:

```bash
gosvc release prepare \
  --project . \
  --repository ailtonmacedo/gosvc \
  --dry-run
```

Apply it once the final GitHub repository is known:

```bash
gosvc release prepare \
  --project . \
  --repository ailtonmacedo/gosvc
```

The operation updates the Go module, internal imports, Makefile references, workflows, and documentation through a staged transactional rewrite. Directories such as `.git`, `dist`, `bin`, and `vendor` are excluded.

## 2. Preconditions

- update `CHANGELOG.md` and version links;
- run `make verify`, `make acceptance`, and targeted race tests;
- ensure the Git remote matches the prepared repository;
- ensure the worktree is clean;
- create a signed or protected tag `vX.Y.Z`.

## 3. Acceptance gate

```bash
make acceptance
```

The command generates every built-in preset, adds a representative UUID resource to database-backed presets, validates architecture and tracked artifacts, and requires idempotent regeneration. The same matrix runs automatically inside `gosvc release check`.

For an auditable CI artifact:

```bash
gosvc acceptance --json > acceptance-report.json
```

## 4. Local preflight

```bash
gosvc release check \
  --project . \
  --repository ailtonmacedo/gosvc \
  --version 1.0.0
```

For a local release candidate before preparation only:

```bash
gosvc release check \
  --project . \
  --repository ailtonmacedo/gosvc \
  --version 1.0.0 \
  --allow-placeholder
```

## 5. Build a deterministic snapshot

```bash
SOURCE_DATE_EPOCH="$(git show -s --format=%ct HEAD)" \
  gosvc release snapshot \
  --project . \
  --repository ailtonmacedo/gosvc \
  --version 1.0.0 \
  --output dist \
  --parallel 3
```

The snapshot contains six platform archives, shell completions, rendered installer scripts, SHA-256 checksums, an SPDX 2.3 SBOM, `release-manifest.json`, `release-evidence.json`, version-specific `RELEASE_NOTES.md`, `gosvc.rb`, and `gosvc.json`.

Cross-platform binaries are built concurrently. The default concurrency is capped at three; use `--parallel 1` for constrained machines.

## 6. Verify without network access

```bash
gosvc release verify --dist dist
```

This verifies every digest and size, inspects all archives, validates Homebrew and Scoop metadata, checks release notes and acceptance evidence, checks the rendered installers, and executes the matching host-platform binary.

To skip execution when cross-validating on an unsupported host:

```bash
gosvc release verify --dist dist --skip-exec
```

## 7. Installer smoke test against a local mirror

```bash
python3 -m http.server 18080 --directory dist &
SERVER_PID=$!
trap 'kill "$SERVER_PID"' EXIT

GOSVC_RELEASE_BASE_URL=http://127.0.0.1:18080 \
GOSVC_INSTALL_DIR="$PWD/tmp-bin" \
sh dist/install.sh 1.0.0

./tmp-bin/gosvc version
```

## 8. GitHub release

Pushing a semantic version tag triggers `.github/workflows/release.yml`. The workflow runs quality gates, builds and verifies the snapshot, smoke-tests the installer, generates attestations, and creates the release.

Verify downloaded artifacts with:

```bash
gh attestation verify gosvc_1.0.0_linux_amd64.tar.gz --repo ailtonmacedo/gosvc
```


## GitHub publication plan

After `release prepare`, run the read-only GitHub publication rehearsal before creating the public repository tag:

```bash
gosvc release github-plan \
  --repository ailtonmacedo/gosvc \
  --version 1.1.0
```

The plan verifies module/repository identity, Git origin when present, changelog coverage, governance documents, and CI/acceptance/certification/release workflows. Use `--json` to archive the plan as release evidence. See `GITHUB_PUBLISHING.md`.

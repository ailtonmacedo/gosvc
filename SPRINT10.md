# Sprint 10 — Release and Distribution

## Goal

Close the `v1.0` lifecycle with reproducible release assets, installation paths, supply-chain metadata, repository governance, and publication automation.

## Delivered

### CLI

- `gosvc completion <bash|zsh|fish|powershell>`.
- `gosvc release check`.
- `gosvc release snapshot`.
- Semantic version validation and placeholder-module blocking.

### Release artifacts

The snapshot builder cross-compiles with `CGO_ENABLED=0` for:

- Linux amd64 and arm64;
- macOS amd64 and arm64;
- Windows amd64 and arm64.

Each archive contains the binary, license, README, changelog, and all completion scripts. Archives use fixed timestamps when `SOURCE_DATE_EPOCH` is set.

Additional assets:

- `checksums.txt`;
- `release-manifest.json`;
- SPDX 2.3 SBOM;
- Bash, Zsh, Fish, and PowerShell completions;
- POSIX and PowerShell installers.

### Supply chain

The GitHub release workflow:

1. verifies and tests the repository;
2. runs release preflight;
3. builds deterministic archives;
4. generates provenance attestations with `actions/attest`;
5. attaches the SPDX SBOM as an attestation;
6. publishes all assets through the GitHub CLI.

### Repository readiness

Added:

- CI workflow;
- Dependabot configuration;
- issue and pull request templates;
- changelog;
- contributing guide;
- security policy;
- code of conduct;
- installation and release documentation.

## Publication blocker

The source module remains `github.com/ailtonmacedo/gosvc` because no final repository path was provided. Public release preflight fails until the module and `ailtonmacedo/gosvc` links are replaced. Local release-candidate snapshots can be built with `--allow-placeholder`.

## Acceptance

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
make build VERSION=1.0.0
go run ./cmd/gosvc release check --version 1.0.0 --allow-placeholder
go run ./cmd/gosvc release snapshot --version 1.0.0 --output dist --allow-placeholder
```

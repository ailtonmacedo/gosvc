# GitHub Publication

This guide covers the final handoff from a locally certified `gosvc` source tree to the public GitHub repository.

## 1. Prepare repository identity

Replace the placeholder module and repository references before the first public tag:

```bash
gosvc release prepare \
  --project . \
  --repository ailtonmacedo/gosvc \
  --dry-run

gosvc release prepare \
  --project . \
  --repository ailtonmacedo/gosvc
```

The operation is transactional and updates the Go module, internal imports, documentation, workflows, schemas, and release links.

## 2. Generate the publication plan

```bash
gosvc release github-plan \
  --project . \
  --repository ailtonmacedo/gosvc \
  --version 1.1.0
```

For CI or machine inspection:

```bash
gosvc release github-plan \
  --project . \
  --repository ailtonmacedo/gosvc \
  --version 1.1.0 \
  --json > github-publication-plan.json
```

The command checks:

- module path matches `github.com/ailtonmacedo/gosvc`;
- Git origin, when configured, points to the same repository;
- the current branch is reported;
- CI, acceptance, certification, and release workflows are present and contain their required gates;
- the requested version exists in `CHANGELOG.md`;
- `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, and `LICENSE` are present.

Warnings do not block publication. Failures do.

## 3. Create or connect the GitHub repository

The publication plan prints the exact Git commands required. A typical first publication is:

```bash
git init
git branch -M main
git remote add origin https://github.com/ailtonmacedo/gosvc.git
git add .
git commit -m "chore: prepare gosvc v1.1.0"
git push -u origin main
```

Repository creation itself remains an explicit GitHub action. The generator does not silently create public repositories or push code.

## 4. Run the real certification workflow

The repository includes `.github/workflows/certification.yml` with `workflow_dispatch` support.

With GitHub CLI installed:

```bash
gh workflow run certification.yml --ref main
```

Do not create the release tag until real certification has completed successfully.

The workflow requires Go 1.25 and executes the real external toolchain, Docker-backed databases, Redis, Kafka, generated OpenAPI/sqlc code, migrations, HTTP CRUD, auth, Outbox, and vulnerability/lint gates.

## 5. Branch protection recommendation

Protect `main` and require pull requests. Recommended required checks are the jobs from:

- `.github/workflows/ci.yml`;
- `.github/workflows/acceptance.yml`;
- `.github/workflows/certification.yml`.

Also recommended:

- prevent force pushes to `main`;
- prevent branch deletion;
- require conversation resolution;
- require the branch to be up to date before merge;
- restrict release publishing to the release workflow.

## 6. Publish the tag

After main and real certification are green:

```bash
git tag -a v1.1.0 -m "gosvc v1.1.0"
git push origin v1.1.0
```

The release workflow then performs:

1. quality gates;
2. release preflight;
3. real integration certification;
4. deterministic cross-platform build;
5. offline release verification;
6. installer smoke test;
7. provenance/SBOM attestations;
8. GitHub Release publication.

## 7. Release verification

After downloading release assets:

```bash
gosvc release verify --dist ./dist
```

The verifier checks required archives, metadata, checksums, release evidence, package-manager manifests, and the host binary where applicable.

## Security boundary

`gosvc release github-plan` is read-only. It never creates a GitHub repository, changes branch protection, pushes commits, creates tags, or publishes a release. Those consequential actions remain explicit and reviewable.

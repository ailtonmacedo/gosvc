# Documentation Fix v15.1 — Quickstart Mode Separation

## Scope

Documentation-only revision over the E2E v15 code baseline. No generator/runtime behavior changed.

## Problem

The v15 Quickstart mixed source-checkout commands (`./bin/gosvc`) with installed-CLI commands (`gosvc`) after changing into a generated sibling project. It also showed project-level `doctor` before `make bootstrap`, which could make first-run diagnostics less clear.

## Fix

- Split Quickstart into two explicit modes:
  - installed CLI: `gosvc ...`;
  - source checkout: `./bin/gosvc ...`, then `../gosvc/bin/gosvc ...` from a sibling generated project.
- Reordered onboarding to `doctor --preset → new → bootstrap → doctor --project → infra/migrate → verify → run`.
- Added interactive tabs to the standalone HTML guide.
- Updated README, installation, command cookbook, troubleshooting, and changelog guidance.
- Kept the E2E baseline at v15 because this revision changes documentation only.

## Rule

Never assume that running `gosvc` from inside the repository selects `./bin/gosvc`; shell command resolution still follows `PATH`. Always use the exact binary form appropriate to the selected mode.

# Changelog

## 2.0.0 — 2026-07-27

IntentCI v2 is an intentionally incompatible, MacBook-first rewrite.

### Added

- One strict `.intentci.yaml` file containing intent, path globs, and commands
- Changed-file selection for staged, unstaged, renamed, deleted, and untracked files
- Sequential fail-fast execution through login `zsh`
- Stack-detecting `init`, `--all`, `version`, and compact help
- Apple Silicon macOS release and smoke validation

### Removed

- Markdown requirements, compiler, IR, providers, evidence, reports, cache,
  scheduler, repair agents, runtime schemas, and configuration overlays
- Linux and Intel Mac releases
- Every direct dependency except YAML and doublestar glob matching

See [the v1 migration guide](docs/migration-v1-to-v2.md). The complete v1
history remains preserved by its immutable tags and GitHub releases.

## 1.1.1 — 2026-07-27

Final v1 release. It corrected missing-confidence handling for probabilistic
evidence and completed the v1 release-validation suite.

## 1.1.0 — 2026-07-27

First release validated against the complete v1 contract.

## 1.0.0

Breaking replacement of the v0 Product Contract model with Markdown
requirements, providers, evidence, and bounded repair.

## 0.4.0

Final v0 release.

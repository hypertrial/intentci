# Changelog

## 2.0.2 — 2026-07-27

### Fixed

- The release asset-validation job now installs Go before running the generated
  Go smoke check, completing end-to-end publication validation.

## 2.0.1 — 2026-07-27

### Fixed

- Login `zsh` now retains paths inherited from the invoking process, so checks
  can find tools installed by GitHub Actions, package managers, and parent
  shells even when a login profile rewrites `PATH`.

The archive passed checksum, contents, and version checks, but the separate
smoke job omitted Go itself. Use the fully validated v2.0.2 release.

## 2.0.0 — 2026-07-27

IntentCI v2 is an intentionally incompatible, MacBook-first rewrite.

The release archive was published successfully, but its packaged smoke
validation exposed the login-shell `PATH` bug fixed by v2.0.1. Use v2.0.1.

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

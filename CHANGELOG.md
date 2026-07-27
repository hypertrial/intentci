# Changelog

## 1.1.0 — 2026-07-27

First release validated against the complete normative v1 contract.

### Added

- Typed requirement, obligation, provider, retry, timeout, dependency,
  platform, evidence-class, and confidence metadata
- Deterministic canonical IR and verification plans with stable hashes
- Complete command, JUnit, SARIF, JSONPath-subset, boundary, git-diff, manual,
  and external-provider v1 behavior
- Provenance-complete Git repository state, impact selection, scheduler, cache,
  evidence, reporting, and manifest models
- Immutable multi-attempt repair runs with pinned contracts, boundary and
  protected-path enforcement, repeated-work detection, and independent
  re-verification
- Explicit configuration precedence and documented `INTENTCI_*` overrides
- `verify --head`, `--provider`, `--max-parallel`, `--fail-fast`, and
  `--no-git`; named repair-agent discovery
- Tracked Go, Python, TypeScript, Rust, and Java examples and integration
  fixtures
- Linux/macOS release validation, 20 executable acceptance criteria,
  zero-survivor mutation checks, performance records, deterministic archives,
  checksums, and packaged-binary smoke tests

### Fixed

- False passes from stale generated reports, incomplete or informational
  evidence, low-confidence probabilistic evidence, cancellation, and stale
  cache keys
- Invalid status/priority acceptance, unsafe paths and symlinks, unmapped
  change handling, verifier exit mapping, and repair attempt semantics
- Per-attempt repository state and binary diff persistence in repair evidence
  bundles

### Compatibility

Valid v1.0.x requirements and configuration remain accepted. Invalid or
ambiguous inputs that v1.0.x silently accepted now fail with actionable
diagnostics. See
[docs/migration-v1.0-to-v1.1.md](docs/migration-v1.0-to-v1.1.md).

## 1.0.0 — Breaking rewrite

IntentCI v1 replaces the v0.x Product Contract model.

### Added

- Markdown requirements with YAML front matter and obligation providers
- Canonical Intent IR (`intentci compile`)
- Providers: command, junit, sarif, boundary, git-diff, json, manual
- Evidence bundles under `.intentci/runs/`
- Bounded `intentci repair` with repair packets and protected-path checks
- `status`, `doctor`, `schema` commands
- Exit codes `0`–`10` per v1 specification
- Apache License 2.0

### Removed

- `.intentci/contract.yaml` Product Contracts
- Change Specs, waivers, hooks, `--attest`, `--trust`, `policy.semantic`
- Exit codes `10/11/12/20/21/30` (v0 meanings)

See [docs/migration-v0-to-v1.md](docs/migration-v0-to-v1.md).

## 0.4.0

Semantic verification overlay on Product Contracts (final v0 line).

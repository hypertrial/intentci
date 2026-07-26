# Changelog

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

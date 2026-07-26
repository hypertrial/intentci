# Migrating from IntentCI v0.x to v1.0.0

**Breaking release.** v1.0.0 replaces the Product Contract model. There is no dual-read of `.intentci/contract.yaml`.

## Decision

| Line | License | Model |
| --- | --- | --- |
| v0.4.x (last) | MIT | Product Contract YAML + Change Specs |
| v1.0.0+ | Apache-2.0 | Markdown requirements + obligations + providers |

v0.4 binaries remain the last Product Contract line. Upgrade requires rewriting repository configuration.

## Artifact mapping

| v0.x | v1.0 |
| --- | --- |
| `.intentci/contract.yaml` | `.intentci/config.yaml` + `.intentci/requirements/**/*.md` |
| `requirements[].status: approved` | front matter `status: active` |
| `severity: blocking` | `priority: required` |
| `severity: advisory` | `priority: recommended` or `informational` |
| `verification.checks: [id]` | obligation `verify` expressions with `provider: command` |
| top-level `checks:` | inline provider configs on obligations |
| `.intentci/changes/*.yaml` Change Specs | temporary/change-scoped requirements as Markdown (or omit) |
| `policy.semantic` | deferred; use provider evidence classes / custom providers post-v1 |
| `intentci validate` | `intentci compile` |
| `intentci check` | `intentci verify --changed` (fast path via profiles/timeouts in config) |
| `intentci hook` / `--attest` | removed (use CI + evidence bundles) |
| `.intentci/tmp/last-result.json` | `.intentci/runs/<run-id>/` evidence bundles |

## Status / verdict vocabulary

| v0.x | v1.0 |
| --- | --- |
| `pass` | `pass` |
| `fail` | `fail` |
| `unverified` | `unproven` |
| `unknown` | `uncertain` or `error` (context-dependent) |
| `waived` | not first-class in v1 (use disabled requirement or manual review) |
| `not_affected` | skipped / not selected by impact |
| — | `review_required` (new) |
| — | `skipped` (obligation-level) |

## Exit codes

| Meaning | v0.x | v1.0 |
| --- | --- | --- |
| Pass | `0` | `0` |
| Failed requirement | `10` | `1` |
| Unproven / unverified | `11` | `2` |
| Uncertain / unknown | `12` | `3` |
| Review required | — | `4` |
| Invalid config / compile | `20` | `5` |
| Verifier execution error | — | `6` |
| Missing prerequisite | `21` | `6` or `8` (context) |
| Internal error | `30` | `7` |
| Invalid CLI usage | `1` (generic) | `8` |
| Repair exhausted | — | `9` |
| Security / boundary | — | `10` |

Update CI scripts that branch on exit codes before upgrading.

## Suggested migration steps

1. Install IntentCI `v1.0.0`.
2. Back up `.intentci/`.
3. Run `intentci init --force` in a clean branch (or hand-author `config.yaml`).
4. For each approved blocking requirement, create a Markdown file under `.intentci/requirements/` with Intent, obligations, and `provider: command` mappings to existing tests.
5. Convert path includes to `applies_to.paths` and boundary rules to `provider: boundary` obligations where needed.
6. Delete `contract.yaml`, `changes/`, and v0 hook sections.
7. Run `intentci compile --strict` then `intentci verify --all`.
8. Point CI at `intentci verify --changed` (or `--all`) and the new exit-code table.

## Removed features

- Change Specs and waivers
- Managed git pre-push hooks
- `--attest` attestations
- `--trust` trusted-repos file (v1 treats local command execution as an explicit operator choice; protect via CI and repair immutability)
- `policy.semantic` local/HTTP overlay

# IntentCI

**CI for product intent.**

IntentCI is a local-first verification system that determines whether a code change satisfies a repository’s approved product requirements. It organizes existing tests and checks around persistent product intent and reports requirement-level evidence before you push.

```text
STATE-001   Failed writes must not advance state       PASS
RETRY-002   Retries must not duplicate records         PASS
API-004     Existing configuration remains compatible  UNVERIFIED
ARCH-003    Core logic remains destination-neutral     FAIL
```

IntentCI does not replace unit tests, linters, or remote CI. It answers:

> Which approved product requirements are affected by this change, and what evidence demonstrates that each one still holds?

## Status

**v0.4.0** — Semantic verification: optional local/HTTP providers, structured findings with evidence citations, deterministic failures outrank semantic assessments, and `verify --show-semantic-input`, with a CI-enforced 100% statement coverage gate. See [docs/v0.4.md](docs/v0.4.md), [docs/acceptance-v0.4.md](docs/acceptance-v0.4.md), [docs/roadmap.md](docs/roadmap.md), and [v1.md](v1.md).

## Install

### From release binaries

Download the binary for your platform from the [GitHub Releases](https://github.com/hypertrial/intentci/releases) page, make it executable, and place it on your `PATH`.

```bash
chmod +x intentci
sudo mv intentci /usr/local/bin/
intentci version
```

### From source

Requires Go 1.23+.

```bash
git clone https://github.com/hypertrial/intentci.git
cd intentci
go install ./cmd/intentci
```

## Quickstart (5 minutes)

1. Initialize IntentCI in a Git repository:

```bash
cd your-repo
intentci init
```

2. Edit `.intentci/contract.yaml`. Promote a draft requirement to `approved`, set `severity: blocking`, map `applies_to` paths, and point `verification.checks` at a real command (for example `go test ./...`).

3. Validate configuration:

```bash
intentci validate
```

4. Run the fast development profile or full verification:

```bash
intentci check
intentci verify --format json
```

Create and validate Change Specs, then verify with the change and without cache:

```bash
intentci change create DEMO-1
intentci validate
intentci verify --change DEMO-1 --no-cache --trust
intentci verify --attest --trust
intentci explain BUILD-001
intentci explain AC-001 --change DEMO-1
intentci hook install
```

5. Trust the repository on first run when prompted, or pass `--trust`.

If `policy.default_base` (usually `origin/main`) is missing locally, pass an explicit base:

```bash
intentci verify --base main --trust
```

Missing base references exit with code `21` rather than silently falling back.

## CLI

| Command | Purpose |
| --- | --- |
| `intentci init` | Create `.intentci/` with a starter Product Contract |
| `intentci validate` | Validate the Product Contract and Change Specs |
| `intentci change create <id>` | Create a draft Change Spec scaffold |
| `intentci explain <id>` | Explain a requirement or acceptance criterion |
| `intentci check` | Fast profile verification |
| `intentci verify` | Full profile verification |
| `intentci hook install` | Install managed pre-push hook (`verify --attest`) |
| `intentci hook uninstall` | Remove managed IntentCI hook section |
| `intentci version` | Print version |

Common flags for `check` / `verify`:

```text
--base <ref>              Comparison base (default: policy.default_base)
--all                     Verify all approved blocking requirements
--change <id>             Change Spec id (verify scoped ACs / affected requirements)
--format text|json        Output format
--output <path>           Write report to a file
--trust                   Trust this repository for local command execution
--no-cache                Disable the successful-check cache
--attest                  (verify only) Write PASS-only attestation under .intentci/tmp/
--show-semantic-input     (verify only) Print semantic provider request JSON; do not call provider
```

### Semantic verification (optional)

Semantic analysis is off by default. Enable it in `.intentci/contract.yaml`:

```yaml
policy:
  semantic:
    enabled: true
    enforcement: advisory   # or blocking
    confidence_threshold: 0.8
    provider:
      type: local           # or http
      command: ./tools/intentci-semantic
      # url: https://semantic.example/v1
      timeout: 2m
```

IntentCI posts a versioned JSON request to the provider and merges structured findings. HTTP credentials use `INTENTCI_SEMANTIC_TOKEN` only (never repository config). Inspect what would be sent (no checks, trust prompt, or provider call):

```bash
intentci verify --show-semantic-input
```

No repository content is sent over the network unless `provider.type: http` is configured. Provider credentials must use `INTENTCI_SEMANTIC_TOKEN`, not URL userinfo.

Git hooks are bypassable with `git push --no-verify` and are not organizational enforcement. Trust the repository once (`--trust` or interactive prompt) before relying on the pre-push hook, which runs `intentci verify --attest` without `--trust`.

CI runs `./scripts/check-coverage.sh`, which requires **100.0%** statement coverage across `./...`.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Passed |
| `10` | Blocking requirement failed |
| `11` | Blocking requirement unverified |
| `12` | Blocking requirement unknown |
| `20` | Invalid Product Contract or Change Spec |
| `21` | Missing prerequisite |
| `30` | Internal error |

Agents should consume JSON (`--format json`) rather than parsing terminal text.

## Privacy

IntentCI makes no network requests by default. It executes repository-defined check commands on the local machine. Treat untrusted repositories like untrusted build scripts — IntentCI warns before the first run unless `--trust` is set.

## Platform support

macOS and Linux are supported. Windows via WSL.

## License

[MIT](LICENSE)

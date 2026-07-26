# IntentCI

**Intent compiler and evidence-based verification for agent-generated code.**

IntentCI connects human intent to machine-verifiable evidence:

```text
Requirement → Obligation → Verifier → Evidence → Verdict → Repair
```

It organizes existing tests and checks around Markdown requirements and reports obligation-level evidence before you push.

```text
FAIL REQ-AUTH-001
  PASS OBL-001
  FAIL OBL-002
  PASS OBL-003
```

## Status

**v1.0.0** — Spec-complete rewrite: Markdown requirements, Intent IR, providers (command/junit/sarif/boundary/git-diff/json/manual), evidence bundles, bounded repair, exit codes `0`–`10`. See [docs/v1.md](docs/v1.md), [docs/acceptance-v1.md](docs/acceptance-v1.md), [v1.md](v1.md).

Breaking change from v0.x Product Contracts: [docs/migration-v0-to-v1.md](docs/migration-v0-to-v1.md).

## Install

### From release binaries

Download the binary from [GitHub Releases](https://github.com/hypertrial/intentci/releases), make it executable, and place it on your `PATH`.

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

## Quickstart

```bash
cd your-repo
intentci init
# edit .intentci/requirements/*.md
intentci compile --strict
intentci verify --changed
intentci explain REQ-001 --show-evidence
```

Repair with an external agent command:

```bash
intentci repair \
  --agent-command './scripts/run-agent.sh {packet}' \
  --max-attempts 3
```

## CLI

| Command | Purpose |
| --- | --- |
| `intentci init` | Create `.intentci/config.yaml` + example requirement |
| `intentci compile` | Compile Markdown → canonical Intent IR |
| `intentci verify` | Select, execute providers, emit verdicts |
| `intentci explain <id>` | Explain a requirement from the latest run |
| `intentci repair` | Bounded agent repair loop |
| `intentci status` | Repository status from latest run |
| `intentci doctor` | Local dependency / config checks |
| `intentci schema <name>` | Print JSON schemas |
| `intentci version` | Print version |

### Verify flags

```text
--all                     Verify all active requirements
--changed                 Verify requirements affected by the Git diff (default)
--requirement <id>        Single requirement
--obligation <id>         Single obligation
--base <ref>              Comparison base
--format text|json|junit  Output format
--output <path>           Write report to a file
--no-cache                Disable successful-provider cache
```

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Passed |
| `1` | Requirement failed |
| `2` | Unproven |
| `3` | Uncertain |
| `4` | Review required |
| `5` | Compile failed |
| `6` | Verifier execution error |
| `7` | Internal error |
| `8` | Invalid CLI usage |
| `9` | Repair attempts exhausted |
| `10` | Security / boundary violation |

Agents should consume JSON (`--format json`) rather than parsing terminal text.

## Privacy

Telemetry is off by default (`telemetry.enabled: false`). IntentCI executes repository-defined provider commands locally. HTTP is only used if you configure providers that perform it.

## Platform support

macOS and Linux. Windows via WSL.

## License

[Apache License 2.0](LICENSE)

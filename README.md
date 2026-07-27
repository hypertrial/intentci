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

**v1.1.0** is the first release validated against the complete normative
[`v1.md`](v1.md), including the executable
[§38 acceptance matrix](docs/acceptance-v1.md). The v1.0.x releases remain
available as immutable historical tags.

Breaking change from v0.x Product Contracts: [docs/migration-v0-to-v1.md](docs/migration-v0-to-v1.md).
Existing v1.0.x users: [v1.0.x → v1.1 migration](docs/migration-v1.0-to-v1.1.md).

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
go install github.com/hypertrial/intentci/cmd/intentci@v1.1.0
```

For development:

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
--changed                 Verify requirements affected by the Git diff (default; empty diff verifies nothing)
--requirement <id>        Single requirement
--obligation <id>         Single obligation
--provider <id>           Single verifier id
--base <ref>              Comparison base
--head <ref>              Comparison head
--max-parallel <n>        Override bounded concurrency
--fail-fast               Stop scheduling new work after a non-pass
--no-git                  Allow all/explicit verification without Git provenance
--format text|json|junit  Output format
--output <path>           Write report to a file
--no-cache                Disable successful-provider cache
```

Invalid formats and empty selectors use exit `8`.

### Repair agents

`--agent NAME` resolves `intentci-agent-NAME` on `PATH`. It is mutually
exclusive with `--agent-command`. `{packet}`, `{repository}`, and `{attempt}`
placeholders are expanded for command adapters.

`repair.max_attempts` is the total number of immutable verification attempts,
including the initial failed state. No agent runs after the last permitted
verification. IntentCI v1 executes agents on the host and is not a sandbox.

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

## Evidence

Each run under `.intentci/runs/<run-id>/` contains canonical IR, the selected
plan, initial and per-attempt repository state/diffs, evidence, verdicts, logs,
artifacts, terminal/JSON/JUnit reports, a manifest, and a final verdict that
records the manifest hash. Attempts and finalized runs are immutable.

See the [configuration reference](docs/configuration.md),
[provider protocol](docs/provider-protocol-v1.md), and
[security model](docs/SECURITY.md). The
[normative conformance index](docs/v1-conformance.md) maps every v1 section to
its executable release control.

## Privacy

Telemetry is off by default (`telemetry.enabled: false`). IntentCI executes
repository-defined providers and repair agents locally with a minimal
environment plus explicit allowlists. HTTP is only used by tools you configure.

## Platform support

macOS amd64/arm64 and Linux amd64. Windows via WSL.

## License

[Apache License 2.0](LICENSE)

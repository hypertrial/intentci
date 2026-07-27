# IntentCI

IntentCI runs the checks that matter for your current local changes.

```text
changed files → matching checks → zsh commands → pass/fail
```

Version 2 is intentionally small and incompatible with v1. The complete v1
implementation remains available from the immutable
[`v1.1.1`](https://github.com/hypertrial/intentci/releases/tag/v1.1.1)
release.

## Install

IntentCI v2 supports Apple Silicon Macs.
Source builds on other platforms are incidental and unsupported.

Download `intentci_2.0.4_darwin_arm64.tar.gz` from
[GitHub Releases](https://github.com/hypertrial/intentci/releases), or install
from source with Go 1.23 or newer:

```bash
go install github.com/hypertrial/intentci/v2/cmd/intentci@v2.0.4
```

## Start

From any directory inside a Git repository:

```bash
intentci init
```

`init` detects a root-level Go, Node, Python, Rust, Maven, or Gradle project and
creates `.intentci.yaml`. Edit it whenever the generated command or paths are
not the right ones for your repository.

```yaml
version: 2

checks:
  - id: go-tests
    intent: Go changes must keep tests passing.
    paths:
      - "**/*.go"
      - go.mod
      - go.sum
    run: go test ./...
```

Then use one command during normal development:

```bash
intentci
```

IntentCI takes the union of staged and unstaged tracked paths, then adds
deleted, renamed, and untracked non-ignored files. Commands run against the
current working tree in YAML order and stop at the first failure. A change to
`.intentci.yaml` runs every check.

Run everything explicitly with:

```bash
intentci --all
```

## Configuration

Every field is required:

- `version` must be `2`.
- `checks` must contain at least one check.
- `id` must be unique and match `[a-z0-9][a-z0-9-]*`.
- `intent` explains why the check exists.
- `paths` contains repository-relative
  [doublestar](https://github.com/bmatcuk/doublestar) globs. Use `**` for every
  changed file.
- `run` is a command passed to `/bin/zsh -lc`; YAML multiline strings work.

Unknown fields, absolute or traversing paths, backslashes, malformed globs, and
empty values are errors. There are no includes, overlays, retries, providers,
reports, caches, or environment overrides.

## Commands

```text
intentci             Run checks matching working-tree changes
intentci --all       Run every check
intentci init        Create .intentci.yaml without overwriting
intentci version     Print the version
intentci --help      Print usage
```

Exit codes are `0` for success or no matching work, `1` for a failed check, `2`
for usage/configuration/Git/launch errors, `130` for `SIGINT`, and `143` for
`SIGTERM`.

An interrupt stops repository discovery or the active check and never starts a
later check. IntentCI sends `SIGTERM` to the complete check process group,
allows one second for a graceful exit, then forcibly terminates anything still
running.

## Trust

IntentCI is not a sandbox. Commands run from the repository root with your full
environment and user permissions. Only run checks from repositories you trust.
No results are persisted and IntentCI does not access the network itself.

See [migration guidance](docs/migration-v1-to-v2.md) when moving from v1.

## License

[Apache License 2.0](LICENSE)

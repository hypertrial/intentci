# Migrating from IntentCI v1 to v2

Version 2 is intentionally incompatible with v1. It replaces requirements,
obligations, providers, evidence, reports, and repair with path-aware local
commands.

The immutable
[`v1.1.1`](https://github.com/hypertrial/intentci/releases/tag/v1.1.1)
release remains available for repositories that need the v1 contract:

```bash
go install github.com/hypertrial/intentci/cmd/intentci@v1.1.1
```

## Convert a command obligation

A v1 command obligation such as:

```yaml
applies_to:
  paths: ["**/*.go"]

verify:
  provider: command
  run: go test ./...
```

becomes:

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

Create `.intentci.yaml`, verify it with `intentci --all`, then remove the old
`.intentci/` directory. `intentci init` deliberately stops when it finds a v1
configuration so migration cannot happen silently.

There is no automatic conversion for JUnit, SARIF, JSON, manual, boundary,
git-diff, or external providers; dependency graphs; evidence history; reports;
caching; or repair agents. Keep v1.1.1 if those capabilities remain necessary.

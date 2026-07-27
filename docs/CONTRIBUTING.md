# Contributing

## Development

```bash
go test ./...
./scripts/check-coverage.sh
go test -race ./...
go build -o intentci ./cmd/intentci
./intentci compile --strict
./intentci verify --all --no-cache
./scripts/check_examples.sh
./scripts/validate_v1_release.sh
```

## Branching

Feature work lands through pull requests to protected `main`. Public tags are
immutable and reserved for verified releases.

## Style

- Prefer interfaces over package-level `var` hooks for testability.
- Keep provider contracts in `internal/provider`.
- Add behavior-focused tests for every change and keep the 100% statement gate.
- Update the executable §38 suite when an acceptance contract changes.

## License

Contributions are accepted under the Apache License 2.0.

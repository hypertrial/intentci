# Contributing

## Development

```bash
go test ./...
./scripts/check-coverage.sh
go build -o intentci ./cmd/intentci
./intentci compile --strict
./intentci verify --all --no-cache
```

## Branching

Feature work for v1 lands on `v1-rewrite` (or `main` after merge). Public tags are reserved for release versions (`v1.0.0`).

## Style

- Prefer interfaces over package-level `var` hooks for testability.
- Keep provider contracts in `internal/provider`.
- Update [docs/acceptance-v1.md](acceptance-v1.md) when completing acceptance items.

## License

Contributions are accepted under the Apache License 2.0.

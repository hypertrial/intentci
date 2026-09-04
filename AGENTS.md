<!-- agent-dev:begin -->
## Agent-dev worker contract

- It’s a Plan project key: `INT`.
- Repository purpose: IntentCI validates that code changes match their stated intent and repository policy.
- Architecture: Go CLI under `cmd/intentci` with internal packages under `internal`.
- Work only on the delegated issue. Avoid unrelated changes and all secrets.
- Do not create branches, commit, push, run `gh`, merge, or alter issue state. The supervisor owns those operations.
- Run relevant checks during development. The supervisor runs the complete gate in this order:
- `go test -race ./...`
- `go vet ./...`
- `go build -trimpath ./cmd/intentci`
- `go run ./cmd/intentci --all`
- Definition of done: focused implementation, relevant tests, no secret material, and a concise final report.
- Return a concise summary, changed files, validation performed, and blockers.
<!-- agent-dev:end -->

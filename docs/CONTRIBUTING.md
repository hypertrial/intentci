# Contributing

IntentCI v2 is deliberately narrow. Add behavior only when it is needed by the
local Apple Silicon Mac workflow.

Before opening a pull request:

```bash
go test -race ./...
go vet ./...
go build -trimpath ./cmd/intentci
go run ./cmd/intentci --all
```

Prefer the standard library, preserve strict input validation, and include one
focused test for non-trivial behavior. Feature work lands through pull requests
to protected `main`; published tags are immutable.

Contributions are accepted under the Apache License 2.0.

# v0.1.0 Acceptance Checklist

Complete before tagging `v0.1.0`.

## Automated

- [ ] `go test ./...` passes locally on macOS or Linux
- [ ] `go vet ./...` passes
- [ ] CI green on `ubuntu-latest` and `macos-latest`
- [ ] Cross-compile artifacts build for `linux/amd64`, `darwin/amd64`, `darwin/arm64`
- [ ] Fixture integration test covers pass and fail paths (`fixtures/go-service`)

## Manual

- [ ] `intentci init` in a fresh temp Git repo creates `.intentci/contract.yaml`
- [ ] Promoting a draft requirement to `approved` and running `intentci validate` succeeds
- [ ] `intentci verify --trust --all --format json` on this repository exits `0` on a clean tree (or correctly reports affected failures)
- [ ] Missing `policy.default_base` / `--base` ref exits `21` with a clear error (no silent fallback)
- [ ] JSON output includes `schema_version`, `requirements`, `summary`, and stable status strings
- [ ] First run without `--trust` prompts before executing checks
- [ ] README quickstart is accurate
- [ ] [docs/v0.1.md](v0.1.md) and [docs/roadmap.md](roadmap.md) match shipped CLI surface

## Release

- [ ] Version string via `intentci version` reflects `0.1.0` for release builds
- [ ] Tag `v0.1.0` created
- [ ] GitHub Release published with binaries and checksums
- [ ] No IntentCI-initiated network requests in default paths (code review)

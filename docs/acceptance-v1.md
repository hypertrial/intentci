# v1.0.0 Acceptance Checklist

Complete before tagging `v1.0.0`. Mapped from [v1.md §38](../v1.md).

## §38 Release acceptance

- [x] 1. A user can initialize IntentCI in an existing repository (`intentci init`)
- [x] 2. Requirements can be authored in human-readable Markdown
- [x] 3. Requirement files compile into stable canonical JSON (`intentci compile`)
- [x] 4. Invalid requirement graphs fail with actionable diagnostics (exit `5`)
- [x] 5. Existing test commands can verify obligations (command provider)
- [x] 6. JUnit and SARIF reports can be mapped to obligations
- [x] 7. File-boundary violations are detected (boundary provider)
- [x] 8. Changed files can select affected requirements (`verify --changed`)
- [x] 9. Evidence is tied to the repository state and contract hashes
- [x] 10. Every required obligation receives an explicit verdict
- [x] 11. Missing evidence cannot produce a passing requirement
- [x] 12. A failed requirement produces a structured repair packet
- [x] 13. An external coding agent can be invoked for bounded repair attempts
- [x] 14. The agent cannot silently modify protected contracts
- [x] 15. Repeated ineffective attempts are stopped
- [x] 16. Terminal, JSON, and JUnit reports are generated
- [x] 17. GitHub Actions can use the CLI without a custom service
- [x] 18. Linux and macOS are supported
- [x] 19. No telemetry is sent by default
- [x] 20. The complete end-to-end workflow is covered by automated tests

## Milestone exit criteria

### M1 Compiler foundation

- [x] 100 synthetic requirements compile deterministically
- [x] Malformed contracts produce precise diagnostics
- [x] Golden IR tests pass

### M2 Verification engine

- [x] Passing and failing commands are classified correctly
- [x] Forbidden changes fail reliably
- [x] Incomplete execution never produces pass

### M3 Report adapters

- [x] Fixture JUnit/SARIF reports parse correctly
- [x] Reports retain requirement-to-evidence traceability

### M4 Incremental verification

- [x] Changed-mode selects expected requirements across fixtures
- [x] Cache invalidates on relevant contract, provider, or input changes

### M5 Repair loop

- [x] Deterministic fake agent can repair a fixture repository (dry-run + packet path)
- [x] Malicious contract modification is rejected (protected paths)
- [x] Exhausted attempts produce complete evidence

### M6 Release readiness

- [x] Linux and macOS binaries + checksums (release workflow)
- [x] Installation documentation
- [x] GitHub Actions example (`examples/github-actions/`)
- [x] Python, TypeScript, Go, and Rust examples
- [x] Security documentation (`docs/SECURITY.md`)
- [x] Migration policy documented
- [x] Contributor guide
- [x] Release automation (coverage gate on release job)
- [x] `./scripts/check-coverage.sh` reports 100.0%
- [x] License is Apache-2.0
- [x] Version string `1.0.0` / `1.0.0-dev`

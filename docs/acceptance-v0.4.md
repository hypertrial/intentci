# v0.4.0 Acceptance Checklist

- [ ] `./scripts/check-coverage.sh` reports 100.0%
- [ ] CI enforces coverage on Linux and macOS
- [ ] Default / `policy.semantic.enabled: false` path makes no IntentCI-initiated network requests
- [ ] `enabled: true` without provider fails `validate` / does not silently call a provider
- [ ] Local provider returns structured findings merged into requirement results
- [ ] HTTP provider only contacts the configured URL; token from `INTENTCI_SEMANTIC_TOKEN` only
- [ ] `intentci verify --show-semantic-input` prints request JSON without `--trust`, without running checks, and without invoking a provider
- [ ] Deterministic check FAIL is never overridden by a positive semantic assessment
- [ ] Advisory mode never converts deterministic PASS → FAIL
- [ ] Blocking FAIL only when confidence, evidence, approved, and `semantic: required` all hold
- [ ] `verification.semantic: required` with unavailable provider yields requirement `unknown`
- [ ] Goals and non-goals from a Change Spec appear in semantic input
- [ ] `intentci explain` surfaces semantic findings from the last local result
- [ ] Contract weakenings include disabling required semantic / removing provider / softening enforcement
- [ ] Version string reflects `0.4.0-dev` (dev) / `0.4.0` (release builds)
- [ ] GitHub Release `v0.4.0` published

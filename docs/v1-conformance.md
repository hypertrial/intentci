# v1 normative conformance index

This index maps every normative section of repository-root
[`v1.md`](../v1.md) to an executable validation control. It complements the
20 product-level §38 acceptance criteria; it does not replace package-level
tests.

Status is derived from the named command or hosted job. The document contains
no manually asserted pass state.

| Control | v1.md scope | Executable validation |
| --- | --- | --- |
| V1-S04 | §4 product model and evidence principles | `go test ./internal/verdict ./internal/evidence` |
| V1-S05 | §5 user workflows | `go test ./tests/acceptance -run '^TestV1Acceptance$'` |
| V1-S06 | §6 repository layout | `go test ./internal/initcmd ./internal/evidence` |
| V1-S07 | §7 typed configuration and precedence | `go test ./internal/config` |
| V1-S08 | §8 Markdown requirement format | `go test ./internal/parser ./internal/compiler` |
| V1-S09 | §9 canonical Intent IR | `go test ./internal/compiler ./internal/ir` |
| V1-S10 | §10 obligation model | `go test ./internal/compiler ./internal/verdict` |
| V1-S11 | §11 verifier model and logical expressions | `go test ./internal/compiler ./internal/executor ./internal/verdict` |
| V1-S12 | §12 dependencies and execution graph | `go test ./internal/compiler ./internal/executor` |
| V1-S13 | §13 CLI surface and selector rules | `go test ./internal/cli` |
| V1-S14 | §14 compile command | `go test ./internal/compiler ./internal/cli` |
| V1-S15 | §15 verify command | `go test ./internal/verify ./internal/impact ./internal/cli` |
| V1-S16 | §16 explain, status, and doctor | `go test ./internal/cli` |
| V1-S17 | §17 verdict and exit-code contract | `go test ./internal/verdict ./internal/exitcode ./internal/cli` |
| V1-S18 | §18 built-in providers and adapters | `go test ./internal/provider` |
| V1-S19 | §19 evidence model and schemas | `go test ./internal/evidence ./pkg/schema` |
| V1-S20 | §20 compilation stages, errors, and warnings | `go test ./internal/compiler` |
| V1-S21 | §21 repository state and change impact | `go test ./internal/git ./internal/impact` |
| V1-S22 | §22 execution, environment, retry, scheduling, and cache | `go test ./internal/executor ./internal/provider` |
| V1-S23 | §23 independent verification and protected contracts | `go test ./internal/repair ./internal/security` |
| V1-S24 | §24 bounded repair loop | `go test ./internal/repair` |
| V1-S25 | §25 terminal, JSON, JUnit, and GitHub reports | `go test ./internal/report ./internal/cli` |
| V1-S26 | §26 GitHub Actions integration | `actionlint .github/workflows/*.yml` and `release-validation` hosted jobs |
| V1-S27 | §27 trust, secrets, paths, symlinks, and integrity | `go test ./internal/security ./internal/provider ./internal/evidence ./internal/repair` |
| V1-S28 | §28 performance targets | `./scripts/record_performance.sh` on Linux and macOS |
| V1-S29 | §29 interruption and persistence reliability | `go test ./internal/evidence ./internal/executor ./internal/verify ./internal/repair` |
| V1-S30 | §30 OS, Git, and language compatibility | `./scripts/check_examples.sh`, `./scripts/cross_compile.sh`, and both hosted OS jobs |
| V1-S31 | §31 internal component and persistence boundaries | `go test ./...` |
| V1-S32 | §32 internal Go interface behavior | `go test ./internal/provider ./internal/evidence ./internal/report` |
| V1-S33 | §33 external provider v1 subprocess protocol | `go test ./internal/provider -run 'External'` |
| V1-S34 | §34 unit, golden, integration, E2E, fuzz, mutation, race, and coverage strategy | `./scripts/check_fuzz.sh`; `INTENTCI_RUN_MUTATION=1 ./scripts/validate_v1_release.sh` |
| V1-S35 | §35 functional requirements | `go test ./tests/acceptance -run '^TestV1Acceptance$'` |
| V1-S36 | §36 first-run and error-message UX | `go test ./internal/initcmd ./internal/cli` |
| V1-S37 | §37 milestone exit criteria | staged green PR history plus `./scripts/validate_v1_release.sh` |
| V1-S38 | §38 release acceptance | 20 named subtests and `acceptance-v1.json` |
| V1-S39 | §39 measurable post-release success signals | immutable run evidence, performance records, and release artifacts |
| V1-S40 | §40 risk mitigations | compiler warnings, strict mode, security tests, mutation gate, and bounded repair tests |
| V1-S41 | §41 deferred capabilities | scope audit in `docs/v1.md` and `docs/roadmap.md` |
| V1-S42 | §42 end-to-end example workflow | AC-20 in `tests/acceptance/v1_acceptance_test.go` |
| V1-S43 | §43 product positioning boundary | README and `docs/v1.md` documentation review |
| V1-S44 | §44 final v1 product boundary | complete release validation plus independent final repository review |

Sections 1–3 describe motivation, goals, and explicit non-goals; they introduce
no additional runtime contract beyond the controls above. Every subheading and
normative bullet in §§4–44 is owned by its section control. A release is blocked
if any referenced test, release-validation stage, required hosted job, or final
review fails.

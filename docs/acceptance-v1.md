# v1 acceptance matrix

IntentCI v1 conformance is executable. The canonical gate is:

```bash
./scripts/validate_v1_release.sh
```

It runs vet, race detection, the 100% statement-coverage gate, runtime schema
checks, all five language examples, cross-compilation, performance recording,
and the named §38 acceptance suite. The suite emits
`dist/release-evidence/acceptance-v1.json`.

Mutation checks are release-only because they are intentionally expensive:

```bash
INTENTCI_RUN_MUTATION=1 ./scripts/validate_v1_release.sh
```

## v1.md §38

Every checked item below links to the executable suite that generated the
machine-readable matrix. A release is blocked if any subtest or surrounding
release gate fails.

- [x] AC-01 — initialize an existing repository
- [x] AC-02 — author requirements in Markdown
- [x] AC-03 — compile byte-stable canonical JSON
- [x] AC-04 — reject invalid graphs with actionable diagnostics
- [x] AC-05 — verify obligations with existing test commands
- [x] AC-06 — map JUnit and SARIF reports
- [x] AC-07 — detect boundary violations
- [x] AC-08 — select requirements from changed paths
- [x] AC-09 — bind evidence to repository and contract hashes
- [x] AC-10 — assign every required obligation a verdict
- [x] AC-11 — prevent missing evidence from passing
- [x] AC-12 — produce a structured repair packet
- [x] AC-13 — invoke an external agent within a bounded loop
- [x] AC-14 — reject protected contract modification
- [x] AC-15 — stop repeated ineffective attempts
- [x] AC-16 — generate terminal, JSON, and JUnit reports
- [x] AC-17 — run in GitHub Actions without a service
- [x] AC-18 — build for Linux and macOS
- [x] AC-19 — keep telemetry disabled by default
- [x] AC-20 — cover both required end-to-end workflows and their manifests

Executable source:
[`tests/acceptance/v1_acceptance_test.go`](../tests/acceptance/v1_acceptance_test.go).

## v1.1.0 release evidence

The protected `main` commit `04f87d4` passed
`release-validation (ubuntu-latest)`, `release-validation (macos-latest)`, and
`build-matrix`:
[GitHub Actions run 30273342392](https://github.com/hypertrial/intentci/actions/runs/30273342392).
The tag workflow publishes its machine-readable acceptance matrix, Linux and
macOS performance records, and mutation reports alongside the v1.1.0 release.

## Release blockers

A v1 release cannot proceed with:

- an unchecked AC-01–AC-20 matrix entry;
- statement coverage below 100%;
- a race, vet, schema, example, or cross-platform failure;
- an unexplained live covered mutant in verdict, compiler, boundary, or repair
  contract-immutability code;
- a P0/P1 final review finding;
- a missing Linux or macOS required check;
- a dirty release worktree or missing release artifact.

# Roadmap

North-star specification: [`v1.md`](../v1.md).  
Current freeze: [`v1.md`](v1.md).

## Historical (shipped under Product Contract model)

### v0.1.0 — Local intent gate

Vertical slice: Product Contract → path impact → local checks → requirement statuses.

### v0.2.0 — Change Specs, cache, explain

Change Specs, successful-check cache, `explain`, 100% coverage gate.

### v0.3.0 — Local workflow hardening

Hooks, attestations, contract-weakening detection, JUnit parse, waivers.

### v0.4.0 — Semantic verification

Optional local/HTTP semantic providers (Product Contract overlay).

These lines are **superseded** by v1.0.0. See [migration-v0-to-v1.md](migration-v0-to-v1.md).

## v1.1.0 — Full v1 conformance (released 2026-07-27)

All acceptance criteria in [v1.md §38](../v1.md) / [acceptance-v1.md](acceptance-v1.md).

Internal milestones ([v1.md §37](../v1.md)):

1. Compiler foundation — Markdown → IR, `init` / `compile` / `schema`
2. Verification engine — providers, executor, verdicts, `verify`
3. Report adapters — JUnit/SARIF/JSON providers and reporters, `explain` / `status`
4. Incremental verification — `--changed`, cache
5. Repair loop — packets, bounded agent, `repair`
6. Release readiness — examples, docs, Apache-2.0, binaries

The v1.0.x tags remain immutable historical releases. v1.1.0 is the first
release blocked on the machine-readable §38 matrix, Linux/macOS release gates,
mutation evidence, performance records, and independent final review.

## Post-v1

Hosted services, OPA-native integration, distributed execution, container
sandboxing, signed evidence bundles, and the other features in v1.md §41 remain
deferred.

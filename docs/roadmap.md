# Roadmap

North-star specification: [`v1.md`](../v1.md).  
Current freeze: [`v1.md`](v1.md) (breaking rewrite toward `v1.0.0`).

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

## v1.0.0 — Spec complete (in progress)

All acceptance criteria in [v1.md §38](../v1.md) / [acceptance-v1.md](acceptance-v1.md).

Internal milestones ([v1.md §37](../v1.md)):

1. Compiler foundation — Markdown → IR, `init` / `compile` / `schema`
2. Verification engine — providers, executor, verdicts, `verify`
3. Report adapters — JUnit/SARIF/JSON providers and reporters, `explain` / `status`
4. Incremental verification — `--changed`, cache
5. Repair loop — packets, bounded agent, `repair`
6. Release readiness — examples, docs, Apache-2.0, binaries

Public tag: **`v1.0.0`** only when §38 is green.

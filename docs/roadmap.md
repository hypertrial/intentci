# Roadmap

North-star specification: [`v1.md`](../v1.md).  
Current freeze: [`v0.4.md`](v0.4.md).

## v0.1.0 — Local intent gate (shipped)

Vertical slice: Product Contract → path impact → local checks → requirement statuses with text/JSON/exit codes.

## v0.2.0 — Change Specs, cache, explain (shipped)

- Change Specs (`intentci change create`, acceptance criteria as temporary requirements)
- Content-addressed successful-check cache
- `intentci explain <requirement-id>`
- CI-enforced 100% statement coverage

See [v0.2.md](v0.2.md).

## v0.3.0 — Local workflow hardening (shipped)

- Pre-push hook install/uninstall
- Attestations (`--attest`)
- Contract-weakening detection against the base commit
- Optional JUnit result parsing
- Waivers with expiry validation

See [v0.3.md](v0.3.md).

## v0.4.0 — Semantic verification (current)

- Optional local/remote semantic providers (explicit opt-in)
- Structured semantic findings with evidence citations
- Deterministic failures outrank semantic assessments

See [v0.4.md](v0.4.md).

## v1.0.0 — Spec complete

All acceptance criteria in [v1.md §31](../v1.md).

# Roadmap

North-star specification: [`v1.md`](../v1.md).  
Current freeze: [`v0.1.md`](v0.1.md).

## v0.1.0 — Local intent gate (current)

Vertical slice: Product Contract → path impact → local checks → requirement statuses with text/JSON/exit codes.

See [v0.1.md](v0.1.md) for the in/out cut line.

## v0.2.0 — Change Specs and cache

- Change Specs (`intentci change create`, acceptance criteria as temporary requirements)
- Content-addressed successful-check cache
- `intentci explain <requirement-id>`

## v0.3.0 — Local workflow hardening

- Pre-push hook install/uninstall
- Attestations (`--attest`)
- Contract-weakening detection against the base commit
- Optional JUnit result parsing
- Waivers with expiry validation

## v0.4.0 — Semantic verification

- Optional local/remote semantic providers (explicit opt-in)
- Structured semantic findings with evidence citations
- Deterministic failures outrank semantic assessments

## v1.0.0 — Spec complete

All acceptance criteria in [v1.md §31](../v1.md):

- Multi-language fixture corpus (Python, Go, TypeScript, Rust)
- Full reliability and UX requirements
- Progressive adoption path documented and verified

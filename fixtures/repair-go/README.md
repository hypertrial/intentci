# Deterministic repair fixture

The committed implementation is intentionally wrong. The tracked fake agent
copies `repair/counter.fixed` over `counter.go`; IntentCI must independently
verify the second immutable attempt.

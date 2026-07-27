---
id: REQ-RUST-001
title: Rust calculator adds integers
status: active
priority: required
owners:
  - example-maintainers
applies_to:
  paths:
    - "src/**/*.rs"
---

# Intent

The Rust calculator must add two integers correctly.

# Obligations

```yaml
- id: OBL-RUST-001
  statement: The Rust test suite passes.
  required: true
  verify:
    provider: command
    id: cargo-test
    run: "cargo test && printf 'intentci-ok\n'"
    inherit_environment:
      - HOME
      - CARGO_HOME
      - RUSTUP_HOME
    result:
      type: exit_code
      equals: 0
      stdout:
        contains: intentci-ok
```

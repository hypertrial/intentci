---
id: REQ-REPAIR-001
title: Counter adds integers
status: active
priority: required
owners:
  - fixture-maintainers
applies_to:
  paths:
    - counter.go
    - counter_test.go
---

# Intent

The counter package must add two integers correctly.

# Boundaries

```yaml
allowed:
  - counter.go
forbidden:
  - counter_test.go
```

# Obligations

```yaml
- id: OBL-REPAIR-001
  statement: The Go test suite passes.
  required: true
  verify:
    provider: command
    id: go-test
    run: "go test ./... && printf 'intentci-ok\n'"
    inherit_environment:
      - HOME
      - GOCACHE
    result:
      type: exit_code
      equals: 0
      stdout:
        contains: intentci-ok
```

---
id: REQ-GO-001
title: Go calculator adds integers
status: active
priority: required
owners:
  - example-maintainers
applies_to:
  paths:
    - "*.go"
---

# Intent

The Go calculator must add two integers correctly.

# Obligations

```yaml
- id: OBL-GO-001
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

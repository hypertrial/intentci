---
id: REQ-PYTHON-001
title: Python calculator adds integers
status: active
priority: required
owners:
  - example-maintainers
applies_to:
  paths:
    - "*.py"
---

# Intent

The Python calculator must add two integers correctly.

# Obligations

```yaml
- id: OBL-PYTHON-001
  statement: The Python unit tests pass.
  required: true
  verify:
    provider: command
    id: python-test
    run: "python3 -m unittest -v && printf 'intentci-ok\n'"
    inherit_environment:
      - HOME
      - PYTHONPATH
      - VIRTUAL_ENV
    result:
      type: exit_code
      equals: 0
      stdout:
        contains: intentci-ok
```

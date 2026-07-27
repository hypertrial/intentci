---
id: REQ-TYPESCRIPT-001
title: TypeScript calculator adds integers
status: active
priority: required
owners:
  - example-maintainers
applies_to:
  paths:
    - "src/**/*.ts"
    - "test/**/*.mjs"
---

# Intent

The TypeScript calculator must compile and add two integers correctly.

# Obligations

```yaml
- id: OBL-TYPESCRIPT-001
  statement: TypeScript compilation and tests pass.
  required: true
  verify:
    provider: command
    id: npm-test
    run: "npm test && printf 'intentci-ok\n'"
    inherit_environment:
      - HOME
      - NODE_OPTIONS
    result:
      type: exit_code
      equals: 0
      stdout:
        contains: intentci-ok
```

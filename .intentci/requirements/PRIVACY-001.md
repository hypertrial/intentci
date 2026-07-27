---
id: PRIVACY-001
title: Telemetry remains disabled by default
status: active
priority: required
owners:
  - intentci
depends_on: []
applies_to:
  paths:
    - internal/config/**
    - .intentci/config.yaml
tags:
  - privacy
---

# Intent

IntentCI must not enable telemetry by default.

# Rationale

Local-first operation requires explicit opt-in for any outbound product telemetry.

# Obligations

```yaml
- id: OBL-001
  statement: Default config has telemetry.enabled false
  required: true
  verify:
    all:
      - provider: command
        id: telemetry-default
        run: "go test ./internal/config -run TestDefaultAndValidate && printf 'intentci-ok\n'"
        inherit_environment:
          - HOME
          - GOCACHE
        result:
          type: exit_code
          equals: 0
          stdout:
            contains: intentci-ok
```

---
id: BUILD-001
title: IntentCI packages test cleanly
status: active
priority: required
owners:
  - intentci
depends_on: []
applies_to:
  paths:
    - cmd/**
    - internal/**
    - pkg/**
    - go.mod
    - go.sum
tags:
  - build
---

# Intent

The IntentCI Go packages under cmd/, internal/, and pkg/ must pass unit and integration tests.

# Rationale

A broken test suite cannot gate product intent.

# Constraints

## Must

- id: CON-001
  statement: Use the repository Go module test command.

## Must Not

- id: CON-002
  statement: Do not skip the coverage gate in CI.

# Boundaries

```yaml
allowed:
  - cmd/**
  - internal/**
  - pkg/**
  - go.mod
  - go.sum
  - scripts/**
  - docs/**
  - examples/**
  - .github/**
  - .intentci/**
forbidden: []
```

# Obligations

```yaml
- id: OBL-001
  statement: go test ./... passes
  required: true
  verify:
    all:
      - provider: command
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

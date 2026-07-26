# Security

## Local execution

IntentCI runs repository-defined provider commands on the local machine (and in CI). Treat untrusted repositories like untrusted build scripts.

## Telemetry

`telemetry.enabled` defaults to `false`. IntentCI does not phone home.

## Secrets

Evidence redaction patterns under `evidence.redact.environment` replace matching environment variable values with `[REDACTED]` in retained outputs where applied.

Do not embed credentials in requirement files or provider URLs.

## Repair immutability

During `intentci repair`, modifications under `.intentci/config.yaml` and `.intentci/requirements/**` are rejected unless `repair.allow_requirement_changes` is true. Violations exit with code `10`.

## Reports

Stdout/stderr retention is configurable. Prefer failing closed on missing evidence (`unproven`) rather than treating absence as success.

# Security

## Host execution trust boundary

IntentCI v1 is not a sandbox. Command providers, external providers, and repair
agents execute on the host as the current user. Treat an untrusted repository
like an untrusted build script. Run it in an isolated CI worker or VM when that
trust is not appropriate. Repair prints a warning before invoking an agent.

## Paths and artifacts

Provider working directories, reports, outputs, artifacts, protected paths, and
evidence locations must be repository-relative. IntentCI rejects absolute
paths, traversal, and symlink escape. Security or boundary stops use exit `10`.

The default repair-protected paths include `.intentci/**` and
`.github/workflows/**`. Contract/config hashes are pinned before the first
attempt, so pre-dirty protected content and agent changes are checked against
the original state.

## Environment and secrets

Providers receive a minimal environment plus explicit `inherit_environment`
and `environment` entries. IntentCI injects only documented run metadata.
Do not embed credentials in requirements, provider URLs, commands, or repair
packets.

`evidence.redact.environment` matches environment variable names. Their current
values and conventional `NAME=value` renderings are redacted before logs,
reports, packets, and JSON evidence reach disk. Redaction is defense in depth,
not permission to print secrets: transformed, encoded, split, or previously
persisted credentials may not be recognizable.

## Evidence and interruption

Run files use same-directory temporary writes and rename. Attempts and finalized
runs are immutable. `manifest.json` hashes all immutable artifacts except
itself and `final-verdict.json`; the final verdict records the manifest hash.

`SIGINT` and `SIGTERM` cancel provider commands, retain completed evidence and
partial logs, mark incomplete work as error, and cannot produce a passing final
verdict.

## Telemetry and network

Telemetry defaults to disabled and IntentCI does not phone home. Network access
occurs only through repository-defined commands, external providers, agents, or
the operator's package/release tooling.

## Reporting vulnerabilities

Do not open a public issue for a suspected vulnerability. Use GitHub's private
security-advisory reporting for `hypertrial/intentci` and include affected
versions, reproduction steps, impact, and any suggested mitigation.

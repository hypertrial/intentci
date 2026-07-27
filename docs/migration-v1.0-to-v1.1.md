# Migrating from v1.0.x to v1.1.0

v1.1.0 keeps schema/protocol major version `1` and accepts valid v1.0.x
requirements and configuration. It is stricter about malformed input and adds
provenance fields to outputs.

The published v1.0.0 and v1.0.1 tags are immutable historical releases.
v1.1.0, released 2026-07-27, supersedes them as the first release validated
against every normative v1 requirement.

## Compatible additions

- requirement and obligation dependency, timeout, retry, platform, evidence
  class, and confidence metadata;
- typed provider working directory, environment, inputs, outputs, exclusivity,
  retry, and artifact fields;
- repository base/head, dirty fingerprint, diff hash, rename/delete/binary,
  line-count, mode, and untracked metadata;
- attempt, provider-version, plan-hash, source-evidence-hash, artifact, and
  manifest provenance;
- verification-plan and stable-report schemas;
- CLI selectors and execution controls documented in the README.

Consumers must ignore unknown additive fields within schema major version 1.

## Inputs that now fail

v1.1 correctly rejects inputs that v1.0.x might have accepted or silently
misinterpreted:

- unknown YAML keys;
- invalid status, priority, verdict, evidence-class, or schema versions;
- invalid or absolute globs and unsafe paths;
- duplicate IDs, missing dependencies, and dependency cycles;
- empty or unsupported logical verifier expressions;
- unsupported JSON expressions and provider fields;
- empty CLI selectors and invalid formats.

Compile failures use exit `5`; invalid CLI usage uses exit `8`.

## Operational changes

- successful evidence caching is provenance-complete and only applies to
  deterministic passing evidence;
- generated JUnit/SARIF reports must be fresh;
- repair `max_attempts` counts the initial failed verification;
- repair pins the compiled contract for the entire run;
- evidence attempts are immutable and the final verdict references the
  manifest hash;
- repair still executes on the host and prints a prominent warning.

Recommended upgrade check:

```bash
intentci compile --strict
intentci verify --all --no-cache --format json
./scripts/validate_v1_release.sh  # when developing IntentCI itself
```

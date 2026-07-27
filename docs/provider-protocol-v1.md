# Provider protocol v1

Built-in and external providers produce evidence; they never assign the final
requirement verdict.

## Provider fields

Provider specifications support:

- `provider`, `id`, `working_directory`;
- `inherit_environment` and explicit `environment`;
- `timeout`, `retry.attempts`, and `retry.backoff`;
- `depends_on`, `inputs`, `outputs`, and `exclusive`;
- `artifacts` for collected repository-relative output files;
- `evidence_class`;
- provider-specific `run`, `report`, `result`, `assert`, `match`, `allow`,
  `allowed`, `forbidden`, `paths`, `expect`, `prompt`, and `configuration`.

All paths are repository-relative and are checked for traversal and symlink
escape. Output conflicts and dependencies are part of the bounded execution
DAG.

## External provider discovery

The provider name `foo` resolves to `intentci-provider-foo` on `PATH`.
IntentCI writes one JSON request to stdin, reads one JSON response from stdout,
and treats stderr as diagnostics. A nonzero process exit, timeout, malformed
JSON, missing version, or incompatible protocol major is an error.

Request shape:

```json
{
  "protocol_version": "1.0",
  "run_id": "01...",
  "attempt_id": "attempt-001",
  "requirement_id": "REQ-001",
  "obligation_id": "OBL-001",
  "repository": {
    "root": "/absolute/repository/path",
    "commit": "full-head-sha",
    "base_commit": "full-base-sha",
    "diff_hash": "sha256",
    "changed_files": ["src/example.go"]
  },
  "verifier": {},
  "configuration": {},
  "timeout_ms": 30000
}
```

Response shape:

```json
{
  "protocol_version": "1.0",
  "provider": "foo",
  "provider_version": "2.3.1",
  "status": "completed",
  "evidence": [
    {
      "id": "check",
      "class": "deterministic",
      "summary": "check passed",
      "passed": true
    }
  ],
  "diagnostics": [],
  "extra": {}
}
```

Unknown response fields are ignored for forward-compatible minor additions.
The `status` value is `completed`, `error`, or `skipped`.

## JSON provider subset

The v1 JSON provider accepts a deliberately small JSONPath-compatible subset:

- root: `$`;
- member access: `$.metrics.coverage`;
- array index access: `$.results[0].level`.

Operations are `exists`, `equals`, `not_equals`, `gt`, `gte`, `lt`, and `lte`.
Unsupported syntax is a compile error.

## Generated reports

When `run` is present on JUnit or SARIF providers, IntentCI removes an existing
report before execution. A generator must create a fresh report for that
invocation. A nonzero generator with a passing or missing report is `error`; a
fresh report containing violations is `fail`.

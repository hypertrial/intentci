# Configuration

IntentCI resolves configuration in this order, from highest to lowest:

1. command-line flags;
2. explicit `INTENTCI_*` environment variables;
3. `.intentci/config.local.yaml`;
4. `.intentci/config.yaml`;
5. built-in defaults.

Unknown YAML keys and malformed environment values are errors.
`config.local.yaml` is gitignored by `intentci init`.

## Environment overrides

Strings are used verbatim. Booleans and integers use Go syntax. The timeout
uses Go duration syntax such as `30s` or `5m`. Lists must be JSON arrays, not
comma-separated strings.

| YAML leaf | Environment variable | Type |
| --- | --- | --- |
| `version` | `INTENTCI_VERSION` | integer |
| `project.name` | `INTENTCI_PROJECT_NAME` | string |
| `requirements.paths` | `INTENTCI_REQUIREMENTS_PATHS` | JSON string array |
| `verification.default_timeout` | `INTENTCI_VERIFICATION_DEFAULT_TIMEOUT` | Go duration |
| `verification.max_parallel` | `INTENTCI_VERIFICATION_MAX_PARALLEL` | integer |
| `verification.fail_fast` | `INTENTCI_VERIFICATION_FAIL_FAST` | boolean |
| `verification.working_directory` | `INTENTCI_VERIFICATION_WORKING_DIRECTORY` | string |
| `verification.require_clean_worktree` | `INTENTCI_VERIFICATION_REQUIRE_CLEAN_WORKTREE` | boolean |
| `change_impact.base_ref` | `INTENTCI_CHANGE_IMPACT_BASE_REF` | string |
| `change_impact.include_untracked` | `INTENTCI_CHANGE_IMPACT_INCLUDE_UNTRACKED` | boolean |
| `change_impact.run_unmapped_requirements` | `INTENTCI_CHANGE_IMPACT_RUN_UNMAPPED_REQUIREMENTS` | boolean |
| `change_impact.fail_on_unmapped` | `INTENTCI_CHANGE_IMPACT_FAIL_ON_UNMAPPED` | boolean |
| `change_impact.global_paths` | `INTENTCI_CHANGE_IMPACT_GLOBAL_PATHS` | JSON string array |
| `evidence.directory` | `INTENTCI_EVIDENCE_DIRECTORY` | string |
| `evidence.retain_stdout` | `INTENTCI_EVIDENCE_RETAIN_STDOUT` | boolean |
| `evidence.retain_stderr` | `INTENTCI_EVIDENCE_RETAIN_STDERR` | boolean |
| `evidence.hash_algorithm` | `INTENTCI_EVIDENCE_HASH_ALGORITHM` | string |
| `evidence.redact.environment` | `INTENTCI_EVIDENCE_REDACT_ENVIRONMENT` | JSON string array |
| `repair.max_attempts` | `INTENTCI_REPAIR_MAX_ATTEMPTS` | integer |
| `repair.stop_on_repeated_diff` | `INTENTCI_REPAIR_STOP_ON_REPEATED_DIFF` | boolean |
| `repair.stop_on_repeated_failure` | `INTENTCI_REPAIR_STOP_ON_REPEATED_FAILURE` | boolean |
| `repair.allow_requirement_changes` | `INTENTCI_REPAIR_ALLOW_REQUIREMENT_CHANGES` | boolean |
| `repair.allow_test_changes` | `INTENTCI_REPAIR_ALLOW_TEST_CHANGES` | boolean |
| `repair.protected_paths` | `INTENTCI_REPAIR_PROTECTED_PATHS` | JSON string array |
| `ci.fail_on` | `INTENTCI_CI_FAIL_ON` | JSON string array |
| `telemetry.enabled` | `INTENTCI_TELEMETRY_ENABLED` | boolean |

Example:

```bash
export INTENTCI_VERIFICATION_MAX_PARALLEL=8
export INTENTCI_CHANGE_IMPACT_GLOBAL_PATHS='["go.mod","go.sum",".github/**"]'
intentci verify --changed --fail-fast
```

Provider-level environment is separate. Providers receive a small baseline
environment, the `inherit_environment` allowlist, explicit `environment`
values, and the documented `INTENTCI_*` run variables. This prevents accidental
whole-environment capture.

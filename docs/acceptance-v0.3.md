# v0.3.0 Acceptance Checklist

- [ ] `./scripts/check-coverage.sh` reports 100.0%
- [ ] CI enforces coverage on Linux and macOS
- [ ] `intentci hook install` / `uninstall` compose and strip marked sections only
- [ ] Unmanaged pre-push without markers is refused (no silent overwrite)
- [ ] `intentci verify --attest` writes attestation on PASS; skips write on non-PASS
- [ ] Contract weakenings vs merge-base appear in `contract_changes`
- [ ] Effective base policy still verifies removed/weakened base requirements
- [ ] Without `type: contract` Change Spec, weakenings force overall `unverified`
- [ ] With approved `type: contract` Change Spec, weakenings are reported but do not force unverified
- [ ] Approved Change Spec waiver yields requirement status `waived` and does not fail the run
- [ ] Draft Change Spec waivers do not skip blocking failures
- [ ] Expired / incomplete waivers fail `validate` / `verify` (exit 20)
- [ ] Softening `unknown_blocks` / `unverified_blocks` or removing JUnit `results` appears in `contract_changes` and is restored by effective policy
- [ ] `--attest` skips writing when any check record is fail/unknown even if overall PASS via waiver
- [ ] JUnit `results.format: junit` maps suite failures to check FAIL and parse errors to UNKNOWN
- [ ] GitHub Release `v0.3.0` published

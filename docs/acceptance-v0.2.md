# v0.2.0 Acceptance Checklist

- [ ] `./scripts/check-coverage.sh` reports 100.0%
- [ ] CI enforces coverage on Linux and macOS
- [ ] `intentci change create FOO-1` creates `.intentci/changes/FOO-1.yaml`
- [ ] `intentci verify --change FOO-1` surfaces AC statuses
- [ ] `affected_requirements` force selection without path hits
- [ ] Approved Change Spec edits (and demotions) appear in `change_findings` vs merge-base
- [ ] Change Spec filename id must match YAML `id`
- [ ] Cache hit/invalidate/corrupt/no-inputs behaviors
- [ ] `--no-cache` forces re-execution
- [ ] `intentci explain BUILD-001` works offline
- [ ] GitHub Release `v0.2.0` published

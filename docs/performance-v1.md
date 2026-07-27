# v1 performance validation

Performance is recorded rather than gated by noisy hosted-runner wall time.

```bash
./scripts/record_performance.sh
```

The record contains platform, Go version, commit, packaged binary size, peak
resident memory for the no-op `version` command, and five-run benchmark samples
for:

- CLI startup;
- compilation of 100 and 1,000 requirements;
- change-impact analysis over 10,000 files;
- aggregation of 10,000 obligation/test-case verdicts;
- bounded scheduler overhead.

Peak resident memory for `intentci version` is the v1 idle-memory proxy because
IntentCI is a terminating CLI rather than a resident service.

The targets remain those in [v1.md §28](../v1.md). Absolute Linux and macOS
records are uploaded as release evidence. Regressions are reviewed against the
same platform's previous record; hosted wall time is not an absolute merge
gate.

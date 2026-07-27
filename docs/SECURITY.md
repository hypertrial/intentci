# Security

IntentCI v2 is a local command runner, not a sandbox.

Checks execute as `/bin/zsh -lc` from the repository root with the current
user's complete environment and permissions. A repository can therefore read,
modify, or transmit anything the user can. Inspect `.intentci.yaml` before
running IntentCI in an untrusted repository; use a disposable VM when trust is
uncertain.

IntentCI validates its configuration and path globs, writes only
`.intentci.yaml` during `init`, persists no command output, has no telemetry,
and performs no network access itself. Commands may still write files, reveal
environment values, or use the network.

Report suspected vulnerabilities through GitHub's private security-advisory
flow for `hypertrial/intentci`, not a public issue.

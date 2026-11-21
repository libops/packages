# Pipeline contract

This describes the intended pipeline. See the [roadmap](roadmap.md) for actual
implementation status.

## Scheduling

1. Validate inventory, base pins, generated jobs, and changed files.
2. Select affected target recipes plus their transitive dependents.
3. Start independent builds immediately. A child's build needs only publication
   of its direct prerequisites in the same distro release.
4. Publish each target's artifacts. For a combined architecture image, wait only
   for that target's architectures.
5. Run upstream source checks and install/run smoke tests separately, reusing
   saved build state instead of recompiling unnecessarily.
6. Require all selected builds and tests to succeed for the merge status.

A test failure must fail merge checks without suppressing unrelated builds.
Skipped unselected targets are expected; skipped selected targets are failures.
Plan failures and cancellations cannot produce a successful aggregate check.

## Change selection

| Change | Work selected |
| --- | --- |
| Recipe or its patches | That target and its descendants |
| Distro release base pin | Targets within that distro release |
| Format-specific builder | Targets using that format |
| Shared build logic | Every affected target |
| Documentation, issue forms, CODEOWNERS | Validation, no package rebuilds |
| Generated jobs / Renovate config / command allowlist | Validate generation; inventory and recipe inputs select work |

Treat recipe additions, deletions, renames, and inventory edits explicitly. PRs
compare against the main merge base; main compares against the previous commit.
A manual full rebuild remains available.

## Publication and auth

Release images use the [base/version tag contract](consuming.md#contract).
PR and cache references must include enough target/PR/architecture identity to
avoid collisions. Never overwrite release tags from PRs. Preserve working cache
reuse across commits instead of accumulating a unique cache tag forever.

Auth belongs in Actions or the local environment. Build scripts use configured
Docker credentials. Untrusted fork code must not receive publish credentials or
run update scripts on the trusted self-hosted runner.

## Cleanup

Use trusted main-branch cleanup code after a PR closes. Prevent races with an
in-flight publisher, enumerate exact PR-owned refs, paginate API requests, and
make retries safe. Deleting a GHCR version removes every tag on it: do not delete
versions shared with release tags. Do not indiscriminately delete untagged
manifests, because a multiarchitecture image may still reference them.

## Renovate cadence

The current draft schedule uses minutes 7, 22, 37, and 52 during weekdays
07:00-23:59 UTC, and hourly outside that window and on weekends. This is a broad
release window, not a project-specific guaranteed upstream schedule. Persistent
cache and serialized runs reduce waste. Source PR count throttles are disabled.

GitHub cron is best effort. An operator-controlled scheduler can dispatch:

```sh
gh workflow run renovate.yml --repo libops/packages --ref main
```

Dispatch requires operator-provided GitHub credentials. Do not provision another
scheduler before measuring actual scheduling delays.

# CI standards

## Commands

Package commands have no prefix: `plan`, `build`, `publish`, `manifest`, `test`,
`update`, `revise-base`, `review-lines`. Other concerns use `dev-*`, `builder-*`, `registry-*`
and `docs-*`. `make help` groups them. Actions supplies runners, scheduling and
credentials; Make/Go owns execution locally and in CI.

`make dev-check` runs actionlint, formatting, vet, Go tests, coverage floors, inventory/source
validation and generated-file checks. Artifact builder coverage must be at least
70%; the source/metadata SDK at least 70%. Test artifact production and failure
cleanup, not just plan serialization. `make docs-build` validates docs strictly.
`make dev-test-install` uses Docker to
verify that direct files beat newer distro packages and resolve a carrier-only
dependency through the index; CI runs it when the plan selects package work.

## Build graph

Select changed recipes and descendants; base changes select that release.
Documentation changes do not rebuild packages. Helper/workflow/module PRs validate
one representative Nginx target with the changed helper. Main does not repeat that
package validation; its utility workflow publishes the helper separately. A weekly
Sunday 03:11 UTC full build/test covers every enabled family and architecture
without release promotion. Its separate concurrency groups cannot lock main releases.

Each target publishes after its architecture builds finish. Children wait for
publication, not parent tests. Separate upstream/install tests and every selected
build must pass the aggregate merge check. Selected skips, cancellation or failed
planning cannot pass. Forks build/test local carriers without publishing.

New pushes cancel older runs of the same PR. Different PRs run concurrently.
Main pushes keep their runs; each target's publication uses a shared concurrency
group with cancellation disabled and `queue: max` (up to 100 waiting jobs).
After a force-push, planning first fetches the previous commit by object ID;
if GitHub no longer retains it, planning selects all packages conservatively. Explicit comparison and PR errors still fail.
Planning needs no registry credentials: recipe, base and prerequisite
changes must increase the version or `pkgrel` against the Git base. Renovate base
updates run `make revise-base`. Missing bumps fail before builds. Manual full-matrix
dispatch validates candidates without publishing releases.

Main performs authenticated collision checks and fails on changed bytes under an
existing revision. Bootstrap denial is deferred until after candidate publication;
final promotion still fails closed. Identical manifest retries pass. A released
child can use only published parent revision digests; PR candidates stay disposable.
Version aliases advance to the highest published revision.

## Trust and credentials

Every workflow starts with `permissions: {}` and grants permissions per job.
Every checkout disables credential persistence. Pass secrets individually.
Package workflows receive only registry secrets, never `GH_APP_PRIV_KEY`.
Fork code never receives publishing secrets or uses the trusted self-hosted runner.

Use `GH_APP_ID` and `GH_APP_PRIV_KEY` for the existing lbops-renovate app.
GHCR normally uses `GITHUB_TOKEN`; other registries use configured Docker helpers
or `REGISTRY_USER`/`REGISTRY_PASSWORD`. Interactive local commands hide secret
input. Missing noninteractive auth fails; credentials never enter arguments/logs.

Pin action commits, tool/base image digests and an exact Go toolchain. Use bundled
BuildKit frontend features without a `# syntax=` image. Native subprocesses need
bounded contexts and process-group cancellation; downloads need limits/timeouts.
Source signatures or independently authenticated checksums are preferred.
Trust-on-first-use updates need human review and cannot auto-merge.

## Cleanup

Carriers publish as `ghcr.io/<owner>/<package>`. Cleanup requires the repository
owner and verifies each package/repository association before touching its tags.
Run only trusted main cleanup code for closed PRs. Match exact PR tag prefixes,
wait for publishers, paginate and recheck candidates before deleting. Protect
release/shared versions and reachable index children. Dry-run is local default. GHCR cleanup does not imply DockerHub/GAR GC.
Never promote disposable digests outside their publishing workflow. Main candidates
include run ID/attempt; successful-main cleanup checks workflow identity and protects
released/reachable/shared digests. Failed-run candidates are retained; use a full
rerun until partial manifest retry/recovery has been verified live. Active
`cache-main-*` tags are reused per target and architecture. Daily cache
collection quarantines only unreachable OCI manifests whose config media type is
`application/vnd.buildkit.cacheconfig.v0`. A dated retirement tag must remain for
seven days before deletion; fresh tags/identity are rechecked. Application images,
unknown untagged artifacts, indexes and referenced/shared cache versions are
protected. `make registry-clean-cache` previews this; `DRY_RUN=0` applies it.

## Workflow lint

`make dev-lint` runs pinned actionlint. Version 1.7.12 does not yet recognize GitHub's documented
[`queue: max`](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency);
use `-ignore 'unexpected key "queue" for "concurrency" section'` for that one
compatibility error. Do not suppress unrelated findings.

The installation regression is a separate required job (`make dev-test-install`,
25-minute timeout). Package builds depend only on planning and their parent
builds; Alpine mirror availability during this regression cannot delay them.
The fixture uses any catalogued Alpine 3.24 base, independent of Nginx support.

Weekly validation schedules 32 architecture builds plus their tests, including
multi-hour Java compilations. Budget runner hours, disk and concurrency for this
full matrix; separate concurrency groups avoid serialization, but runner
capacity is still shared with releases. Use dedicated validation runners if shared runner capacity becomes a
bottleneck. Review actual Actions usage before expanding the matrix.

A failed scheduled run opens a repository issue linking the run, through
`validation-alert.yml`. The maintaining team must watch repository **Issues**
(custom notifications) or route those issues through its existing team integration;
cron-owner email is not the alert channel. This workflow uses no PR checkout or
registry credentials. GitHub delivery and issue notifications still need live
verification before relying on this route.

GitHub can delay scheduled runs and disables public-repository schedules after
60 days without repository activity. Monitor the most recent scheduled run from
outside GitHub. An external scheduler can dispatch `push.yml` with `all=true`
using `gh workflow run push.yml --ref main -f all=true`; do not use keepalive
commits. Manual full-matrix failures need the same operational review as weekly
failures. A GitHub-only alert cannot detect a schedule that never starts.

See GitHub’s [schedule limits](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule)
and [notification routing](https://docs.github.com/en/actions/concepts/workflows-and-actions/notifications-for-workflow-runs).


Go's full upstream suite runs with three explicit environment exclusions:
`TestTSAN` requires ASLR controls unavailable in the build container (also
excluded by [Alpine's Go recipe](https://github.com/alpinelinux/aports/blob/master/community/go/APKBUILD)).
On AMD64 only, `TestSigStackSwapping` and `TestSigaltstack` abort because their
C fixtures allocate musl's fixed `SIGSTKSZ`, below the kernel minimum on some
GitHub runners. The same runner limitation is documented in the
[kernel signal-stack fix](https://lore-kernel.gnuweeb.org/lkml/3c2lpv75qz67rkcirawgfcd63pazgur6tw6qp7m6z5tdfimeeh%40v2em57j4ix6e/T/).
These are exact test-name exclusions, not package/suite skips; ARM64 keeps the
signal-stack tests. Review them when upstream fixtures or the runner environment
change. Source code and compiler output receive no downstream patch.


Ruby suites install Bash and timezone data. Bootstrap tests use normal stack
defaults; the full unit suite uses 4 MiB VM/machine test stacks.
Ruby 3.3/3.4 exclude only `test_dlopen_linker_script_group_linux`: its expected
`libc.so` name differs from musl's `ld-musl-<arch>.so`. Other Fiddle tests and
all timezone, recursion and crash-report tests remain enabled.


Python exclusion lists apply to individual test names via upstream's `--ignorefile`;
no module is disabled. Python 3.12/3.13 lists cover musl locale/coercion differences,
loader error text and invalid-FD `pathconf`. Python 3.12 additionally excludes
the signed `NODEV` value and a test-only
use-after-close: `_testcapi.run_fileexflags` calls `fileno()` after `fclose()`
([upstream fixture](https://github.com/python/cpython/blob/3.12/Modules/_testcapi/run.c)).
The freeze test keeps running with a ten-minute timeout to rebuild CPython.
Python 3.14 excludes only deleted-libpython remote execution: the interpreter
cannot be found after unlinking its mapped library in this build container;
other remote-execution tests remain enabled. This limitation remains visible in
the roadmap pending upstream/container resolution. Review all exclusions when
upstream or musl changes; they are compatibility limits, not full upstream passes.

Renovate runs its own pinned container on GitHub-hosted Ubuntu, including trusted
Go updater commands. Optional repository variable `RENOVATE_RUNNER` selects a
dedicated runner label; it must provide Docker and allow the shared Go updater.

Documentation refreshes the registry-backed package reference after a successful
main `Packages` run. The `workflow_run` job checks out trusted main, never PR code
or artifacts. PR documentation builds use the committed reference without registry
credentials. `make dev-test-install` executes the same named installer consumers
use against an older-than-Alpine fixture and checks its carrier-only dependency.

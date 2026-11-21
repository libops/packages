# Implementation guide

The repository provides Alpine APK builds for the targets in `packages.json`.
Debian, Ubuntu and additional architectures require separate adapters and validation.
The [roadmap](roadmap.md) contains unfinished work; it is not a release history.

## Metadata and execution

`buildhelpers.InventoryPackage` is the shared inventory model used by the catalog,
source updater, pin checks and cleanup; unknown fields are rejected. Target paths identify the distro, release,
software and maintained line. Software path segments are alphanumeric so generated
job IDs cannot collide through punctuation normalization.

`Dockerfile.package` builds every APK carrier. Source arguments come from the
native recipe's version, archive and strong checksum. The helper validates those
arguments and runs `abuild fetch verify` before compilation. There are no generated
per-target Dockerfiles. Every carrier includes a verified, signed APK index for
dependency resolution. Go emits a relocatable `/packages/<native-name>` installer
for every APK, selecting exact files for dependencies produced in the same carrier
(including shared-library providers). PHP defaults add FPM and OPcache. Installers
use bundled `/packages/keys`; the legacy `/keys` path remains available.

`make dev-check` runs Go validation, regression tests, generated-file checks and
source-lock checks. `make build` and `make test` use the same helpers as CI without
publishing locally. `make publish` and `make manifest` require registry access.
See [first release](first-release.md) for setup and the
[helper API](build-helpers.md) for utility images.

## Publication and trust

Carriers use package names (`ghcr.io/libops/php`, `ghcr.io/libops/python`);
utility images use `ghcr.io/libops/package-builder`.
Main publication is serialized per target with GitHub's queued concurrency.
Planning compares local Git history (recovering force-pushed commits when needed): compare package inputs with the Git base and require a new
version or revision, including affected descendants. New/private/fork targets do
not need registry read access to plan. Helper/workflow PRs validate a representative
Nginx target; helper-only main merges do not repeat package builds. Utility images
have their own publication workflow. Manual full-matrix dispatch and the weekly Sunday run are validation-only. Their
concurrency groups are separate so validation does not lock out urgent releases.

Authenticated release preflight checks existing tags before compilation. A denied
lookup during registry bootstrap is deferred, never treated as absence. Final
promotion checks again after candidate publication: different bytes fail, identical
manifest retries succeed. Version aliases follow the highest published revision.
Children on main must use their parent's published revision digest; disposable
candidate digests cannot be release dependencies.

OCI digest pinning is the carrier's trust anchor. APK keys are generated per build
and bundled with the APKs. These signatures detect inconsistent files within a
carrier; they do not independently authenticate LibOps as publisher. Never trust
an APK key obtained from an unverified carrier. See [consuming](consuming.md).

Builds use live Alpine dependencies and fresh signing keys; byte-for-byte
reproducibility is not claimed. Every changed release needs a new revision.

PHP and Nginx verify pinned-key GPG signatures. Python uses the maintained Sigstore
Go verifier with a reviewed release-manager identity and TUF-authenticated roots.
Other adapters still use upstream-hosted checksums and require human review.
Java automatic refresh is disabled pending official source/docs provenance.
Ruby imports Alpine's musl main-stack compatibility patch and build flags, while
retaining both bootstrap and full upstream suites. General aports attribution
is in `NOTICE` and included in Ruby APKs/source bundles. The aports tree has no
blanket license declaration; third-party reuse terms still require review.
No carrier SBOM is generated: consumers scan their final runtime images instead.

## CI boundaries

Workflows start with an empty permission set and grant permissions per job.
Checkout never retains credentials. Reusable package jobs receive only registry
secrets, not the GitHub App private key. Fork contributions build and test local
carriers without publishing or receiving registry credentials.

Helper changes use the current helper in the PR’s representative package check,
including during bootstrap. Each source target has an independent build/test chain.

The Go toolchain and Actions revisions are explicitly pinned. Renovate's runtime
image is digest-pinned. Native commands have deadlines and terminate process groups
on cancellation. Artifact-path tests cover successful collection, verification/build
failures, missing output, recipe restoration and private-key removal.

Every line has a support deadline. When upstream has no verified fixed EOL, the
inventory records a conservative review deadline with an explanatory note; this
must not be presented as an upstream support promise. Expiry blocks publication of
that target and the scheduled review reports retirement/review issues. Catalog
loading, cleanup and unrelated targets remain usable.

## Verification limits

Local Go checks exercise orchestration and artifact collection without requiring
registry writes. They do not establish that every upstream test suite passes on
every native architecture. Full native source/install testing, the GitHub/GHCR
workflow, registry immutability settings and isolated live cleanup must be verified
before production release. See the roadmap for the remaining source-authenticity,
platform validation work.

Local validation includes a native ARM64 Nginx build, separate upstream/install
tests, direct carrier-file installation using signed indexes and checksum locks, and
final-image inventory with Syft and Trivy.
Both scanners detect the installed Nginx version; this does not validate advisory
matching for every custom package. PHP 8.4/8.5/8.6 signatures and Python 3.12–3.15
Sigstore verification have passed against their current upstream releases.

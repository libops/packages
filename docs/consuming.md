# Consume packages

Use the [PHP quick start](index.md#quick-start-install-php) or choose any line
from the [package reference](packages.md), including verified digests and install commands. Carriers are scratch
images containing packages; mount them during your runtime image build.

## Contract

| Path | Contents |
| --- | --- |
| `/packages/<native-package-name>` | Executable installer for that package and carrier dependencies |
| `/packages/keys/*.pub` | Verification keys used by installers |
| `/packages/*.apk` | Native APKs and subpackages |
| `/packages/APKINDEX.tar.gz` | Signed index for this carrier’s APKs |
| `/keys/*.pub` | Public verification keys |
| `/metadata/release.json` | Package/source hashes, versions and security references |
| `/sources/*.src.tar.gz` | Corresponding source and packaging inputs |

Match the distro release: `alpine3.24` packages require Alpine 3.24. Pin an explicit
packaging revision and the carrier's OCI digest. Revision tags never move; a
published rebuild increments `rN`. Version-only aliases follow the highest revision.

```sh
docker buildx imagetools inspect ghcr.io/libops/php:8.4.28-r0-alpine3.24
```

Use the returned multiarchitecture index digest, not a source archive checksum or
one architecture's child manifest. Published artifacts can precede test completion;
select a digest whose required tests passed.

Copy carrier keys into `/etc/apk/keys` instead of mounting over that directory,
which would hide the runtime base's existing keys. Key filenames include a unique
build identity so multiple carriers can be installed together. The OCI digest is
the trust anchor: a key bundled with an unverified carrier is not independent
proof of publisher identity.

## Bake contexts

The isle-buildkit pattern supplies carriers as named contexts:

```hcl
target "app" {
  contexts = {
    php_packages = "docker-image://ghcr.io/libops/php:8.4.28-r0-alpine3.24@sha256:<verified-manifest-digest>"
  }
}
```

Use `from=php_packages` in the quick-start mounts; omit its `FROM` stage when
Bake supplies that context.

## Renovate

Renovate's Dockerfile manager updates `FROM` digests. For Bake context strings,
add a regex manager capturing the image name, tag and digest. Restrict both the
software line and distro release; new runtime minors require an explicit migration.

For example, constrain a PHP 8.4 / Alpine 3.24 consumer with:

```json
{
  "extends": ["config:recommended", "docker:pinDigests"],
  "packageRules": [{
    "matchDatasources": ["docker"],
    "matchPackageNames": ["ghcr.io/libops/php"],
    "versioning": "loose",
    "allowedVersions": "/^8\\.4\\.[0-9]+-r[0-9]+-alpine3\\.24$/"
  }]
}
```

Use the corresponding software line, registry image name and Alpine release for
other carriers. Revision-tag updates also pick up packaging fixes.

## One-command installation

Mount `/packages` from the pinned carrier and call its named entrypoint:

```dockerfile
RUN --mount=type=bind,from=php_packages,source=/packages,target=/packages \
    /packages/php84
```

The installer copies keys, uses the signed index, and selects exact carrier APK
files for the package and its dependencies within that carrier. Alpine supplies
remaining system dependencies. This avoids silently selecting Alpine's build
when it has a newer or equal version. Checksum constraints stay in `/etc/apk/world`.
No temporary repository tags or untrusted-install flags are used.

PHP's main installer includes CLI, FPM and OPcache. Other primary installers
follow their native package dependencies: OpenJDK includes its JDK. Every
subpackage has its own installer; call `/packages/php84-mbstring` to add that
extension. Docs and development packages are opt-in. Installers take no arguments.
They require Alpine's `/bin/sh`, `apk`, `cp`, and write access to the image root.

For multiple carriers, mount them at distinct paths and invoke each installer;
entrypoints locate their files relative to themselves. Match Alpine releases.
Older carriers without installers still support explicit APK-file installation
with `apk add --repository /packages/APKINDEX.tar.gz /packages/<exact-file>.apk`.

## Possible future APK repository

Current scope is Docker carriers; no hosted APK repository is available or required.
If non-Docker consumers need one, the same APK bytes, package names, versions and
signed indexes can be exported without rebuilding software. Docker carriers would
remain supported alongside it during a transition; migration would be opt-in.

Before offering HTTP consumption, define immutable per-release index URLs and
independent trust-key distribution/rotation. Preserve exact package constraints
and explicit version pins. A moving HTTP channel would not preserve an OCI digest
pin by itself. The intended change is repository location and key provisioning,
not package identity. See the [roadmap](roadmap.md).

## Runtime inventory

Keep `/lib/apk/db/installed`. Syft and Trivy discover installed packages from it.
Generate the SBOM from your final image, after installing application dependencies;
do not copy a carrier-wide inventory into it. See [scanning](security-metadata.md).

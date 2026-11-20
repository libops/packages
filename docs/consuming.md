# Consume packages

Use the [PHP quick start](index.md#quick-start-install-php). Carriers are scratch
images containing packages; mount them during your runtime image build.

## Contract

| Path | Contents |
| --- | --- |
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

## Local repositories

Use carrier files with the signed index available for dependency resolution:

```sh
apk add --no-cache --repository /packages/APKINDEX.tar.gz \
  /packages/php84-[0-9]*.apk /packages/php84-common-*.apk \
  /packages/php84-fpm-*.apk
```

File arguments select the carrier's exact bytes, even when Alpine has a newer or
equal package version. Bare names can silently select Alpine's build; exact version
constraints do not distinguish two builds with the same version. Keep the resulting
checksum constraints (`name><Q1...`) in `/etc/apk/world`. Explicitly select any
carrier subpackages you need; the resolver may otherwise choose distro dependencies.

Our flat layout requires the explicit index URL; directory repositories append an
architecture subdirectory. For cross-carrier dependencies, mount each `/packages`
at a distinct path, copy their keys, and supply each index plus its selected APK
files. Pin every carrier digest and match Alpine releases. Do not use temporary
repository tags: missing tags block later APK commands.

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

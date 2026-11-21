# Go build helpers

Module: `github.com/libops/packages`. The CLI in `cmd/packages` backs every Make
command and the utility image. Prefer stdlib; Sigstore's maintained Go verifier
handles Python authenticity, the OCI registry client handles typed registry errors,
and YAML decoding handles Ruby metadata.

## Source verification

```go
updater := buildhelpers.Updater{Context: ctx}
err := updater.Update(".", "alpine/3.24/php/8.4")
```

`Verify` and `VerifyReader` accept SHA-256/384/512 and reject malformed digests,
weak algorithms and mismatched bytes. `Updater.Client` supports HTTP fixtures.
Archives stream to bounded temporary files; metadata has a smaller memory limit.

Each source family has a small discovery adapter. PHP and Nginx check detached
GPG signatures against pinned release keys. Python checks Sigstore certificates,
reviewed issuer/identity, artifact digest, transparency log, SCT and timestamp,
using TUF-authenticated trust roots. Java automatic refresh is disabled pending
provenance review. Remaining same-host checksums require human review.

`InventoryPackage` is the shared metadata model. `SourcePins` derives arguments
for `Dockerfile.package` from the native source lock; `CheckPins` validates those
locks. `build-apk` verifies before compiling, restores test dependencies and
exports APKs, a signed `APKINDEX.tar.gz`, unique public keys, source bundles and
release hashes. Native `apk verify` checks the index before export. Tests run in
a separate stage. Private signing keys are removed before export.

## Utility image

The root Dockerfile compiles the helper and adds native packaging tools to the
explicitly pinned base. Utility publication has its own cadence: helper/base
changes, weekly refresh and manual dispatch. Each release's `builder.json` pins
the utility manifest and matching base digest. Missing/mismatched pins bootstrap
from the local helper; `LOCAL_BUILDER=1` explicitly selects it.

```sh
make builder-build TARGET=alpine/3.24 ARCH=arm64
make builder-pin TARGET=alpine/3.24 DIGEST=sha256:<published-digest>
```

`REPORT=1` opens the pin PR using the app token. Architecture profiles come from
`architectures.json`; only explicitly enabled target architectures build.

## Extending helpers

Pass validated configuration and contexts explicitly. Use direct subprocess args,
never shell-built commands. `CommandContext` bounds native commands and terminates
their process groups on cancellation. Add focused fixtures and artifact-path tests;
run `make dev-check` and update the relevant guide, not a session diary.

## Carrier installation and reference docs

`buildhelpers.WriteInstallers(directory, primary)` reads APK metadata and writes
shell entrypoints with exact file arguments and the transitive carrier dependency
closure. System dependencies remain APK's responsibility. `/packages/keys` and the
signed index accompany the scripts. No Go runtime or helper binary is installed
in consumer images.

`make docs-packages` resolves catalog revision tags and regenerates the package
reference and homepage quick start. `make docs-build` stays offline with respect
to registry metadata. The docs workflow refreshes after successful main package
runs and deploys the generated output without committing it.

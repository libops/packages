# Security metadata and runtime scanning

Our carriers install packages into your image; they are not deployed services.
We do not generate carrier SBOMs or copy one into consumers. An archive inventory
would omit your application and could list subpackages you never installed.

## Scan the final image

Keep the native APK database, `/lib/apk/db/installed`, and scan the completed
application image by digest:

```sh
syft registry.example/app@sha256:<digest> -o cyclonedx-json > app.cdx.json
trivy image --format cyclonedx --output app.trivy.cdx.json registry.example/app@sha256:<digest>
trivy image registry.example/app@sha256:<digest>
```

Syft generates inventory; Trivy's image command can inventory and scan. Preserve
scanner-specific metadata when passing an SBOM to another tool; importing a generic
SBOM can change vulnerability matching. APK version detection does not guarantee
correct vulnerability matching for custom builds: Alpine's advisory revisions
are about Alpine's packages. Check upstream release/advisory evidence too.

For Java, scan application JARs as well as the JDK. For Kubernetes reporting,
associate each running pod/container's resolved image digest with its final-image
inventory. A carrier digest cannot identify which application images or pods
installed its packages. Cluster inventory/UI belongs in consumer infrastructure,
not this package build pipeline.

## Release evidence

`/metadata/release.json` records exact APK/source SHA-256 hashes, native names,
versions, architectures, dependencies and version-specific security references.
BuildKit publication also requests minimal provenance. Neither is a vulnerability
scan or a claim that post-publication tests passed.

Each recipe's `security.json` has `version`, `status`, `advisories` and `references`.
Statuses are `security`, `maintenance` or `unknown`; reviewed classifications need
upstream HTTPS evidence. Missing evidence stays unknown. Updates reset the status
when the version changes.

## References

- [Syft APK discovery](https://oss.anchore.com/docs/capabilities/apk/)
- [Trivy Alpine detection and matching](https://trivy.dev/docs/latest/coverage/os/alpine/)
- [BuildKit provenance](https://docs.docker.com/build/metadata/attestations/slsa-provenance/)

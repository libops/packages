# Add a package or release line

A new runtime minor needs its own directory and review. Renovate updates only
maintenance releases inside configured lines.

1. Confirm the public-facing/untrusted-content scope and official upstream support.
   Record the support URL and end date, or a conservative review deadline.
2. Add `packages/<distro>/<release>/<software>/<line>/APKBUILD`, exact source URLs,
   strong checksums, notices and necessary packaging inputs. No `SKIP` checksums or
   runtime backports. Set LibOps as maintainer and retain original contributor credits.
3. Add a `packages.json` entry using an existing target as the schema example:
   target, family, line, test selector, supported deadline/reference, architecture
   opt-ins, same-release dependencies and fixed-line update rule. Python also
   requires reviewed Sigstore issuer/subject/reference metadata.
4. Reuse a source-family adapter or add one with fixtures. Prefer signatures;
   remaining trust-on-first-use updates need human review and never auto-merge.
5. Add install/run coverage in `test.Dockerfile`, then run:

   ```sh
   make dev-generate
   make dev-check
   make build TARGET=alpine/3.24/php/8.4 ARCH=arm64
   make test TARGET=alpine/3.24/php/8.4 ARCH=arm64
   ```

Replace the example only after the new target exists. Validate every enabled
native architecture, upstream suite, installation and source/license obligations.
`Dockerfile.package` is shared; do not add a generated per-target Dockerfile.
Every rebuilt published version needs a `pkgrel` bump.

Offer ongoing ownership or sponsorship in the package issue form. Acceptance adds
the agreed scope to the [release contract](../decisions.md).

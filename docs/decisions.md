# Project contract

- Package upstream-supported releases for internet-facing services and software
  processing their untrusted content. No runtime backports or extended support. Reviewed distro compatibility patches
  are allowed for supported releases; pin them in native source checksums and
  keep upstream tests enabled.
- Keep distro/release/software/line directories in one Git branch. APK, Debian
  and Ubuntu graphs stay independent; packages never cross distro releases.
- Use package-named carrier repositories (`ghcr.io/libops/php`); tags include software version,
  packaging revision and base. Revision tags are write-once; version aliases follow
  the highest revision. Recipe/base/dependency changes require a new version or
  `pkgrel`; changed bytes cannot reuse an existing revision. Consumers pin tested digests.
- Publish builds immediately. Children wait for prerequisite publication, never
  prerequisite tests. Separate upstream/install tests gate merging afterward.
- Build changed targets and descendants. Shared helper PRs require a representative
  package check; helper-only main merges publish utilities, not all packages. New targets add parallel work rather than a global build stage.
- GHCR is the default. Registry URI and externally configured auth support other
  registries without implicit mirroring. Cleanup is provider-specific and fail-closed.
- Source locks are native recipe checksums. Verify before extraction. Prefer
  publisher signatures; trust-on-first-use needs human review and never auto-merges.
  Runtime minor lines need new directories; Renovate stays within configured lines.
- Keep orchestration in Go, using stdlib where suitable. Use maintained libraries
  for signature verification rather than inventing cryptography. Bash stays small.
- Local and CI execution use the same Make targets. Interactive auth prompts only
  when needed; noninteractive missing credentials fail clearly.
- Use this repository to prove timely, reliable upstream packaging and build
  toward contributing release updates to Alpine aports. Future aports merge
  requests run alongside our release path, using the same verified inputs.
- Original LibOps work is MIT. Third-party code, notices and corresponding-source
  obligations retain their original licenses.

The [CI standards](ci-standards.md), [consumer contract](consuming.md),
[maintainer responsibilities](maintainers.md), and [sponsorship scope](sponsorship.md)
contain operational details. Keep only unfinished tasks in the [roadmap](roadmap.md).

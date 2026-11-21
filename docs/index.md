# Packages

Supported upstream software, packaged sooner. Install our APKs into your own
Alpine images without compiling PHP, Nginx, Python, Ruby, Go or Java in every build.

## Quick start: install PHP

<!-- package-quickstart:start -->
`ghcr.io/libops/php:8.4.28-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.
<!-- package-quickstart:end -->

Mount the carrier's `/packages` directory, then run `/packages/php84`.
It installs CLI, FPM, OPcache and their dependencies. Optional extensions have
entrypoints such as `/packages/php84-mbstring`. Java and other software use the
same pattern with their native package names. Carriers are package sources,
not runtime images. Never use `--allow-untrusted`.

[Consume packages](consuming.md) covers digest lookup, Bake and Renovate.
[Security metadata](security-metadata.md) explains final-image scanning.
[Contribute](guides/add-package.md), [maintain a target](maintainers.md), or
[sponsor package, OS or CPU support](sponsorship.md).

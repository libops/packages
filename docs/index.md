# Packages

Supported upstream software, packaged sooner. Install our APKs into your own
Alpine images without compiling PHP, Nginx, Python, Ruby, Go or Java in every build.

## Quick start

Choose a package for **Alpine 3.24**. Each example pins its carrier and base image
by digest. Use [Renovate](consuming.md#renovate) to keep both updated.

<!-- package-quickstart:start -->
=== "Go 1.26"

    `ghcr.io/libops/go:1.26.11-r4-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Go 1.27"

    `ghcr.io/libops/go:1.27.4-r4-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "ImageMagick 7"

    `ghcr.io/libops/imagemagick:7.1.2.25-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Java 21"

    `ghcr.io/libops/java:21.0.13.1_p2-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Java 25"

    `ghcr.io/libops/java:25.0.5.1_p2-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Leptonica 1"

    `ghcr.io/libops/leptonica:1.87.0-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Nginx 1.30"

    `ghcr.io/libops/nginx:1.30.7-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "PHP 8.4"

    `ghcr.io/libops/php:8.4.28-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "PHP 8.5"

    `ghcr.io/libops/php:8.5.13-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "PHP 8.6"

    `ghcr.io/libops/php:8.6.0-r4-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Python 3.12"

    `ghcr.io/libops/python:3.12.16-r4-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Python 3.13"

    `ghcr.io/libops/python:3.13.18-r4-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Python 3.14"

    `ghcr.io/libops/python:3.14.10-r4-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Ruby 3.3"

    `ghcr.io/libops/ruby:3.3.14-r7-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Ruby 3.4"

    `ghcr.io/libops/ruby:3.4.13-r7-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

=== "Ruby 4.0"

    `ghcr.io/libops/ruby:4.0.10-r7-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

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

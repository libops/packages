# Package reference

Generated from the catalog and registry when documentation is deployed. Each digest pins a multiarchitecture carrier, not a runtime image. Match the Alpine release. Publication can precede tests; check the [required checks](https://github.com/libops/packages/actions/workflows/push.yml) before adopting a release.

Mount `/packages` from the selected carrier at `/packages`, then call its installer. It copies verification keys and installs the exact APKs and carrier dependencies. PHP defaults include CLI, FPM and OPcache; other defaults follow the native package dependencies. Development files, docs and optional extensions have separate named installers.

## Keep images updated

[Renovate](consuming.md#renovate) updates the Dockerfile tag and digest together. Keep updates within your chosen software line and Alpine release; review migrations explicitly.

## go 1.26 (Alpine 3.24)

`ghcr.io/libops/go:1.26.11-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## go 1.27 (Alpine 3.24)

`ghcr.io/libops/go:1.27.4-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## imagemagick 7 (Alpine 3.24)

`ghcr.io/libops/imagemagick:7.1.2.25-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## java 21 (Alpine 3.24)

`ghcr.io/libops/java:21.0.13.1_p2-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## java 25 (Alpine 3.24)

`ghcr.io/libops/java:25.0.5.1_p2-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## leptonica 1 (Alpine 3.24)

`ghcr.io/libops/leptonica:1.87.0-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## nginx 1.30 (Alpine 3.24)

`ghcr.io/libops/nginx:1.30.7-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## php 8.4 (Alpine 3.24)

`ghcr.io/libops/php:8.4.28-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## php 8.5 (Alpine 3.24)

`ghcr.io/libops/php:8.5.13-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## php 8.6 (Alpine 3.24)

`ghcr.io/libops/php:8.6.0-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## python 3.12 (Alpine 3.24)

`ghcr.io/libops/python:3.12.16-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## python 3.13 (Alpine 3.24)

`ghcr.io/libops/python:3.13.18-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## python 3.14 (Alpine 3.24)

`ghcr.io/libops/python:3.14.10-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## ruby 3.3 (Alpine 3.24)

`ghcr.io/libops/ruby:3.3.14-r6-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## ruby 3.4 (Alpine 3.24)

`ghcr.io/libops/ruby:3.4.13-r6-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## ruby 4.0 (Alpine 3.24)

`ghcr.io/libops/ruby:4.0.10-r6-alpine3.24` is not publicly available yet. No verified install command is available for this revision.


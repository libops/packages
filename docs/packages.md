# Package reference

Generated from the catalog and registry when documentation is deployed. Each digest pins a multiarchitecture carrier, not a runtime image. Match the Alpine release. Publication can precede tests; check the [required checks](https://github.com/libops/packages/actions/workflows/push.yml) before adopting a release.

Mount `/packages` from the selected carrier at `/packages`, then call its installer. It copies verification keys and installs the exact APKs and carrier dependencies. PHP defaults include CLI, FPM and OPcache; other defaults follow the native package dependencies. Development files, docs and optional extensions have separate named installers.

## Keep images updated

[Renovate](consuming.md#renovate) updates the Dockerfile tag and digest together. Keep updates within your chosen software line and Alpine release; review migrations explicitly.

## Go 1.26 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/go:1.26.11-r3-alpine3.24@sha256:335d2b96ccbb42cbaefc7a7ddccdeb6765de417f45098d2ab32123c5ac9050fc AS go_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=go_packages,source=/packages,target=/packages \
    /packages/go126
```

## Go 1.27 (Alpine 3.24)

`ghcr.io/libops/go:1.27.4-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## ImageMagick 7 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/imagemagick:7.1.2.25-r2-alpine3.24@sha256:4d2aedd662aa122d1df262621bc4dcdacc265b0ae19ed79fa6d3b84a2ac3575a AS imagemagick_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=imagemagick_packages,source=/packages,target=/packages \
    /packages/imagemagick
```

## Java 21 (Alpine 3.24)

`ghcr.io/libops/java:21.0.13.1_p2-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## Java 25 (Alpine 3.24)

`ghcr.io/libops/java:25.0.5.1_p2-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## Leptonica 1 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/leptonica:1.87.0-r2-alpine3.24@sha256:2cbc2247a9896d64fe78dfb1c11b5fb37f6ce7a55e365bf8ffba9adc5c49c319 AS leptonica_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=leptonica_packages,source=/packages,target=/packages \
    /packages/leptonica
```

## Nginx 1.30 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/nginx:1.30.7-r2-alpine3.24@sha256:dd5c93ab7dc1023b6c13f3fc15e9f9c0570e319fd81b3f6acdde16ce3a73bf33 AS nginx_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=nginx_packages,source=/packages,target=/packages \
    /packages/nginx
```

## PHP 8.4 (Alpine 3.24)

`ghcr.io/libops/php:8.4.28-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## PHP 8.5 (Alpine 3.24)

`ghcr.io/libops/php:8.5.13-r2-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## PHP 8.6 (Alpine 3.24)

`ghcr.io/libops/php:8.6.0-r3-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## Python 3.12 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/python:3.12.16-r3-alpine3.24@sha256:a676dcf246470877c1a98960bb097d34f6e8826119ed9da4f0c18068238d48c5 AS python_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=python_packages,source=/packages,target=/packages \
    /packages/python312
```

## Python 3.13 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/python:3.13.18-r3-alpine3.24@sha256:6065bd0b0541084091804d42a4562d506710373526730d9e8778c858c4fdbb9b AS python_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=python_packages,source=/packages,target=/packages \
    /packages/python313
```

## Python 3.14 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/python:3.14.10-r3-alpine3.24@sha256:929b2d767aa5f58334ea483445434e2d252e33e182a35ee2cba969e32b4e4f81 AS python_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=python_packages,source=/packages,target=/packages \
    /packages/python314
```

## Ruby 3.3 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/ruby:3.3.14-r6-alpine3.24@sha256:a4e7e87605f0a39283bc01522571cdc812a120f054897e7e1cbe826ffebeab3d AS ruby_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=ruby_packages,source=/packages,target=/packages \
    /packages/ruby33
```

## Ruby 3.4 (Alpine 3.24)

`ghcr.io/libops/ruby:3.4.13-r6-alpine3.24` is not publicly available yet. No verified install command is available for this revision.

## Ruby 4.0 (Alpine 3.24)

```dockerfile title="Dockerfile"
FROM ghcr.io/libops/ruby:4.0.10-r6-alpine3.24@sha256:263a91b1d73eee7123b2412118282e72d00b280bda7466714aa9e1763135f151 AS ruby_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=ruby_packages,source=/packages,target=/packages \
    /packages/ruby40
```


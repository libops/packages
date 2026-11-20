# Packages

Supported upstream software, packaged sooner. Install our APKs into your own
Alpine images without compiling PHP, Nginx, Python, Ruby, Go or Java in every build.

## Quick start: install PHP

Replace `<verified-manifest-digest>` with the published carrier digest. Use the
matching Alpine release and keep both image pins updated with Renovate.

```dockerfile
FROM ghcr.io/libops/php:8.4.28-r1-alpine3.24@sha256:<verified-manifest-digest> AS php_packages
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

RUN --mount=type=bind,from=php_packages,source=/packages,target=/packages \
    --mount=type=bind,from=php_packages,source=/keys,target=/package-keys \
    cp /package-keys/*.pub /etc/apk/keys/ && \
    apk add --no-cache --repository /packages/APKINDEX.tar.gz \
      /packages/php84-[0-9]*.apk \
      /packages/php84-common-*.apk \
      /packages/php84-fpm-*.apk \
      /packages/php84-opcache-*.apk

RUN php84 --version
```

The carrier is a package source, not a runtime image. Only installed files and
public verification keys remain in your image. Never use `--allow-untrusted`.
Initial registry publication still needs verification; this is a template.

[Consume packages](consuming.md) covers digest lookup, Bake and Renovate.
[Security metadata](security-metadata.md) explains final-image scanning.
[Contribute](guides/add-package.md), [maintain a target](maintainers.md), or
[sponsor package, OS or CPU support](sponsorship.md).

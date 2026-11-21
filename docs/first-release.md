# First release

## Local checks

```sh
make dev-check
make docs-build
make build TARGET=alpine/3.24/nginx/1.30
make test TARGET=alpine/3.24/nginx/1.30
```

Local build/test requires Docker/Buildx, Make and the configured Go toolchain,
but no publishing credentials. Bootstrap builds the local helper until a real
utility pin exists. Run every enabled native architecture before advertising support.

## GitHub setup

- Install lbops-renovate; add `GH_APP_ID` and `GH_APP_PRIV_KEY` as Actions secrets.
  Grant repository contents/PR/issues permissions required by bot tasks and workflow
  write access for workflow updates. Keep app tokens repository-scoped and short-lived.
- Enable Actions/package publishing for the new `ghcr.io/libops/*` carrier
  packages, verify repository access, and make intended packages public.
  Set optional `repository` variable for another registry and configure Docker auth
  or `REGISTRY_USER`/`REGISTRY_PASSWORD` secrets.
- Renovate runs on GitHub-hosted Ubuntu with Docker. Set `RENOVATE_RUNNER`
  to a trusted dedicated runner label if desired. Require **Required checks**
  for merging; verify trusted and fork PRs and same-PR cancellation.
- Publish the utility, review its digest/base pin PR, then verify carrier architecture
  manifests, provenance, separate tests and a real consumer installation.
- Exercise cleanup on isolated PR fixtures with package delete permission.
- Set **Pages → Source → GitHub Actions** and verify docs deployment.

Only verified new revisions may publish; bump `pkgrel` for rebuilds. Consult
[current limits](implementation.md#verification-limits) and the [roadmap](roadmap.md).

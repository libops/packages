# Documentation

Zensical builds in a pinned container with hash-locked Python dependencies.

```sh
make docs-packages # refresh reference with live registry digests
make docs-build   # strict build, exports site/
make docs-serve   # local preview on :8000; local Docker bind mounts required
```

The `Packages` workflow builds and deploys documentation only on main, after all
package build/test jobs finish and the required checks pass. It checks out the
same commit as the packages, then runs `make docs-packages` before `make docs-build`.
A Go helper generates the package reference and homepage's Alpine 3.24 install
tabs from catalog revisions and verified registry digests. Missing/private revisions
get no fabricated digest or install command; registry outages fail the refresh.
CI deploys generated output without committing it back to main.

Enable **Pages → Source → GitHub Actions**. Documentation is published at
<https://libops.github.io/packages/>. Content tabs, highlighted Dockerfile blocks
and copy buttons are configured in `zensical.toml`.

Keep user instructions short. Put each contract in one guide and link to it.
The roadmap lists only unfinished work; delete completed entries. `AGENTS.md`
points agents to these docs and `CLAUDE.md` symlinks to that entrypoint.

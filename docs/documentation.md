# Documentation

Zensical builds in a pinned container with hash-locked Python dependencies.

```sh
make docs-packages # refresh reference with live registry digests
make docs-build   # strict build, exports site/
make docs-serve   # local preview on :8000; local Docker bind mounts required
```

The docs workflow validates PRs, deploys main to GitHub Pages, and refreshes after
a successful main package workflow. A Go helper generates `docs/packages.md` and
the homepage quick start from current catalog revisions and registry manifests.
Missing/private revisions get no fabricated digest or install command. Registry
outages fail refresh rather than publishing misleading pins. The generated
reference is refreshed in the Pages build; CI does not commit it back to main. Enable the
repository's **Pages → Source → GitHub Actions** setting. The expected URL is
<https://libops.github.io/packages/>.

Keep user instructions short. Put each contract in one guide and link to it.
The roadmap lists only unfinished work; delete completed entries. `AGENTS.md`
points agents to these docs and `CLAUDE.md` symlinks to that entrypoint.

# Documentation

Zensical builds in a pinned container with hash-locked Python dependencies.

```sh
make docs-build   # strict build, exports site/
make docs-serve   # local preview on :8000; local Docker bind mounts required
```

The docs workflow validates PRs and deploys main to GitHub Pages. Enable the
repository's **Pages → Source → GitHub Actions** setting. The expected URL is
<https://libops.github.io/packages/>; deployment still needs live verification.

Keep user instructions short. Put each contract in one guide and link to it.
The roadmap lists only unfinished work; delete completed entries. `AGENTS.md`
points agents to these docs and `CLAUDE.md` symlinks to that entrypoint.

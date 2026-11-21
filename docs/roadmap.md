# Roadmap

Only unfinished work belongs here. Operating contracts live in the linked guides;
[current limits](implementation.md#verification-limits) belong in implementation docs.

## MVP blockers

- Verify every current recipe's build, upstream suite and installation on native
  ARM64 and AMD64. Keep unexplained failures red. Restore Python 3.15 only after upstream resolves
  its small-stack lazy-import crash; retain a native regression before enabling it.
  Resolve the documented Python 3.14 deleted-libpython exclusion. Confirm the Go temporary-directory/Git HTTP fixes pass its suite.
- Establish official Java source/docs provenance before re-enabling updates.
  Review imported patches, licenses and corresponding-source/notices for all targets.
  Clarify aports recipe/patch reuse terms where no explicit file license is present;
  attribution alone is not a license grant.
- Add independent publisher authentication for remaining checksum-only sources.
- Verify live GitHub/GHCR publication, immutable revisions, fork checks, builder pin
  PR and manifests/provenance.
- Exercise closed-PR/candidate cleanup and cache quarantine/GC with isolated live
  GHCR fixtures and concurrent publishers.
- Verify failed-main manifest-only retries versus full reruns, including recovery of
  successful architecture candidates across run attempts; failed runs retain candidates.

## Next platforms and operations

- Add native Debian/Ubuntu builders/tests independently of APK; Alpine 3.23 needs
  its own verified base/bootstrap/recipes. Validate a real nonempty dependency DAG.
- Scope sponsored CPU additions by native hardware, bootstrap, owners and OS/package
  matrix; verify before enabling a profile. Test other registries' publication/auth
  and provider-specific cleanup rather than claiming universal GC.
- Create actual ownership teams and enforce reviewed CODEOWNERS. Agree on CLA versus
  DCO, text, maintained GitHub integration, signature storage/versioning, bot/fork
  behavior and required status. GitHub has no native CLA switch.
- Agree sponsorship amounts, larger OS tiers and continuing commitments. Design
  independent scheduling/registry replication, digest preservation and recovery.
- Verify app permissions/secrets, trusted Renovate runner, external scheduler if
  needed, Pages deployment and isle-buildkit consumer digest updates.
- Add trustworthy advisory classification and an optional per-target release index
  without delaying publication. Verify origin/version detection in final consumer images.
- Possible future work, outside today's Docker-only scope: if non-Docker consumers
  need a hosted APK repository, export existing APKs/indexes without rebuilding.
  Preserve names, versions and bytes; define immutable release URLs, independent
  trust-key distribution/rotation, hosting, retention and ownership. Support Docker
  carriers and HTTP together during an opt-in transition, with a minimal consumer
  URI/key configuration change. Do not introduce a server before there is demand.

- Before enabling production scheduling, measure weekly 32-build runner cost,
  configure the maintaining team's repository issue notifications/integration,
  and exercise the scheduled-failure issue workflow. Add an external stale-run
  monitor/dispatcher so schedule inactivity or GitHub outages cannot go unnoticed.

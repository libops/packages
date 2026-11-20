# Pipeline tooling evaluation

Evaluated 2026-11-20. Recommendation: keep native GitHub Actions scheduling,
Docker BuildKit execution and the small standard-library Go CLI for now.
This recommendation can change when measured bottlenecks justify a new engine.

| Option | Fit | Tradeoff |
| --- | --- | --- |
| Actions + BuildKit + Go | Keeps individual checks, target dependency jobs, existing runner setup and the same local Make commands | Must generate native Actions job definitions ahead of submission |
| Dagger Go SDK | Typed container operations, caching, and local/CI execution in Go | Adds its engine and SDK; GitHub check aggregation, changed-target selection, registry retention and trust policy still need integration |
| Earthly | BuildKit-based target builds and caching | Adds an Earthfile/runner model alongside the already implemented Dockerfile/Bake path |
| Buildkite dynamic pipelines | Adds steps at runtime with native dependency scheduling | A CI platform change rather than a small SDK adoption |

Dagger would be the first SDK to prototype if container orchestration becomes the
main source of duplicated logic. Prototype one target and measure cold/warm build
latency, source-build reuse between publication/tests, multiarchitecture runners,
and per-target GitHub checks before committing to a migration. Do not wrap the
existing pipeline in a second scheduler simply to use an SDK.

Buildkite becomes relevant if Actions' static job graph is the limiting factor.
Earthly deserves consideration if replacing Bake/recipe build orchestration rather
than adding a new layer. None of these tools creates upstream source checksums,
eliminates native packaging work, or supplies provider-independent registry GC.

## Primary sources

- [Dagger Go SDK](https://docs.dagger.io/reference/sdks/go/)
- [Dagger engine and SDK overview](https://github.com/dagger/dagger)
- [Earthly configuration](https://docs.earthly.dev/docs/earthly-config)
- [Buildkite dynamic pipelines](https://buildkite.com/docs/pipelines/configure/dynamic-pipelines)
- [Buildkite dependency scheduling](https://buildkite.com/docs/pipelines/configure/depends-on)

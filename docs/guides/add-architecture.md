# Add a CPU architecture

Architectures are explicit target opt-ins. New CPU support needs native hardware,
bootstrap compilers, upstream tests and ongoing ownership; it is a separate
[sponsorship opportunity](../sponsorship.md).

1. Add an `architectures.json` profile with OCI platform and runner labels. Use
   native runners; emulation alone does not establish release support.
2. Verify the base and utility images, source/compiler bootstrap, recipe architecture
   declarations and dependencies support the CPU. Children require parent support.
3. Add the architecture to the intended `packages.json` entries only. Existing
   targets must not gain architectures implicitly.
4. Run `make dev-generate`, `make dev-check`, local `builder-build`, `build` and
   `test` with `ARCH=<profile>`. Check native APK architecture, installation and
   manifest platform descriptors, then publish/review the real utility digest pin.

Workflow matrices and manifest assembly derive from profiles and target opt-ins;
no separate hard-coded CPU list belongs in workflow jobs. Consumer pins should
identify the multiarchitecture index containing their platform.

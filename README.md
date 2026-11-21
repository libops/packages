# Packages

Get supported upstream releases into public-facing services sooner. We build
reusable packages directly from official upstream source, reducing the wait
between a security release and a deployable update.

Our scope is internet-facing services and the runtimes and libraries that handle
their untrusted requests and content. We package upstream-supported releases
only. We do not backport fixes or extend upstream support lifetimes.

| Software | Release lines |
| --- | --- |
| Nginx | 1.30 stable |
| Python | 3.12, 3.13, 3.14 |
| PHP | 8.4, 8.5, 8.6 |
| OpenJDK | 21, 25 |
| Ruby | 3.3, 3.4, 4.0 |
| Go | 1.26, 1.27 |
| ImageMagick / Leptonica | 7 / 1 |

Alpine APKs are the first supported format, delivered in versioned container
images containing packages and public signing keys. Debian packages are planned;
RPM support may follow.

Speed is part of the contract: build only changed packages and their dependents,
publish each build immediately, and run tests separately. Dependents wait for
package publication, not tests. Package formats run independently: APK never
waits for DEB, or vice versa. A combined test status gates merging.

[Sponsor the project](https://github.com/sponsors/libops) to help fund independent
build infrastructure and registry hosting. Today, our release pipeline and image
distribution depend on GitHub. Sponsorship would help us reduce that dependency
and keep security updates available through outages. You can also sponsor a new
package on existing OS targets, or make a substantially larger commitment to add
an OS. Additional CPU architectures are another sponsorship opportunity, including native
build hardware and ongoing testing. All support commitments fund continuing
maintenance, not just the initial build.
Maintainers can also take responsibility for a package or OS under our release
contract.
[Discuss package or OS sponsorship](https://github.com/libops/packages/issues/new?template=sponsorship.yml)
and see our [support commitments](https://libops.github.io/packages/sponsorship/).

[Documentation](https://libops.github.io/packages/)

Project code and documentation are [MIT licensed](LICENSE). Packaged software retains its upstream license.

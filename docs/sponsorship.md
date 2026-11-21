# Sponsorship

[Sponsor LibOps](https://github.com/sponsors/libops) to fund faster updates,
build capacity and independent infrastructure/registry hosting during outages.

| Scope | Commitment |
| --- | --- |
| Shared infrastructure | Runners, independent scheduling, registry, recovery |
| New package | Smaller recurring tier on agreed existing OS targets |
| New CPU architecture | Separately scoped native hardware, bootstrap and ongoing tests |
| New OS | Substantially larger recurring tier plus introduction cost; maintains another platform across the catalog |

Price for indefinite maintenance within upstream support lifetimes, not a one-time
build. New OS releases/architectures are additional scope. No backports, exclusive
packages or unagreed response-time guarantees. Dollar amounts/live tiers remain unset.

[Open a sponsorship request](https://github.com/libops/packages/issues/new?template=sponsorship.yml)
to agree on targets, lines, architectures, owners, recurring funding and handoff.
Volunteers can instead [maintain a target](maintainers.md); issue forms ask whether
you can maintain, sponsor, both or only request support. Acceptance requires review.
Independent hosting is a funding goal, not a capability already delivered.

A future non-Docker consumer could sponsor static APK repository hosting. It would
export the same packages and signed indexes from carriers; scope storage, signing,
retention and operations when there is a real consumer. Docker carriers are today’s
scope. If HTTP distribution is added, fund parallel
Docker/HTTP availability and a minimal, opt-in consumer transition; see the
[roadmap](roadmap.md). No repository service is implemented now.

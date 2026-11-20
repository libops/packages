# Maintainers and ownership

Maintainers can own an agreed OS or package scope as part of the release contract.
Use its issue form to offer maintenance, sponsorship or both. Record owners,
release lines, architectures, lifecycle, tests and handoff before accepting support.

Package owners maintain their recipes, source verification, updates and tests on
agreed existing OS targets. OS owners maintain platform tooling and compatibility.
Both follow [CI standards](ci-standards.md); ownership must not serialize independent
builds. Review abandoned targets instead of silently advertising support.

## CODEOWNERS

The example `.github/CODEOWNERS` uses placeholder OS/package teams. Replace them
with visible teams having explicit repository write access before enforcement.
Both teams must be on the same recipe rule: the last matching rule wins, and
GitHub accepts either listed owner's approval, not one from every listed team.
Protect shared tooling with repository maintainers.

A contribution agreement policy/integration remains [roadmap work](roadmap.md).
GitHub has no built-in repository CLA switch; contributions remain MIT licensed
under the project policy while third-party licenses stay intact.

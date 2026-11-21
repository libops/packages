# License

LibOps original code and documentation are MIT licensed, copyright 2026 LibOps.
Read the [full license](https://github.com/libops/packages/blob/main/LICENSE).

Third-party recipes, patches, dependencies and packaged software keep their own
licenses/notices. The project license does not relicense them. Review each
version's distribution requirements before publishing. Carriers include native
corresponding-source bundles at `/sources`, with hashes in release metadata;
required upstream notices and target-specific compliance still need verification.
The utility image includes vendored Go dependency license/notice files under
`/usr/share/licenses/libops-packages/dependencies`.

Packaging recipes and compatibility work build on Alpine Linux's aports
contributors. See the project-wide [attribution](https://github.com/libops/packages/blob/main/NOTICE).
The reviewed aports tree has no repository-wide license declaration; an APKBUILD's
`license` field describes its packaged software. Do not label imported aports work
MIT solely because our original code is MIT. Preserve file-specific notices and
verify unclear reuse terms. Imported compatibility patches are pinned in recipes;
Carriers include `LICENSE` and `NOTICE` in `/packages` and `/licenses/libops-packages`.
Installers copy them into `/usr/share/licenses/libops-packages` in consumer images.
Ruby APKs and corresponding-source bundles also include the general attribution.

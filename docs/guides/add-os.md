# Add an OS or package repository

An OS repository is a directory such as `packages/alpine/3.24`, not a Git branch.
Only Alpine APK execution exists today; Debian/Ubuntu need native adapters.

1. Verify distro/software support and compiler bootstrap compatibility.
2. Add `base-image.txt` with the exact release tag and real multiarchitecture digest.
3. Add one [package](add-package.md) and its inventory entry. Dependencies stay
   within that distro release; APK and DEB graphs cannot depend on each other.
4. Run `make dev-generate`, `make dev-check`, `make builder-list`, then locally:

   ```sh
   make builder-build TARGET=alpine/3.24 ARCH=arm64
   make build TARGET=alpine/3.24/nginx/1.30 ARCH=arm64 LOCAL_BUILDER=1
   make test TARGET=alpine/3.24/nginx/1.30 ARCH=arm64 LOCAL_BUILDER=1
   ```

5. Validate all enabled architectures, installation/ABI, tests and dependency contexts.
   Publish the independent utility image and review its real `builder.json` digest/base
   pin. Bootstrap uses the local utility until that pin exists.

A new packaging format also needs native package production/verification, source
bundles, metadata, install tests, dependency resolution and format-specific source
validation. Do not register placeholders as working targets.

Assign an OS owner and fund the ongoing matrix. [OS sponsorship](../sponsorship.md)
is substantially larger than adding a package on an existing OS.

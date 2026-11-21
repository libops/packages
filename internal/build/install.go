package build

import (
	"context"
	"fmt"
	"github.com/libops/packages/internal/catalog"
	"strings"
)

// TestInstall uses the production consumer Dockerfile against an older carrier
// package and a carrier-only subpackage. It needs Docker but no registry writes.
func TestInstall(ctx context.Context) error {
	c, err := catalog.Load(".")
	if err != nil {
		return err
	}
	base, err := installBase(".", c)
	if err != nil {
		return err
	}
	utility := "packages-install-utility:local"
	if err := run(ctx, nil, "docker", "buildx", "build", "--builder", "default", "--load", "--target", "utility", "--build-arg", "BASE_IMAGE="+base, "-t", utility, "-f", "Dockerfile", "."); err != nil {
		return err
	}
	image := "packages-install-fixture:local"
	if err := run(ctx, nil, "docker", "buildx", "build", "--builder", "default", "--load", "--build-context", "utility=docker-image://"+utility, "--build-arg", "BASE_IMAGE="+base, "-t", image, "-f", "ci/install-fixture/Dockerfile", "."); err != nil {
		return err
	}
	// Exact root package version is checked below, including its checksum world pin.
	if err := run(ctx, nil, "docker", "buildx", "build", "--builder", "default", "--load", "--no-cache", "--build-context", "packages=docker-image://"+image, "--build-arg", "BASE_IMAGE="+base, "--build-arg", "PACKAGE=nginx", "-t", "packages-install-consumer:local", "-f", "test.Dockerfile", "."); err != nil {
		return err
	}
	if err := run(ctx, nil, "docker", "run", "--rm", "packages-install-consumer:local", "sh", "-ec", `apk info --exists nginx=0.0.1-r0 nginx-common=0.0.1-r0; test "$(cat /usr/share/libops-fixture/common)" = carrier`); err != nil {
		return fmt.Errorf("carrier lost to distro package: %w", err)
	}
	return nil
}

// installBase deliberately has no dependency on any supported software line.
func installBase(root string, c catalog.Catalog) (string, error) {
	for _, name := range c.Names() {
		if strings.HasPrefix(c[name].Target, "alpine/3.24/") {
			target, err := c.Metadata(root, name)
			if err != nil {
				return "", err
			}
			return target.Base, nil
		}
	}
	return "", fmt.Errorf("installation regression requires an Alpine 3.24 target")
}

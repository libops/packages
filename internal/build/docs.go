package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/libops/packages/internal/catalog"
	"github.com/libops/packages/internal/config"
)

// DocumentPackages resolves published revision tags at documentation build time.
// It never invents digests or substitutes a different version on lookup failure.
func DocumentPackages(ctx context.Context, cfg config.Environment, root string) error {
	c, err := catalog.Load(root)
	if err != nil {
		return err
	}
	repository := cfg.Get("REPOSITORY")
	if repository == "" {
		repository = "ghcr.io/libops"
	}
	page, quick, err := packagePages(root, c, repository, func(ref string) (string, error) {
		digest, err := ReleaseDigest(ctx, cfg, ref)
		var status *transport.Error
		// GHCR also returns 403 for a package that has never been published. This
		// display-only result is not used by release existence or publishing checks.
		if errors.As(err, &status) && status.StatusCode == 403 {
			return "", nil
		}
		return digest, err
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "docs/packages.md"), []byte(page), 0644); err != nil {
		return err
	}
	indexPath := filepath.Join(root, "docs/index.md")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	startMarker, endMarker := "<!-- package-quickstart:start -->", "<!-- package-quickstart:end -->"
	before, rest, found := strings.Cut(string(index), startMarker)
	if !found {
		return fmt.Errorf("missing homepage quick-start marker")
	}
	_, after, found := strings.Cut(rest, endMarker)
	if !found {
		return fmt.Errorf("missing homepage quick-start end marker")
	}
	return os.WriteFile(indexPath, []byte(before+startMarker+"\n"+quick+endMarker+after), 0644)
}

func packagePages(root string, c catalog.Catalog, repository string, resolve func(string) (string, error)) (string, string, error) {
	var out, tabs strings.Builder
	out.WriteString("# Package reference\n\nGenerated from the catalog and registry when documentation is deployed. Each digest pins a multiarchitecture carrier, not a runtime image. Match the Alpine release. Publication can precede tests; check the [required checks](https://github.com/libops/packages/actions/workflows/push.yml) before adopting a release.\n\nMount `/packages` from the selected carrier at `/packages`, then call its installer. It copies verification keys and installs the exact APKs and carrier dependencies. PHP defaults include CLI, FPM and OPcache; other defaults follow the native package dependencies. Development files, docs and optional extensions have separate named installers.\n\n## Keep images updated\n\n[Renovate](consuming.md#renovate) updates the Dockerfile tag and digest together. Keep updates within your chosen software line and Alpine release; review migrations explicitly.\n\n")
	for _, name := range c.Names() {
		p, err := c.Metadata(root, name)
		if err != nil {
			return "", "", err
		}
		ref := repository + "/" + p.Image + ":" + p.RevisionTag
		digest, err := resolve(ref)
		if err != nil {
			return "", "", err
		}
		label := strings.ToUpper(p.Image[:1]) + p.Image[1:]
		switch p.Image {
		case "php":
			label = "PHP"
		case "imagemagick":
			label = "ImageMagick"
		}
		label += " " + p.Line
		fmt.Fprintf(&out, "## %s (Alpine %s)\n\n", label, strings.Split(p.Target, "/")[1])
		var example string
		if digest == "" {
			example = fmt.Sprintf("`%s` is not publicly available yet. No verified install command is available for this revision.\n", ref)
		} else {
			example = fmt.Sprintf("```dockerfile title=\"Dockerfile\"\nFROM %s@%s AS %s_packages\nFROM %s\n\nRUN --mount=type=bind,from=%s_packages,source=/packages,target=/packages \\\n    /packages/%s\n```\n", ref, digest, p.Image, p.Base, p.Image, p.Test)
		}
		out.WriteString(example + "\n")
		if strings.HasPrefix(p.Target, "alpine/3.24/") {
			fmt.Fprintf(&tabs, "=== %q\n\n", label)
			for _, line := range strings.Split(strings.TrimSuffix(example, "\n"), "\n") {
				if line != "" {
					tabs.WriteString("    " + line)
				}
				tabs.WriteString("\n")
			}
			tabs.WriteString("\n")
		}
	}
	return out.String(), tabs.String(), nil
}

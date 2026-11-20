package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/catalog"
)

// Revision validation is offline, including fork/private/bootstrap PRs. Every
// release input or changed prerequisite needs a new package identity.
func validateRevisions(ctx context.Context, root, base string, c, previous catalog.Catalog, targets []string) error {
	for _, name := range targets {
		p := c[name]
		path := "packages/" + p.Target + "/APKBUILD"
		if old, ok := previous[name]; !ok || old.Target != p.Target {
			continue
		}
		cmd, stop := buildhelpers.CommandContext(ctx, time.Minute, "git", "show", base+":"+path)
		cmd.Dir = root
		old, err := cmd.Output()
		stop()
		if err != nil {
			return err
		}
		current, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if err := changedRevision(string(old), string(current)); err != nil {
			return fmt.Errorf("%s: %w", p.Target, err)
		}
	}
	return nil
}
func recipeIdentity(recipe string) (string, int, error) {
	version := regexp.MustCompile(`(?m)^pkgver=([^\s]+)$`).FindStringSubmatch(recipe)
	revision := regexp.MustCompile(`(?m)^pkgrel=([0-9]+)$`).FindStringSubmatch(recipe)
	if len(version) != 2 || len(revision) != 2 {
		return "", 0, fmt.Errorf("recipe requires literal pkgver and pkgrel")
	}
	n, err := strconv.Atoi(revision[1])
	return version[1], n, err
}
func changedRevision(old, current string) error {
	a, x, err := recipeIdentity(old)
	if err != nil {
		return err
	}
	b, y, err := recipeIdentity(current)
	if err != nil {
		return err
	}
	if a == b && y <= x {
		return fmt.Errorf("changed package inputs or prerequisite require a pkgrel bump (was %s-r%d)", a, x)
	}
	return nil
}

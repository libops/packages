package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/catalog"
)

// ReviseBase runs on Renovate's working tree. Compare with HEAD so rerunning a
// post-upgrade task is idempotent, including after Renovate commits the result.
func ReviseBase(ctx context.Context, root, base string) error {
	c, err := catalog.Load(root)
	if err != nil {
		return err
	}
	known := false
	for _, value := range c.Bases() {
		known = known || value == base
	}
	if !known {
		return fmt.Errorf("unknown OS release %q", base)
	}
	previous := func(path string) ([]byte, error) {
		cmd, stop := buildhelpers.CommandContext(ctx, time.Minute, "git", "show", "HEAD:"+path)
		defer stop()
		cmd.Dir = root
		return cmd.Output()
	}
	path := "packages/" + base + "/base-image.txt"
	before, err := previous(path)
	if err != nil {
		return err
	}
	after, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return err
	}
	if bytes.Equal(before, after) {
		return nil
	}
	for _, id := range c.Names() {
		p := c[id]
		if !strings.HasPrefix(p.Target, base+"/") {
			continue
		}
		recipe := "packages/" + p.Target + "/APKBUILD"
		old, err := previous(recipe)
		if err != nil {
			return err
		}
		current, err := os.ReadFile(filepath.Join(root, recipe))
		if err != nil {
			return err
		}
		revised, err := nextRevision(string(old), string(current))
		if err != nil {
			return fmt.Errorf("%s: %w", recipe, err)
		}
		if err := os.WriteFile(filepath.Join(root, recipe), []byte(revised), 0644); err != nil {
			return err
		}
	}
	return nil
}
func nextRevision(old, current string) (string, error) {
	version := regexp.MustCompile(`(?m)^pkgver=([^\s]+)$`)
	revision := regexp.MustCompile(`(?m)^pkgrel=([0-9]+)$`)
	a, b := version.FindStringSubmatch(old), version.FindStringSubmatch(current)
	x, y := revision.FindStringSubmatch(old), revision.FindStringSubmatch(current)
	if len(a) != 2 || len(b) != 2 || len(x) != 2 || len(y) != 2 {
		return "", fmt.Errorf("recipe needs literal pkgver and pkgrel")
	}
	if a[1] != b[1] {
		return current, nil
	} // An upstream update already creates a new version.
	n, err := strconv.Atoi(x[1])
	if err != nil {
		return "", err
	}
	m, err := strconv.Atoi(y[1])
	if err != nil {
		return "", err
	}
	if m > n {
		return current, nil
	}
	if n == int(^uint(0)>>1) {
		return "", fmt.Errorf("pkgrel overflow")
	}
	return revision.ReplaceAllString(current, "pkgrel="+strconv.Itoa(n+1)), nil
}

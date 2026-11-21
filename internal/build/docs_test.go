package build

import (
	"fmt"
	"strings"
	"testing"

	"github.com/libops/packages/internal/catalog"
)

func TestPackageReferenceUsesExactPublishedDigests(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	count := 0
	page, err := packagePage("../..", c, "ghcr.io/libops", func(ref string) (string, error) {
		count++
		if strings.Contains(ref, "/php:") {
			return "", nil
		}
		return digest, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != len(c) || !strings.Contains(page, "@"+digest) || !strings.Contains(page, "/packages/openjdk21") || !strings.Contains(page, "not publicly available") || strings.Contains(page, "<verified") || strings.Contains(page, "AS php_packages") {
		t.Fatal("reference omitted targets or invented an installable release")
	}
	_, err = packagePage("../..", c, "ghcr.io/libops", func(string) (string, error) { return "", fmt.Errorf("registry outage") })
	if err == nil {
		t.Fatal("registry outage hidden")
	}
}

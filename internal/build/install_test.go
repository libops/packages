package build

import (
	"github.com/libops/packages/internal/catalog"
	"testing"
)

func TestInstallBaseWithoutNginx(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range c {
		if p.Family == "nginx" {
			delete(c, name)
		}
	}
	base, err := installBase("../..", c)
	if err != nil || base == "" {
		t.Fatalf("base %q: %v", base, err)
	}
	if _, err := installBase("../..", catalog.Catalog{}); err == nil {
		t.Fatal("missing OS base accepted")
	}
}

package catalog

import (
	"github.com/libops/packages/buildhelpers"
	"reflect"
	"testing"
)

func graph() Catalog {
	c := Catalog{}
	for _, target := range []string{"alpine/3.24/php/8.4", "alpine/3.24/nginx/1.30", "alpine/3.23/php/8.4", "debian/13/php/8.4"} {
		id, _ := ID(target)
		line := "8.4"
		if target == "alpine/3.24/nginx/1.30" {
			line = "1.30"
		}
		c[id] = Package{Package: buildhelpers.Package{Target: target, Line: line, Upstream: "https://example.org/support"}}
	}
	p := c["alpine-3-24-nginx-1-30"]
	p.Dependencies = []string{"alpine-3-24-php-8-4"}
	c["alpine-3-24-nginx-1-30"] = p
	return c
}
func TestAffectedIsolationAndDescendants(t *testing.T) {
	c := graph()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		file string
		want []string
	}{
		{"README.md", []string{}}, {"internal/build/run.go", []string{}},
		{"packages/alpine/3.24/php/8.4/APKBUILD", []string{"alpine-3-24-nginx-1-30", "alpine-3-24-php-8-4"}},
		{"packages/alpine/3.24/base-image.txt", []string{"alpine-3-24-nginx-1-30", "alpine-3-24-php-8-4"}},
		{"packages/debian/13/php/8.4/control", []string{"debian-13-php-8-4"}},
		{"packages/alpine/3.23/php/8.4/APKBUILD", []string{"alpine-3-23-php-8-4"}},
	} {
		if got := c.Affected([]string{tc.file}, nil); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.file, got, tc.want)
		}
	}
	if got := c.Affected([]string{"packages.json"}, c); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestRejectInvalidDependencies(t *testing.T) {
	for _, dep := range []string{"alpine-3-23-php-8-4", "debian-13-php-8-4", "alpine-3-24-nginx-1-30", "missing"} {
		c := graph()
		p := c["alpine-3-24-php-8-4"]
		p.Dependencies = []string{dep}
		c["alpine-3-24-php-8-4"] = p
		if c.Validate() == nil {
			t.Errorf("accepted %s", dep)
		}
	}
	for _, target := range []string{"alpine/../php/8.4", "alpine/3.24/php/8.4;echo", "alpine/3.24/php/8.4/extra"} {
		if _, err := ID(target); err == nil {
			t.Fatal(target)
		}
	}
}

func TestMalformedParentFailsWithoutPanic(t *testing.T) {
	c := graph()
	p := c["alpine-3-24-php-8-4"]
	p.Target = "bad"
	c["alpine-3-24-php-8-4"] = p
	if c.Validate() == nil {
		t.Fatal("accepted malformed parent")
	}
}
func TestInventoryRenameAndRemovalSelectOnlyAffectedDescendants(t *testing.T) {
	previous := graph()
	current := graph()
	p := current["alpine-3-24-php-8-4"]
	delete(current, "alpine-3-24-php-8-4")
	p.Target = "alpine/3.24/php/8.5"
	p.Line = "8.5"
	current["alpine-3-24-php-8-5"] = p
	child := current["alpine-3-24-nginx-1-30"]
	child.Dependencies = []string{"alpine-3-24-php-8-5"}
	current["alpine-3-24-nginx-1-30"] = child
	if err := current.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{"alpine-3-24-nginx-1-30", "alpine-3-24-php-8-5"}
	if got := current.Affected([]string{"packages.json"}, previous); !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	delete(current, "alpine-3-24-php-8-5")
	child.Dependencies = nil
	current["alpine-3-24-nginx-1-30"] = child
	if got := current.Affected([]string{"packages.json"}, previous); !reflect.DeepEqual(got, []string{"alpine-3-24-nginx-1-30"}) {
		t.Fatal(got)
	}
}

func TestGeneratedAndIntakeChangesDoNotRebuildUnrelatedPackages(t *testing.T) {
	c := graph()
	previous := graph()
	delete(previous, "debian-13-php-8-4")
	metadata := []string{"renovate.json", ".github/workflows/push.yml", "ci/renovate-allowed-commands.json", ".github/ISSUE_TEMPLATE/package.yml", ".github/CODEOWNERS", ".github/workflows/cleanup.yml"}
	if got := c.Affected(metadata, nil); len(got) != 0 {
		t.Fatalf("metadata rebuilt packages: %v", got)
	}
	files := append(metadata, "packages.json", "packages/debian/13/php/8.4/control")
	if got := c.Affected(files, previous); !reflect.DeepEqual(got, []string{"debian-13-php-8-4"}) {
		t.Fatalf("adding one package selected unrelated work: %v", got)
	}
}

func TestTargetIDsCannotCollapseHyphenatedSoftware(t *testing.T) {
	for _, target := range []string{"alpine/3.24/php-8/4", "alpine/3.24/p4-php-8-4/1"} {
		if _, err := ID(target); err == nil {
			t.Fatal("ambiguous software segment", target)
		}
	}
}

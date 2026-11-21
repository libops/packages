package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArchitectureOptInAndBuilderUnion(t *testing.T) {
	root := t.TempDir()
	data := `{"amd64":{"platform":"linux/amd64","runner":["ubuntu-24.04"]},"armv7":{"platform":"linux/arm/v7","runner":["self-hosted","linux","arm"]},"riscv64":{"platform":"linux/riscv64","runner":["self-hosted","linux","riscv64"]}}`
	if err := os.WriteFile(filepath.Join(root, "architectures.json"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	c := graph()
	parent := "alpine-3-24-php-8-4"
	child := "alpine-3-24-nginx-1-30"
	p := c[parent]
	p.Architectures = []string{"amd64", "riscv64"}
	c[parent] = p
	p = c[child]
	p.Architectures = []string{"armv7"}
	c[child] = p
	platforms, err := c.Platforms(root, child)
	if err != nil {
		t.Fatal(err)
	}
	if len(platforms) != 1 || platforms[0].Platform != "linux/arm/v7" {
		t.Fatal("variant lost", platforms)
	}
	platforms, err = c.BuilderPlatforms(root, "alpine/3.24")
	if err != nil {
		t.Fatal(err)
	}
	if len(platforms) != 3 || platforms[2].Arch != "riscv64" {
		t.Fatal("builder union incorrect", platforms)
	}
	p = c[parent]
	p.Architectures = []string{"amd64", "unknown"}
	c[parent] = p
	if _, err = c.Platforms(root, parent); err == nil {
		t.Fatal("unknown architecture accepted")
	}
	p.Architectures = []string{"amd64", "amd64"}
	c[parent] = p
	if _, err = c.Platforms(root, parent); err == nil {
		t.Fatal("duplicate architecture accepted")
	}
}

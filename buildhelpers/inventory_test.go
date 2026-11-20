package buildhelpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryRejectsUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{"x":{"target":"alpine/3.24/php/8.4","supported_untill":"2028-12-31"}}`,
		`{"x":{"target":"alpine/3.24/php/8.4","update":{"allowd":"8.4"}}}`,
		`{} {}`,
	} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "packages.json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadInventory(root); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestPHPRecipesNameLibOpsPackager(t *testing.T) {
	for _, line := range []string{"8.4", "8.5", "8.6"} {
		path := filepath.Join("..", "packages", "alpine", "3.24", "php", line, "APKBUILD")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(text, "maintainer=") && text != `maintainer="LibOps <libops@users.noreply.github.com>"` {
				t.Fatalf("%s overrides LibOps packager: %s", path, text)
			}
		}
	}
}

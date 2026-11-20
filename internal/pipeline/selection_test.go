package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libops/packages/internal/catalog"
	"github.com/libops/packages/internal/config"
)

func TestHelperSelectionForcesCurrentBuilder(t *testing.T) {
	root := t.TempDir()
	// A private git fixture only; no mutation of the user's repository/config.
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	for _, file := range []string{"packages.json", "architectures.json", "packages/alpine/3.24/base-image.txt"} {
		data, err := os.ReadFile(filepath.Join("../..", file))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, file)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, data, 0644)
	}
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range c {
		path := filepath.Join(root, "packages", p.Target, "APKBUILD")
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte("pkgver=1.0.0\npkgrel=0\n"), 0644)
	}
	git("init", "--initial-branch=main")
	git("add", ".")
	git("commit", "-m", "fixture")
	base := git("rev-parse", "HEAD")
	missing := strings.Repeat("a", 40)
	rewritten, err := Select(context.Background(), config.Environment{"GITHUB_EVENT_NAME": "push", "GITHUB_REF_NAME": "main", "BEFORE": missing}, root)
	if err != nil || len(rewritten.Targets) != len(c) || !rewritten.LocalBuilder {
		t.Fatalf("rewritten push: %+v %v", rewritten, err)
	}
	if _, err := Select(context.Background(), config.Environment{"GITHUB_EVENT_NAME": "push", "BASE": missing}, root); err == nil {
		t.Fatal("invalid explicit comparison accepted")
	}

	// A fresh clone lacks the old, unreferenced push object but can recover it.
	detached := git("commit-tree", "HEAD^{tree}", "-m", "old force-pushed commit")
	clone := filepath.Join(t.TempDir(), "checkout")
	git("clone", "--no-local", root, clone)
	recovered, err := Select(context.Background(), config.Environment{"GITHUB_EVENT_NAME": "push", "GITHUB_REF_NAME": "main", "BEFORE": detached}, clone)
	if err != nil || len(recovered.Targets) != 0 {
		t.Fatalf("recover push comparison: %+v %v", recovered, err)
	}
	for _, tc := range []struct {
		file  string
		local bool
	}{{"README.md", false}, {"internal/buildapk/build.go", true}} {
		path := filepath.Join(root, tc.file)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte("changed\n"), 0644)
		git("add", tc.file)
		git("commit", "-m", "fixture change")
		selection, err := Select(context.Background(), config.Environment{"BASE": base, "PR_BASE": base}, root)
		if err != nil {
			t.Fatal(err)
		}
		if selection.LocalBuilder != tc.local {
			t.Fatalf("%s: helper mode=%v", tc.file, selection.LocalBuilder)
		}
		if tc.local && (len(selection.Targets) != 1 || c[selection.Targets[0]].Family != "nginx") {
			t.Fatal("helper change did not select packages")
		}
		main, err := Select(context.Background(), config.Environment{"BASE": base, "GITHUB_REF": "refs/heads/main"}, root)
		if err != nil || len(main.Targets) != 0 {
			t.Fatalf("helper-only merge selected package releases: %+v %v", main, err)
		}
		if !tc.local && len(selection.Targets) != 0 {
			t.Fatal("docs selected packages")
		}
	}
	// Recipe changes are rejected offline before any publishing credentials/network.
	path := filepath.Join(root, "packages/alpine/3.24/nginx/1.30/APKBUILD")
	os.WriteFile(path, []byte("pkgver=1.0.0\npkgrel=0\n# changed recipe\n"), 0644)
	git("add", ".")
	git("commit", "-m", "recipe edit without revision")
	if _, err := Select(context.Background(), config.Environment{"BASE": base, "PR_BASE": base, "CI": "true", "REPOSITORY": "127.0.0.1:1/private"}, root); err == nil {
		t.Fatal("recipe edit without pkgrel passed")
	}
	os.WriteFile(path, []byte("pkgver=1.0.0\npkgrel=1\n# changed recipe\n"), 0644)
	git("add", ".")
	git("commit", "-m", "revision bump")
	selection, err := Select(context.Background(), config.Environment{"BASE": base, "PR_BASE": base, "CI": "true", "REPOSITORY": "127.0.0.1:1/private"}, root)
	if err != nil || len(selection.Targets) != 1 {
		t.Fatalf("offline plan failed: %+v %v", selection, err)
	}

	// A released child also needs a new identity when its parent inputs change.
	childBase := git("rev-parse", "HEAD")
	child := c["alpine-3-24-nginx-1-30"]
	child.Dependencies = []string{"alpine-3-24-php-8-4"}
	c["alpine-3-24-nginx-1-30"] = child
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "packages.json"), data, 0644)
	os.WriteFile(filepath.Join(root, "packages/alpine/3.24/php/8.4/APKBUILD"), []byte("pkgver=1.0.1\npkgrel=0\n"), 0644)
	git("add", ".")
	git("commit", "-m", "parent update")
	if _, err := Select(context.Background(), config.Environment{"BASE": childBase, "PR_BASE": childBase}, root); err == nil {
		t.Fatal("child kept old revision after parent change")
	}
	os.WriteFile(path, []byte("pkgver=1.0.0\npkgrel=2\n"), 0644)
	git("add", ".")
	git("commit", "-m", "child rebuild revision")
	selection, err = Select(context.Background(), config.Environment{"BASE": childBase, "PR_BASE": childBase}, root)
	if err != nil || len(selection.Targets) != 2 {
		t.Fatalf("parent/child chain: %+v %v", selection, err)
	}

}

package pipeline

import (
	"context"
	"github.com/libops/packages/internal/catalog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseRevisionBump(t *testing.T) {
	for _, tc := range []struct{ old, current, want string }{
		{"pkgver=8.4.28\npkgrel=0\n", "pkgver=8.4.28\npkgrel=0\n", "pkgver=8.4.28\npkgrel=1\n"},
		{"pkgver=8.4.28\npkgrel=0\n", "pkgver=8.4.28\npkgrel=1\n", "pkgver=8.4.28\npkgrel=1\n"},
		{"pkgver=8.4.28\npkgrel=3\n", "pkgver=8.4.29\npkgrel=0\n", "pkgver=8.4.29\npkgrel=0\n"},
	} {
		got, err := nextRevision(tc.old, tc.current)
		if err != nil || got != tc.want {
			t.Fatalf("%q %v", got, err)
		}
	}
	if _, err := nextRevision("pkgrel=$variable", "pkgrel=1"); err == nil {
		t.Fatal("accepted dynamic fields")
	}
}

func TestBaseRevisionTaskIsGenerated(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	data, err := JSON(Renovate(c))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "make revise-base TARGET=alpine/3.24") {
		t.Fatal("base updates must revise packages")
	}
}

func TestReviseBaseWorkingTreeAndRerun(t *testing.T) {
	root := t.TempDir()
	write := func(path string, data []byte) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"packages.json", "architectures.json", "packages/alpine/3.24/base-image.txt"} {
		data, err := os.ReadFile(filepath.Join("../..", file))
		if err != nil {
			t.Fatal(err)
		}
		write(file, data)
	}
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range c {
		write("packages/"+p.Target+"/APKBUILD", []byte("pkgver=1.0.0\npkgrel=3\n"))
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, output)
		}
	}
	git("init", "--initial-branch=main")
	git("add", ".")
	git("commit", "-m", "fixture")
	ctx := context.Background()
	if err := ReviseBase(ctx, root, "alpine/3.24"); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		for _, p := range c {
			data, err := os.ReadFile(filepath.Join(root, "packages", p.Target, "APKBUILD"))
			if err != nil || !strings.Contains(string(data), "pkgrel="+want+"\n") {
				t.Fatalf("%s: %s %v", p.Target, data, err)
			}
		}
	}
	check("3") // Unchanged base is a no-op.
	write("packages/alpine/3.24/base-image.txt", []byte("alpine:3.24.9@sha256:"+strings.Repeat("a", 64)+"\n"))
	if err := ReviseBase(ctx, root, "alpine/3.24"); err != nil {
		t.Fatal(err)
	}
	check("4")
	if err := ReviseBase(ctx, root, "alpine/3.24"); err != nil {
		t.Fatal(err)
	}
	check("4")
	git("add", ".")
	git("commit", "-m", "base update")
	if err := ReviseBase(ctx, root, "alpine/3.24"); err != nil {
		t.Fatal(err)
	}
	check("4")
	if err := ReviseBase(ctx, root, "../../etc"); err == nil {
		t.Fatal("accepted unknown base")
	}
}

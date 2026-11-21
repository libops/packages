package buildhelpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerPinsCarrierDependencyClosure(t *testing.T) {
	dir := t.TempDir()
	for name, extra := range map[string]string{
		"php84":        "depend = php84-common=1-r0\ndepend = so:libtest.so.1\n",
		"php84-common": "depend = php84=1-r0\n", // cycles must terminate
		"php84-fpm":    "depend = php84\n", "php84-opcache": "depend = php84\n",
		"php84-libs": "provides = so:libtest.so.1=1\n", "php84-dev": "",
	} {
		data := tarStream(t, ".PKGINFO", "pkgname = "+name+"\npkgver = 1-r0\narch = aarch64\n"+extra)
		if err := os.WriteFile(filepath.Join(dir, name+"-1-r0.apk"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteInstallers(dir, "php84"); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	args := filepath.Join(t.TempDir(), "args")
	for name, script := range map[string]string{"mkdir": "#!/bin/sh\nexit 0\n", "cp": "#!/bin/sh\nexit 0\n", "apk": "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$TEST_ARGS\"\nexit \"${APK_EXIT:-0}\"\n"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_ARGS", args)
	command := func(arguments ...string) error {
		return exec.Command("sh", append([]string{filepath.Join(dir, "php84")}, arguments...)...).Run()
	}
	if err := command(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"php84", "php84-common", "php84-fpm", "php84-opcache", "php84-libs"} {
		if !strings.Contains(string(data), filepath.Join(dir, name+"-1-r0.apk")) {
			t.Fatalf("missing exact carrier dependency %s: %s", name, data)
		}
	}
	if strings.Contains(string(data), "php84-dev") {
		t.Fatal("default installs development files")
	}
	if err := command("--allow-untrusted"); err == nil {
		t.Fatal("unexpected argument accepted")
	}
	t.Setenv("APK_EXIT", "42")
	if err := command(); err == nil {
		t.Fatal("apk failure hidden")
	}
	if err := WriteInstallers(dir, "missing"); err == nil {
		t.Fatal("missing primary accepted")
	}
}

func TestPHPInstallersWithBuiltinOPcache(t *testing.T) {
	for _, primary := range []string{"php85", "php86"} {
		t.Run(primary, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{primary, primary + "-common", primary + "-fpm"} {
				info := "pkgname = " + name + "\npkgver = 1-r0\narch = aarch64\n"
				if name == primary {
					info += "depend = " + primary + "-common=1-r0\n"
				}
				if err := os.WriteFile(filepath.Join(dir, name+"-1-r0.apk"), tarStream(t, ".PKGINFO", info), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteInstallers(dir, primary); err != nil {
				t.Fatal(err)
			}
			script, err := os.ReadFile(filepath.Join(dir, primary))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(script), primary+"-opcache") || !strings.Contains(string(script), primary+"-fpm-1-r0.apk") || !strings.Contains(string(script), primary+"-common-1-r0.apk") {
				t.Fatalf("incorrect built-in OPcache install: %s", script)
			}
			if err := os.Remove(filepath.Join(dir, primary+"-fpm-1-r0.apk")); err != nil {
				t.Fatal(err)
			}
			if err := WriteInstallers(dir, primary); err == nil {
				t.Fatal("missing FPM accepted")
			}
		})
	}
}

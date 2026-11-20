package buildapk

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinsBeforeBuild(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	recipe := "pkgver=1.2.3\nsha256sums=\"" + checksum + "  source.tar.gz\"\n"
	env := map[string]string{"EXPECTED_BASE_IMAGE": "alpine:3.24@sha256:test", "PACKAGES_BASE_IMAGE": "alpine:3.24@sha256:test", "SOURCE_VERSION": "1.2.3", "SOURCE_ALGORITHM": "sha256", "SOURCE_CHECKSUM": checksum, "SOURCE_ARCHIVE": "source.tar.gz"}
	get := func(key string) string { return env[key] }
	if err := Validate(recipe, get); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"EXPECTED_BASE_IMAGE", "PACKAGES_BASE_IMAGE", "SOURCE_VERSION", "SOURCE_ALGORITHM", "SOURCE_CHECKSUM", "SOURCE_ARCHIVE"} {
		original := env[key]
		env[key] = "wrong"
		if Validate(recipe, get) == nil {
			t.Fatal("accepted bad " + key)
		}
		env[key] = original
		delete(env, key)
		if Validate(recipe, get) == nil {
			t.Fatal("accepted missing " + key)
		}
		env[key] = original
	}
}

// Exercise the artifact-producing path with actual APK/source files and fake
// native executables. No host abuild, root filesystem or registry is required.
func TestRunBuildArtifactsAndRestoration(t *testing.T) {
	for _, failure := range []string{"", "abuild-keygen -ain", "abuild fetch verify", "abuild -r -K", "abuild srcpkg", "missing sources", "missing apks", "apk index", "abuild-sign", "apk verify", "missing index"} {
		t.Run(failure, func(t *testing.T) {
			home := t.TempDir()
			t.Chdir(home)
			checksum := strings.Repeat("a", 64)
			recipe := "pkgver=1.2.3\nsha256sums=\"" + checksum + "  source.tar.gz\"\n"
			if err := os.WriteFile("APKBUILD", []byte(recipe), 0644); err != nil {
				t.Fatal(err)
			}
			for k, v := range map[string]string{"EXPECTED_BASE_IMAGE": "base", "PACKAGES_BASE_IMAGE": "base", "SOURCE_VERSION": "1.2.3", "SOURCE_ALGORITHM": "sha256", "SOURCE_CHECKSUM": checksum, "SOURCE_ARCHIVE": "source.tar.gz", "PACKAGE_TARGET": "alpine/3.24/nginx/1.30"} {
				t.Setenv(k, v)
			}
			var calls []string
			execute := func(name string, args ...string) error {
				call := name + " " + strings.Join(args, " ")
				calls = append(calls, call)
				if name == "abuild-keygen" {
					os.MkdirAll(".abuild", 0755)
					os.WriteFile(".abuild/private.rsa", []byte("secret"), 0600)
				}
				if name == "apk" && len(args) > 0 && args[0] == "index" {
					if failure == "apk index" {
						return fmt.Errorf("index failed")
					}
					if failure != "missing index" {
						os.WriteFile(args[2], []byte("signed-index-fixture"), 0644)
					}
				}
				if name == failure || name == "apk" && len(args) > 0 && "apk "+args[0] == failure {
					return fmt.Errorf("injected index/signature failure")
				}
				if call == failure {
					return fmt.Errorf("injected %s failure", call)
				}
				if call == "abuild -r -K" && failure != "missing apks" {
					var payload bytes.Buffer
					gz := gzip.NewWriter(&payload)
					tw := tar.NewWriter(gz)
					info := "pkgname = nginx\npkgver = 1.2.3-r0\narch = aarch64\nlicense = MIT\nbuilddate = 1234\n"
					tw.WriteHeader(&tar.Header{Name: ".PKGINFO", Mode: 0644, Size: int64(len(info))})
					tw.Write([]byte(info))
					tw.Close()
					gz.Close()
					os.MkdirAll("packages/main/aarch64", 0755)
					os.WriteFile("packages/main/aarch64/nginx.apk", payload.Bytes(), 0644)
				}
				if call == "abuild srcpkg" && failure != "missing sources" {
					os.MkdirAll("packages/src", 0755)
					os.WriteFile("packages/src/nginx.src.tar.gz", []byte("source"), 0644)
				}
				return nil
			}
			err := runBuild(home, config.Snapshot(), execute)
			if (err == nil) != (failure == "") {
				t.Fatalf("failure %q: %v (%v)", failure, err, calls)
			}
			got, _ := os.ReadFile("APKBUILD")
			if string(got) != recipe {
				t.Fatal("recipe was not restored")
			}
			if _, err := os.Stat(".abuild/private.rsa"); !os.IsNotExist(err) {
				t.Fatal("private signing key retained")
			}
			if failure == "" {
				for _, file := range []string{"apks/nginx.apk", "apks/APKINDEX.tar.gz", "sources/nginx.src.tar.gz", "metadata/release.json"} {
					if _, err := os.Stat(file); err != nil {
						t.Fatal(err)
					}
				}
				if len(calls) != 9 || strings.Join(calls[:6], "|") != "abuild-keygen -ain|abuild-apk update|abuild fetch verify|abuild -r -K|abuild -r deps|abuild srcpkg" {
					t.Fatal(calls)
				}
			}
		})
	}
}

func TestSigningIdentityDoesNotCollideAcrossBuilds(t *testing.T) {
	seen := map[string]bool{}
	for _, target := range []string{"alpine/3.24/nginx/1.30", "alpine/3.24/go/1.27", "alpine/3.24/go/1.27"} {
		home := t.TempDir()
		t.Chdir(home)
		sum := strings.Repeat("a", 64)
		os.WriteFile("APKBUILD", []byte("pkgver=1.2.3\nsha256sums=\""+sum+"  source.tar.gz\"\n"), 0644)
		for k, v := range map[string]string{"EXPECTED_BASE_IMAGE": "base", "PACKAGES_BASE_IMAGE": "base", "SOURCE_VERSION": "1.2.3", "SOURCE_ALGORITHM": "sha256", "SOURCE_CHECKSUM": sum, "SOURCE_ARCHIVE": "source.tar.gz", "PACKAGE_TARGET": target} {
			t.Setenv(k, v)
		}
		var identity string
		err := runBuild(home, config.Snapshot(), func(name string, args ...string) error {
			if name == "abuild-keygen" {
				data, e := os.ReadFile(filepath.Join(home, ".abuild/abuild.conf"))
				if e != nil {
					return e
				}
				identity = string(data)
			}
			return fmt.Errorf("stop after keygen")
		})
		if err == nil || identity == "" || seen[identity] {
			t.Fatalf("colliding/missing identity: %q", identity)
		}
		id, _ := buildhelpers.TargetID(target)
		if !strings.Contains(identity, "<"+id+"-") || !strings.Contains(identity, "@keys.libops.invalid>") {
			t.Fatal(identity)
		}
		seen[identity] = true
	}
}

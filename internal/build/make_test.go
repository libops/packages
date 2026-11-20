package build

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeLocalAndCIPackageHandoffs(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "published")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(200)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"name":"nginx","tags":[]}`)
			return
		}
		if _, err := os.Stat(marker); err == nil {
			w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("0", 63)+"1")
			w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
			w.Header().Set("Content-Length", "123")
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	repository := strings.TrimPrefix(server.URL, "http://") + "/libops"
	binary := filepath.Join(dir, "packages")
	compile := exec.Command("go", "build", "-o", binary, "./cmd/packages")
	compile.Dir = root
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v %s", err, out)
	}
	script := `#!/bin/bash
printf '%s\n' "$*" >> "$DOCKER_CAPTURE.args"
if [[ "$1 $2 $3" == 'buildx imagetools create' && "$*" == *-r[0-9]*-alpine3.24* ]]; then touch "$RELEASE_MARKER"; fi
if [[ "$1" == login ]]; then cat >/dev/null; fi
if [[ "$1 $2" == 'buildx bake' ]]; then cat >> "$DOCKER_CAPTURE.plans"; printf '\n' >> "$DOCKER_CAPTURE.plans"; fi
if [[ "$1 $2 $3" == 'buildx imagetools inspect' && "$4" != *:candidate-* && "$4" != *:pr-* ]]; then echo 'manifest unknown' >&2; exit 1; fi
if [[ "$1 $2 $3" == 'buildx imagetools inspect' ]]; then printf 'sha256:%064d\n' 1; fi
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("RELEASE_MARKER", marker)
	t.Setenv("TARGET", "alpine/3.24/nginx/1.30")
	t.Setenv("ARCH", "arm64")
	t.Setenv("REPOSITORY", repository)
	t.Setenv("LOCAL_BUILDER", "1")
	t.Setenv("GITHUB_SHA", strings.Repeat("a", 40))
	t.Setenv("GITHUB_REPOSITORY", "libops/packages")
	t.Setenv("GITHUB_REF", "refs/heads/main")
	t.Setenv("PR", "")
	t.Setenv("CONTEXTS", "")
	t.Setenv("PUBLISH", "")
	makeTarget := func(target string) {
		t.Helper()
		cmd := exec.Command("make", "--silent", target, "PACKAGES_CLI="+binary)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("make %s: %v\n%s", target, err, out)
		}
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, ci := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "CI"}[ci], func(t *testing.T) {
			capture := filepath.Join(dir, t.Name()[strings.LastIndex(t.Name(), "/")+1:])
			t.Setenv("DOCKER_CAPTURE", capture)
			t.Setenv("CI", "")
			t.Setenv("PACKAGE_REF", "")
			t.Setenv("GITHUB_OUTPUT", "")
			t.Setenv("REGISTRY_USER", "")
			t.Setenv("REGISTRY_PASSWORD", "")
			t.Setenv("GITHUB_TOKEN", "")
			t.Setenv("GITHUB_ACTOR", "")
			published := "docker-image://" + repository + "/nginx@sha256:" + strings.Repeat("0", 63) + "1"
			if ci {
				t.Setenv("CI", "true")
				t.Setenv("REGISTRY_USER", "bot")
				t.Setenv("REGISTRY_PASSWORD", "test-secret")
				output := capture + ".outputs"
				if err := os.WriteFile(output, nil, 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GITHUB_OUTPUT", output)
				makeTarget("publish")
				makeTarget("manifest")
				if !strings.Contains(read(output), "context="+published) {
					t.Fatal("manifest did not emit pinned test context")
				}
				t.Setenv("PACKAGE_REF", published)
			} else {
				makeTarget("build")
			}
			// Local tests must use the local carrier even if HEAD changes after building.
			if !ci {
				t.Setenv("GITHUB_SHA", strings.Repeat("b", 40))
			}
			makeTarget("test")
			args := read(capture + ".args")
			if ci {
				if !strings.Contains(args, "--push") || !strings.Contains(args, "packages="+published) {
					t.Fatal("missing CI publication/test handoff", args)
				}
			} else {
				if strings.Contains(args, "--push") || strings.Contains(args, "login") {
					t.Fatal("local build/test attempted authentication or publication", args)
				}
				if !strings.Contains(args, "--builder default") {
					t.Fatal("local smoke check cannot resolve loaded daemon image", args)
				}
				if !strings.Contains(args, "packages=docker-image://"+repository+"/nginx:local-alpine-3-24-nginx-1-30-arm64") {
					t.Fatal("local test did not use stable built image", args)
				}
			}
			lines := strings.Split(strings.TrimSpace(read(capture+".plans")), "\n")
			if len(lines) != 2 {
				t.Fatal("expected build then upstream-test plans", len(lines))
			}
			for index, line := range lines {
				var plan Plan
				if err := json.Unmarshal([]byte(line), &plan); err != nil {
					t.Fatal(err)
				}
				target := plan["target"]["package"]
				if index == 1 {
					if _, ok := target["tags"]; ok {
						t.Fatal("test retags carrier")
					}
					if _, ok := target["cache-to"]; ok {
						t.Fatal("test exports build cache")
					}
					if target["target"] != "test" {
						t.Fatal("upstream tests not selected")
					}
				}
				if !ci {
					if _, ok := target["cache-to"]; ok {
						t.Fatal("local work writes remote cache")
					}
				}
			}
		})
	}
}

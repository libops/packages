package build

import (
	"encoding/json"
	"github.com/libops/packages/internal/catalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlansDoNotPublishLocalBuilds(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range []map[string]string{{}, {"PUBLISH": "1", "PR": "12", "GITHUB_SHA": "abc"}} {
		plan, err := PlanFor("../..", c, "alpine-3-24-php-8-4", env)
		if err != nil {
			t.Fatal(err)
		}
		p := plan["target"]["package"]
		_, cache := p["cache-to"]
		if cache != (env["PUBLISH"] == "1") {
			t.Fatal("cache publication mismatch", p)
		}
		if cache && !strings.Contains(p["cache-to"].([]string)[0], ":cache-pr-12-") {
			t.Fatal(p)
		}
	}
	for _, env := range []map[string]string{{"PR": "../12"}, {"ARCH": "unknown"}, {"CONTEXTS": "unknown=x"}} {
		if _, err := PlanFor("../..", c, "alpine-3-24-php-8-4", env); err == nil {
			t.Fatal("accepted invalid build input", env)
		}
	}
}

func TestBuilderPinFollowsExactBase(t *testing.T) {
	root := t.TempDir()
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	target := "alpine-3-24-php-8-4"
	p, err := c.Metadata("../..", target)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"architectures.json", "packages/alpine/3.24/base-image.txt", p.RecipeDir + "/APKBUILD", p.RecipeDir + "/security.json"} {
		data, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	pin := filepath.Join(root, "packages/alpine/3.24/builder.json")
	image := "ghcr.io/libops/package-builder:alpine3.24@sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, base, override string
		local                bool
	}{{"exact", p.Base, "", false}, {"stale base", "alpine:old", "", true}, {"explicit local", p.Base, "1", true}} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]string{"image": image, "base": tc.base})
			if err := os.WriteFile(pin, data, 0644); err != nil {
				t.Fatal(err)
			}
			plan, err := PlanFor(root, c, target, map[string]string{"LOCAL_BUILDER": tc.override})
			if err != nil {
				t.Fatal(err)
			}
			context := plan["target"]["package"]["contexts"].(map[string]string)["builder"]
			if (context == "target:builder") != tc.local {
				t.Fatalf("wrong builder context %s", context)
			}
			if !tc.local && context != "docker-image://"+image {
				t.Fatal(context)
			}
		})
	}
}

func TestSecurityLabelsMatchReviewedMetadata(t *testing.T) {
	root := t.TempDir()
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	id := "alpine-3-24-php-8-4"
	p, err := c.Metadata("../..", id)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"architectures.json", "packages/alpine/3.24/base-image.txt", p.RecipeDir + "/APKBUILD"} {
		data, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(root, p.RecipeDir, "security.json")
	for _, tc := range []struct {
		name, version, status, reference string
		valid                            bool
	}{
		{"reviewed", p.Version, "security", "https://example.org/advisory", true},
		{"stale", "0.0.0", "security", "https://example.org/advisory", false},
		{"no evidence", p.Version, "maintenance", "", false},
		{"bad link", p.Version, "security", "http://example.org/advisory", false},
		{"bad status", p.Version, "safe", "https://example.org/advisory", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs := []string{}
			if tc.reference != "" {
				refs = append(refs, tc.reference)
			}
			data, _ := json.Marshal(map[string]any{"version": tc.version, "status": tc.status, "advisories": []string{"CVE-2026-12345"}, "references": refs})
			if err := os.WriteFile(file, data, 0644); err != nil {
				t.Fatal(err)
			}
			plan, err := PlanFor(root, c, id, nil)
			if (err == nil) != tc.valid {
				t.Fatalf("validation result: %v", err)
			}
			if err == nil {
				labels := plan["target"]["package"]["labels"].(map[string]string)
				if labels["io.libops.packages.security.status"] != tc.status || labels["io.libops.packages.security.advisories"] != "CVE-2026-12345" {
					t.Fatal("labels disagree", labels)
				}
			}
		})
	}
}

func TestDefaultCarrierNamespaceIsIsolated(t *testing.T) {
	c, err := catalog.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range c.Names() {
		plan, err := PlanFor("../..", c, target, map[string]string{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range plan["target"]["package"]["tags"].([]string) {
			if !strings.HasPrefix(tag, "ghcr.io/libops/") {
				t.Fatalf("carrier escaped its namespace: %s", tag)
			}
		}
	}
}

// Package build runs the same BuildKit operations locally and in Actions.
package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/catalog"
)

type Plan map[string]map[string]map[string]any

func PlanFor(root string, c catalog.Catalog, name string, env map[string]string) (Plan, error) {
	p, err := c.Metadata(root, name)
	if err != nil {
		return nil, err
	}
	arch := env["ARCH"]
	if arch == "" {
		arch = runtime.GOARCH
	}
	profiles, err := c.Platforms(root, p.ID)
	if err != nil {
		return nil, err
	}
	platform := ""
	for _, profile := range profiles {
		if profile.Arch == arch {
			platform = profile.Platform
		}
	}
	if platform == "" {
		return nil, fmt.Errorf("architecture %s is not enabled for %s", arch, p.ID)
	}
	repository := env["REPOSITORY"]
	if repository == "" {
		repository = "ghcr.io/libops"
	}
	scope, suffix := "main", ""
	if pr := env["PR"]; pr != "" {
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(pr) {
			return nil, fmt.Errorf("invalid PR number")
		}
		scope = "pr-" + pr
		sha := env["GITHUB_SHA"]
		if sha == "" {
			sha = "local"
		}
		suffix = "-" + sha
	}
	cache := repository + "/" + p.Image + ":cache-" + scope + "-" + p.ID + suffix + "-" + arch
	contexts := map[string]string{"alpine": "docker-image://" + p.Base}
	for _, dependency := range p.Dependencies {
		dep, err := c.Metadata(root, dependency)
		if err != nil {
			return nil, err
		}
		contexts[dependency] = "docker-image://" + repository + "/" + dep.Image + ":" + dep.RevisionTag
	}
	for _, override := range strings.Fields(env["CONTEXTS"]) {
		dependency, ref, ok := strings.Cut(override, "=")
		if !ok {
			return nil, fmt.Errorf("malformed dependency context")
		}
		if _, ok = contexts[dependency]; !ok || dependency == "alpine" {
			return nil, fmt.Errorf("unknown dependency context %s", dependency)
		}
		if ref != "" {
			contexts[dependency] = ref
		}
	}
	parts := strings.Split(p.Target, "/")
	pin := filepath.Join(root, "packages", parts[0], parts[1], "builder.json")
	var record struct{ Image, Base string }
	if data, err := os.ReadFile(pin); err == nil {
		if err = json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if record.Image != "" && !regexp.MustCompile(`^[a-z0-9./:_-]+@sha256:[a-f0-9]{64}$`).MatchString(record.Image) {
		return nil, fmt.Errorf("builder image must be pinned by digest")
	}
	local := env["LOCAL_BUILDER"] == "1" || record.Image == "" || record.Base != p.Base
	if local {
		contexts["builder"] = "target:builder"
	} else {
		contexts["builder"] = "docker-image://" + record.Image
	}
	image := env["IMAGE_REF"]
	if image == "" {
		image = repository + "/" + p.Image + ":local-" + p.ID
	}
	source := env["GITHUB_REPOSITORY"]
	if source == "" {
		source = "libops/packages"
	}
	revision := env["GITHUB_SHA"]
	if revision == "" {
		revision = "local"
	}
	base, _, _ := strings.Cut(p.Base, "@")
	baseDigest := strings.SplitN(p.Base, "@", 2)[1]
	recipe, err := os.ReadFile(filepath.Join(root, p.RecipeDir, "APKBUILD"))
	if err != nil {
		return nil, err
	}
	args, err := buildhelpers.SourcePins(string(recipe))
	if err != nil {
		return nil, err
	}
	args["EXPECTED_BASE_IMAGE"] = p.Base
	args["PACKAGE_TARGET"] = p.Target
	args["RECIPE_DIR"] = p.RecipeDir
	target := map[string]any{"context": ".", "dockerfile": "Dockerfile.package", "target": "packages", "args": args, "contexts": contexts, "platforms": []string{platform}, "tags": []string{image}, "labels": map[string]string{
		"org.opencontainers.image.source": "https://github.com/" + source, "org.opencontainers.image.version": p.Revision, "org.opencontainers.image.revision": revision, "org.opencontainers.image.base.name": base, "org.opencontainers.image.base.digest": baseDigest, "io.libops.packages.target": p.Target, "io.libops.packages.security.status": p.Security.Status, "io.libops.packages.security.advisories": strings.Join(p.Security.Advisories, ",")}}
	if env["PUBLISH"] == "1" || env["CI"] != "" || env["PACKAGE_REF"] != "" {
		refs := []string{"type=registry,ref=" + cache}
		mainCache := repository + "/" + p.Image + ":cache-main-" + p.ID + "-" + arch
		if cache != mainCache {
			refs = append(refs, "type=registry,ref="+mainCache)
		}
		target["cache-from"] = refs
	}
	if env["PUBLISH"] == "1" {
		target["cache-to"] = []string{"type=registry,mode=max,image-manifest=true,oci-mediatypes=true,ref=" + cache}
		target["attest"] = []string{"type=provenance,mode=min"}
	}
	targets := map[string]map[string]any{"package": target}
	if local {
		targets["builder"] = map[string]any{"context": ".", "dockerfile": "Dockerfile", "target": "tools", "contexts": map[string]string{"alpine": "docker-image://" + p.Base}, "platforms": []string{platform}, "args": map[string]string{"BASE_IMAGE": p.Base}}
	}
	return Plan{"target": targets}, nil
}

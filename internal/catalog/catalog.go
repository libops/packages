// Package catalog defines target identities, validation and affected-only selection.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/libops/packages/buildhelpers"
)

type Update = buildhelpers.Update
type Package = buildhelpers.InventoryPackage
type Catalog map[string]Package

type Metadata struct {
	Package
	ID          string                `json:"id"`
	Image       string                `json:"image"`
	Version     string                `json:"version"`
	Revision    string                `json:"revision"`
	Tag         string                `json:"tag"`
	RevisionTag string                `json:"revision_tag"`
	RecipeDir   string                `json:"recipe_dir"`
	Base        string                `json:"base"`
	Security    buildhelpers.Security `json:"security"`
}

func ID(target string) (string, error) { return buildhelpers.TargetID(target) }

func (c Catalog) Names() []string {
	result := []string{}
	for name := range c {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
func (c Catalog) Bases() []string {
	seen := map[string]bool{}
	for _, p := range c {
		parts := strings.Split(p.Target, "/")
		seen[strings.Join(parts[:2], "/")] = true
	}
	result := []string{}
	for base := range seen {
		result = append(result, base)
	}
	sort.Strings(result)
	return result
}
func (c Catalog) Validate() error {
	for name, p := range c {
		id, err := ID(p.Target)
		if err != nil {
			return err
		}
		if id != name {
			return fmt.Errorf("%s: key must match target identity", name)
		}
		parts := strings.Split(p.Target, "/")
		if p.Line != parts[3] {
			return fmt.Errorf("%s: release line differs from target", name)
		}
		if !strings.HasPrefix(p.Upstream, "https://") {
			return fmt.Errorf("%s: missing upstream support reference", name)
		}
		for _, dep := range p.Dependencies {
			parent, ok := c[dep]
			if !ok {
				return fmt.Errorf("%s: unknown dependency %s", name, dep)
			}
			if _, err := ID(parent.Target); err != nil {
				return err
			}
			if strings.Join(strings.Split(parent.Target, "/")[:2], "/") != strings.Join(parts[:2], "/") {
				return fmt.Errorf("%s: dependency must use same distro release", name)
			}
		}
	}
	visited, active := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if active[name] {
			return fmt.Errorf("dependency cycle at %s", name)
		}
		if visited[name] {
			return nil
		}
		active[name] = true
		for _, dep := range c[name].Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(active, name)
		visited[name] = true
		return nil
	}
	for name := range c {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}
func Load(root string) (Catalog, error) {
	records, err := buildhelpers.ReadInventory(root)
	if err != nil {
		return nil, err
	}
	c := Catalog(records)
	if err = c.Validate(); err != nil {
		return nil, err
	}
	for name, p := range c {
		if _, err = c.Platforms(root, name); err != nil {
			return nil, err
		}
		for _, dep := range p.Dependencies {
			for _, arch := range p.Architectures {
				supported := false
				for _, parentArch := range c[dep].Architectures {
					if parentArch == arch {
						supported = true
					}
				}
				if !supported {
					return nil, fmt.Errorf("%s: dependency %s does not support %s", name, dep, arch)
				}
			}
		}
		if err = buildhelpers.ValidatePackage(p.Package); err != nil {
			return nil, err
		}
		if _, err = os.Stat(filepath.Join(root, "packages", p.Target, "APKBUILD")); err != nil {
			return nil, fmt.Errorf("%s: native builder not implemented or recipe missing: %w", name, err)
		}
		parts := strings.Split(p.Target, "/")
		base, err := os.ReadFile(filepath.Join(root, "packages", parts[0], parts[1], "base-image.txt"))
		if err != nil {
			return nil, err
		}
		value := strings.TrimSpace(string(base))
		if !regexp.MustCompile(`^[a-z0-9./:_-]+@sha256:[a-f0-9]{64}$`).MatchString(value) {
			return nil, fmt.Errorf("%s: base must be digest-pinned", name)
		}
		if parts[0] == "alpine" && !strings.HasPrefix(value, "alpine:"+parts[1]+".") {
			return nil, fmt.Errorf("%s: wrong distro release base", name)
		}
	}
	return c, nil
}
func (c Catalog) Metadata(root, name string) (Metadata, error) {
	if strings.Contains(name, "/") {
		id, err := ID(name)
		if err != nil {
			return Metadata{}, err
		}
		name = id
	}
	p, ok := c[name]
	if !ok {
		return Metadata{}, fmt.Errorf("unknown target %q", name)
	}
	directory := filepath.Join("packages", p.Target)
	data, err := os.ReadFile(filepath.Join(root, directory, "APKBUILD"))
	if err != nil {
		return Metadata{}, err
	}
	field := func(key string) (string, error) {
		match := regexp.MustCompile(`(?m)^` + key + `=(\S+)$`).FindSubmatch(data)
		if match == nil {
			return "", fmt.Errorf("missing %s", key)
		}
		return string(match[1]), nil
	}
	version, err := field("pkgver")
	if err != nil {
		return Metadata{}, err
	}
	revision, err := field("pkgrel")
	if err != nil {
		return Metadata{}, err
	}
	revision = version + "-r" + revision
	parts := strings.Split(p.Target, "/")
	base, err := os.ReadFile(filepath.Join(root, "packages", parts[0], parts[1], "base-image.txt"))
	if err != nil {
		return Metadata{}, err
	}
	security, err := buildhelpers.ReadSecurity(filepath.Join(root, directory), version)
	if err != nil {
		return Metadata{}, err
	}
	suffix := "-" + parts[0] + parts[1]
	return Metadata{Package: p, ID: name, Image: parts[2], Version: version, Revision: revision, Tag: version + suffix, RevisionTag: revision + suffix, RecipeDir: directory, Base: strings.TrimSpace(string(base)), Security: security}, nil
}
func (c Catalog) Affected(files []string, previous Catalog) []string {
	selected := map[string]bool{}
	utility := map[string]bool{"Dockerfile": true, "go.mod": true, ".github/workflows/builders.yml": true, ".github/workflows/builder.yml": true}
	// Generated files are validated by make dev-check. Their inventory inputs select
	// targets; regenerating them must not fan out a one-package change to all builds.
	metadata := map[string]bool{"renovate.json": true, ".github/workflows/push.yml": true,
		"ci/renovate-allowed-commands.json": true, ".github/CODEOWNERS": true,
		".github/workflows/renovate.yml": true, ".github/workflows/review-lines.yml": true,
		".github/workflows/cleanup.yml": true}
	docs := map[string]bool{"README.md": true, "LICENSE": true, ".gitignore": true, "zensical.toml": true, "requirements-docs.txt": true, "Dockerfile.docs": true, ".github/workflows/docs.yml": true, ".github/FUNDING.yml": true}
	for _, file := range files {
		if file == "" {
			continue
		}
		parts := strings.Split(filepath.ToSlash(file), "/")
		if file == "packages.json" && previous != nil {
			for name, p := range c {
				old, exists := previous[name]
				if !exists || old.Target != p.Target || !reflect.DeepEqual(old.Dependencies, p.Dependencies) || !reflect.DeepEqual(old.Architectures, p.Architectures) {
					selected[name] = true
				}
			}
			continue
		}
		if metadata[file] || strings.HasPrefix(file, ".github/ISSUE_TEMPLATE/") {
			continue
		}
		if utility[file] || ValidationInput(file) || strings.HasSuffix(file, "/builder.json") {
			continue
		}
		if parts[0] == "packages" {
			for name, p := range c {
				recipe := "packages/" + p.Target
				release := "packages/" + strings.Join(strings.Split(p.Target, "/")[:2], "/")
				if strings.HasPrefix(file, recipe+"/") || strings.HasPrefix(file, release+"/") && len(parts) <= 4 {
					selected[name] = true
				}
			}
			continue
		}
		if docs[file] || parts[0] == "docs" || parts[0] == "site" || strings.HasSuffix(file, ".md") {
			continue
		}
		for name := range c {
			selected[name] = true
		}
	}
	for {
		changed := false
		for name, p := range c {
			if selected[name] {
				continue
			}
			for _, dep := range p.Dependencies {
				if selected[dep] {
					selected[name] = true
					changed = true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
	result := []string{}
	for name := range selected {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// HelperInput affects the executable or tool image used inside native builds.
func HelperInput(file string) bool {
	return file == "Dockerfile" || file == "go.mod" || file == "go.sum" || strings.HasPrefix(file, "cmd/") || strings.HasPrefix(file, "internal/") || strings.HasPrefix(file, "buildhelpers/")
}

// ValidationInput changes orchestration, not a package release's source lock.
func ValidationInput(file string) bool {
	return HelperInput(file) || strings.HasPrefix(file, ".github/workflows/") || strings.HasPrefix(file, "ci/") || file == "Makefile" || file == "Dockerfile.package" || file == "test.Dockerfile" || file == "architectures.json"
}

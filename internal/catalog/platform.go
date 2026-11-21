package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Platform joins a stable artifact suffix to an OCI platform and native runner.
// A suffix such as armv7 may map to linux/arm/v7 without putting slashes in tags.
type Platform struct {
	Arch     string   `json:"arch"`
	Platform string   `json:"platform"`
	Runner   []string `json:"runner"`
}

func Architectures(root string) (map[string]Platform, error) {
	data, err := os.ReadFile(filepath.Join(root, "architectures.json"))
	if err != nil {
		return nil, err
	}
	var platforms map[string]Platform
	if err = json.Unmarshal(data, &platforms); err != nil {
		return nil, err
	}
	if len(platforms) == 0 {
		return nil, fmt.Errorf("no architectures configured")
	}
	for name, p := range platforms {
		if !regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(name) || !regexp.MustCompile(`^linux/[a-z0-9]+(?:/v[0-9]+)?$`).MatchString(p.Platform) || len(p.Runner) == 0 {
			return nil, fmt.Errorf("invalid architecture profile %q", name)
		}
		for _, label := range p.Runner {
			if !regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(label) {
				return nil, fmt.Errorf("invalid runner label for %s", name)
			}
		}
		p.Arch = name
		platforms[name] = p
	}
	return platforms, nil
}
func (c Catalog) Platforms(root, name string) ([]Platform, error) {
	profiles, err := Architectures(root)
	if err != nil {
		return nil, err
	}
	p, ok := c[name]
	if !ok {
		return nil, fmt.Errorf("unknown package %s", name)
	}
	if len(p.Architectures) == 0 {
		return nil, fmt.Errorf("%s: explicit architectures required", name)
	}
	result := []Platform{}
	seen := map[string]bool{}
	for _, arch := range p.Architectures {
		profile, ok := profiles[arch]
		if !ok || seen[arch] {
			return nil, fmt.Errorf("%s: unknown or duplicate architecture %s", name, arch)
		}
		seen[arch] = true
		result = append(result, profile)
	}
	return result, nil
}
func (c Catalog) BuilderPlatforms(root, base string) ([]Platform, error) {
	seen := map[string]Platform{}
	for name, p := range c {
		if strings.HasPrefix(p.Target, base+"/") {
			profiles, err := c.Platforms(root, name)
			if err != nil {
				return nil, err
			}
			for _, p := range profiles {
				seen[p.Arch] = p
			}
		}
	}
	result := []Platform{}
	for _, p := range seen {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Arch < result[j].Arch })
	if len(result) == 0 {
		return nil, fmt.Errorf("no packages registered for %s", base)
	}
	return result, nil
}

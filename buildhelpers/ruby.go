package buildhelpers

import (
	"fmt"
	"go.yaml.in/yaml/v3"
)

type RubyRelease struct {
	Version string            `yaml:"version"`
	SHA256  map[string]string `yaml:"sha256"`
	SHA512  map[string]string `yaml:"sha512"`
}

func ParseRubyReleases(data []byte) ([]RubyRelease, error) {
	var releases []RubyRelease
	if err := yaml.Unmarshal(data, &releases); err != nil {
		return nil, err
	}
	return releases, nil
}
func RubyChecksum(metadata, version string) (string, string, error) {
	releases, err := ParseRubyReleases([]byte(metadata))
	if err != nil {
		return "", "", err
	}
	for _, release := range releases {
		if release.Version != version {
			continue
		}
		if sum := release.SHA512["gz"]; sum != "" {
			return "sha512", sum, nil
		}
		if sum := release.SHA256["gz"]; sum != "" {
			return "sha256", sum, nil
		}
	}
	return "", "", fmt.Errorf("official Ruby metadata has no supported checksum for %s", version)
}

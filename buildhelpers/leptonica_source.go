package buildhelpers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func (u Updater) leptonicaSource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	if !strings.HasPrefix(version, p.Line+".") || !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
		return source{}, fmt.Errorf("invalid Leptonica stable version")
	}
	filename = "leptonica-" + version + ".tar.gz"
	url = "https://github.com/DanBloomberg/leptonica/releases/download/" + version + "/" + filename
	data, e := u.fetch("https://api.github.com/repos/DanBloomberg/leptonica/releases/tags/" + version)
	if e != nil {
		return source{}, e
	}
	var metadata struct {
		Draft      bool
		Prerelease bool
		Assets     []struct {
			Name   string
			Digest string
		}
	}
	if e = json.Unmarshal(data, &metadata); e != nil {
		return source{}, e
	}
	if metadata.Draft || metadata.Prerelease {
		return source{}, fmt.Errorf("not a stable published release")
	}
	for _, asset := range metadata.Assets {
		if asset.Name == filename {
			algorithm, expected, _ = strings.Cut(asset.Digest, ":")
		}
	}
	return source{filename, url, algorithm, expected, recipe}, nil
}

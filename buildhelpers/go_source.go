package buildhelpers

import (
	"encoding/json"
)

func (u Updater) goSource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	filename = "go" + version + ".src.tar.gz"
	url = "https://go.dev/dl/" + filename
	data, e := u.fetch("https://go.dev/dl/?mode=json&include=all")
	if e != nil {
		return source{}, e
	}
	var releases []struct {
		Version string
		Stable  bool
		Files   []struct {
			Filename string
			SHA256   string
		}
	}
	if e = json.Unmarshal(data, &releases); e != nil {
		return source{}, e
	}
	algorithm = "sha256"
	for _, release := range releases {
		if release.Version == "go"+version && release.Stable {
			for _, file := range release.Files {
				if file.Filename == filename {
					expected = file.SHA256
				}
			}
		}
	}
	return source{filename, url, algorithm, expected, recipe}, nil
}

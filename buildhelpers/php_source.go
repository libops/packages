package buildhelpers

import (
	"encoding/json"
)

func (u Updater) phpSource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	filename = "php-" + version + ".tar.xz"
	url = "https://www.php.net/distributions/" + filename
	data, e := u.fetch("https://www.php.net/releases/index.php?json&version=" + version)
	if e != nil {
		return source{}, e
	}
	var metadata struct {
		Source []struct {
			Filename string `json:"filename"`
			SHA256   string `json:"sha256"`
		} `json:"source"`
	}
	if e = json.Unmarshal(data, &metadata); e != nil {
		return source{}, e
	}
	algorithm = "sha256"
	for _, file := range metadata.Source {
		if file.Filename == filename {
			expected = file.SHA256
		}
	}
	return source{filename, url, algorithm, expected, recipe}, nil
}

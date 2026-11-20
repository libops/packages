package buildhelpers

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

func (u Updater) imagemagickSource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	tag, e := field(recipe, "# upstream-tag")
	if e != nil {
		return source{}, e
	}
	if !regexp.MustCompile(`^` + regexp.QuoteMeta(p.Line) + `\.[0-9]+\.[0-9]+-[0-9]+$`).MatchString(tag) {
		return source{}, fmt.Errorf("invalid ImageMagick stable tag")
	}
	version = strings.ReplaceAll(tag, "-", ".")
	recipe = setField(recipe, "pkgver", version)
	filename = "ImageMagick-" + tag + ".tar.xz"
	url = "https://download.imagemagick.org/archive/releases/" + filename
	data, e := u.fetch("https://download.imagemagick.org/archive/releases/digest.rdf")
	if e != nil {
		return source{}, e
	}
	var metadata struct {
		Contents []struct {
			About  string `xml:"about,attr"`
			SHA256 string `xml:"sha256"`
		} `xml:"Content"`
	}
	if e = xml.Unmarshal(data, &metadata); e != nil {
		return source{}, e
	}
	algorithm = "sha256"
	for _, entry := range metadata.Contents {
		if entry.About == filename {
			expected = entry.SHA256
		}
	}
	return source{filename, url, algorithm, expected, recipe}, nil
}

package buildhelpers

import (
	"fmt"
	"regexp"
)

func (u Updater) nginxSource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	if !regexp.MustCompile(`^` + regexp.QuoteMeta(p.Line) + `\.[0-9]+$`).MatchString(version) {
		return source{}, fmt.Errorf("nginx version must remain in %s", p.Line)
	}
	filename = "nginx-" + version + ".tar.gz"
	url = "https://nginx.org/download/" + filename
	return source{filename, url, algorithm, expected, recipe}, nil
}

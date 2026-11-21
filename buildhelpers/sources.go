package buildhelpers

import "fmt"

type source struct{ filename, url, algorithm, expected, recipe string }

func (u Updater) discoverSource(p Package, recipe, version string) (source, error) {
	switch p.Family {
	case "php":
		return u.phpSource(p, recipe, version)
	case "go":
		return u.goSource(p, recipe, version)
	case "python":
		return u.pythonSource(p, recipe, version)
	case "ruby":
		return u.rubySource(p, recipe, version)
	case "nginx":
		return u.nginxSource(p, recipe, version)
	case "imagemagick":
		return u.imagemagickSource(p, recipe, version)
	case "leptonica":
		return u.leptonicaSource(p, recipe, version)
	case "openjdk":
		return source{}, u.updateJava("", recipe, p)
	default:
		return source{}, fmt.Errorf("no verified source updater for %s", p.Family)
	}
}

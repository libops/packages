package buildhelpers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// WriteInstallers emits relocatable shell entrypoints using exact carrier APKs.
// Carrier dependencies are selected explicitly so distro builds cannot win.
func WriteInstallers(directory, primary string) error {
	paths, err := filepath.Glob(filepath.Join(directory, "*.apk"))
	if err != nil {
		return err
	}
	files := map[string]string{}
	dependencies := map[string][]string{}
	providers := map[string][]string{}
	valid := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := APKInfo(data)
		if err != nil {
			return err
		}
		name := info["pkgname"][0]
		if !valid.MatchString(name) || !valid.MatchString(filepath.Base(path)) {
			return fmt.Errorf("unsafe APK name %q", name)
		}
		if files[name] != "" {
			return fmt.Errorf("multiple APKs for %s", name)
		}
		files[name] = filepath.Base(path)
		dependencies[name] = info["depend"]
		for _, provided := range append([]string{name}, info["provides"]...) {
			keys := strings.FieldsFunc(provided, func(r rune) bool { return strings.ContainsRune("<>=~", r) })
			if len(keys) != 0 {
				providers[keys[0]] = append(providers[keys[0]], name)
			}
		}
	}
	if files[primary] == "" {
		return fmt.Errorf("missing primary APK %s", primary)
	}
	for name := range files {
		selected := map[string]bool{}
		var selectPackage func(string)
		selectPackage = func(n string) {
			if selected[n] {
				return
			}
			selected[n] = true
			for _, dep := range dependencies[n] {
				if strings.HasPrefix(dep, "!") {
					continue
				}
				key := strings.FieldsFunc(dep, func(r rune) bool { return strings.ContainsRune("<>=~", r) })
				if len(key) == 0 {
					continue
				}
				for _, provider := range providers[key[0]] {
					selectPackage(provider)
				}
			}
		}
		selectPackage(name)
		// PHP's default runtime includes FPM and OPcache; optional extensions have
		// their own entrypoints. Other defaults follow native package dependencies.
		if name == primary && regexp.MustCompile(`^php[0-9]+$`).MatchString(name) {
			for _, extra := range []string{name + "-fpm", name + "-opcache"} {
				if files[extra] == "" {
					return fmt.Errorf("missing default PHP APK %s", extra)
				}
				selectPackage(extra)
			}
		}
		names := make([]string, 0, len(selected))
		for n := range selected {
			names = append(names, n)
		}
		sort.Strings(names)
		script := "#!/bin/sh\nset -eu\nif [ \"$#\" -ne 0 ]; then echo 'This installer takes no arguments; use the entrypoint for the package you need.' >&2; exit 2; fi\ndir=$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\ncp \"$dir\"/keys/*.pub /etc/apk/keys/\nexec apk add --no-cache --repository \"$dir/APKINDEX.tar.gz\""
		for _, n := range names {
			script += " \\\n  \"$dir/" + files[n] + "\""
		}
		script += "\n"
		if err := os.WriteFile(filepath.Join(directory, name), []byte(script), 0755); err != nil {
			return err
		}
	}
	return nil
}

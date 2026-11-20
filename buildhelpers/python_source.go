package buildhelpers

func (u Updater) pythonSource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	filename = "Python-" + version + ".tar.xz"
	url = "https://www.python.org/ftp/python/" + version + "/" + filename
	return source{filename, url, algorithm, expected, recipe}, nil
}

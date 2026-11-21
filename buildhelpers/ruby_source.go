package buildhelpers

func (u Updater) rubySource(p Package, recipe, version string) (source, error) {
	var filename, url, algorithm, expected string

	filename = "ruby-" + version + ".tar.gz"
	url = "https://cache.ruby-lang.org/pub/ruby/" + p.Line + "/" + filename
	data, e := u.fetch("https://raw.githubusercontent.com/ruby/www.ruby-lang.org/master/_data/releases.yml")
	if e != nil {
		return source{}, e
	}
	algorithm, expected, e = RubyChecksum(string(data), version)
	if e != nil {
		return source{}, e
	}
	return source{filename, url, algorithm, expected, recipe}, nil
}

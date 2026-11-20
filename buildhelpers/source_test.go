package buildhelpers

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFamilySourceDiscovery(t *testing.T) {
	for _, tc := range []struct{ family, line, version, recipe, metadata, filename, algorithm, checksum string }{
		{"php", "8.4", "8.4.28", "", `{"source":[{"filename":"php-8.4.28.tar.xz","sha256":"abc"}]}`, "php-8.4.28.tar.xz", "sha256", "abc"},
		{"go", "1.27", "1.27.4", "", `[{"version":"go1.27.4","stable":true,"files":[{"filename":"go1.27.4.src.tar.gz","sha256":"abc"}]}]`, "go1.27.4.src.tar.gz", "sha256", "abc"},
		{"ruby", "4.0", "4.0.1", "", "- version: '4.0.1'\n  sha512: {gz: abc}\n", "ruby-4.0.1.tar.gz", "sha512", "abc"},
		{"nginx", "1.30", "1.30.7", "", "", "nginx-1.30.7.tar.gz", "", ""},
		{"python", "3.15", "3.15.2", "", "", "Python-3.15.2.tar.xz", "", ""},
		{"imagemagick", "7", "7.1.2.3", "# upstream-tag=7.1.2-3\npkgver=7.1.2.2\n", `<RDF><Content about="ImageMagick-7.1.2-3.tar.xz"><sha256>abc</sha256></Content></RDF>`, "ImageMagick-7.1.2-3.tar.xz", "sha256", "abc"},
		{"leptonica", "1", "1.86.0", "", `{"assets":[{"name":"leptonica-1.86.0.tar.gz","digest":"sha256:abc"}]}`, "leptonica-1.86.0.tar.gz", "sha256", "abc"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			calls := 0
			u := Updater{Client: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.metadata))}, nil
			})}}
			src, err := u.discoverSource(Package{Family: tc.family, Line: tc.line}, tc.recipe, tc.version)
			if err != nil {
				t.Fatal(err)
			}
			if src.filename != tc.filename || src.algorithm != tc.algorithm || src.expected != tc.checksum {
				t.Fatalf("%+v", src)
			}
			if !strings.HasSuffix(src.url, "/"+tc.filename) {
				t.Fatal(src.url)
			}
			if tc.family == "python" && calls != 0 {
				t.Fatal("Python scraped metadata instead of verifying its signature")
			}
		})
	}
}
func TestJavaRefreshCannotTrustDownloads(t *testing.T) {
	u := Updater{Client: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("disabled Java updater made a network request")
		return nil, nil
	})}}
	if _, err := u.discoverSource(Package{Family: "openjdk", Line: "21"}, "", "21.0.13"); err == nil {
		t.Fatal("unverified Java update was allowed")
	}
}
func TestRubyMetadataRejectsDuplicateVersionKeys(t *testing.T) {
	if _, err := ParseRubyReleases([]byte("- version: 4.0.1\n  version: 4.0.2\n")); err == nil {
		t.Fatal("ambiguous metadata accepted")
	}
}

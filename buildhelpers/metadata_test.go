package buildhelpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tarStream(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	archive.Write([]byte(content))
	archive.Close()
	gz.Close()
	return buffer.Bytes()
}
func TestNativeAPKReleaseMetadata(t *testing.T) {
	data := append(tarStream(t, ".SIGN.RSA.key.pub", "signature"), tarStream(t, ".PKGINFO", "pkgname = nginx\npkgver = 1.30.7-r0\narch = aarch64\nlicense = BSD-2-Clause\ndepend = so:libc.musl-aarch64.so.1\n")...)
	info, err := APKInfo(data)
	if err != nil || info["pkgname"][0] != "nginx" {
		t.Fatalf("%v %v", info, err)
	}
	directory := t.TempDir()
	os.WriteFile(filepath.Join(directory, "nginx.apk"), data, 0644)
	destination := filepath.Join(directory, "metadata")
	if err := WriteMetadata(directory, directory, destination, "alpine/3.24/nginx/1.30", "1.30.7", Security{Version: "1.30.7", Status: "unknown"}); err == nil {
		t.Fatal("accepted a carrier without corresponding source")
	}
	if err := os.WriteFile(filepath.Join(directory, "nginx.src.tar.gz"), []byte("verified upstream source bundle"), 0644); err != nil {
		t.Fatal(err)
	}
	err = WriteMetadata(directory, directory, destination, "alpine/3.24/nginx/1.30", "1.30.7", Security{Version: "1.30.7", Status: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(filepath.Join(destination, "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = json.Unmarshal(output, &document); err != nil {
		t.Fatal(err)
	}
	if len(document["artifacts"].([]any)) != 1 {
		t.Fatal("missing native package inventory")
	}
	if _, ok := document["sbom"]; ok {
		t.Fatal("release claims a carrier SBOM")
	}
	if _, err := os.Stat(filepath.Join(destination, "sbom.spdx.json")); !os.IsNotExist(err) {
		t.Fatal("carrier SBOM was generated")
	}

	if _, err = APKInfo([]byte("unsupported format")); err == nil {
		t.Fatal("silently accepted unknown APK format")
	}
}
func TestSecurityClassificationDoesNotLeakAcrossVersions(t *testing.T) {
	directory := t.TempDir()
	input := `{"version":"1.2.3","status":"security","advisories":["CVE-2026-12345"],"references":["https://example.org/advisory"]}`
	os.WriteFile(filepath.Join(directory, "security.json"), []byte(input), 0644)
	if _, err := ReadSecurity(directory, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSecurity(directory, "1.2.4"); err == nil {
		t.Fatal("accepted stale classification")
	}
	if err := updateSecurity(directory, "1.2.4"); err != nil {
		t.Fatal(err)
	}
	result, err := ReadSecurity(directory, "1.2.4")
	if err != nil || result.Status != "unknown" || len(result.Advisories) != 0 {
		t.Fatal("carried old security claims to a new version")
	}
	bad := strings.Replace(input, "https://example.org/advisory", "http://example.org/advisory", 1)
	os.WriteFile(filepath.Join(directory, "security.json"), []byte(bad), 0644)
	if _, err = ReadSecurity(directory, "1.2.3"); err == nil {
		t.Fatal("accepted unvalidated advisory link")
	}
}

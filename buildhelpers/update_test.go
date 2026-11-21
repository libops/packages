package buildhelpers

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyAlgorithmsAndTampering(t *testing.T) {
	data := []byte("official archive")
	a := sha256.Sum256(data)
	b := sha512.Sum384(data)
	c := sha512.Sum512(data)
	for _, test := range []struct{ name, sum string }{{"SHA-256", hex.EncodeToString(a[:])}, {"sha384", hex.EncodeToString(b[:])}, {"SHA512", hex.EncodeToString(c[:])}} {
		t.Run(test.name, func(t *testing.T) {
			if err := Verify(data, test.name, test.sum); err != nil {
				t.Fatal(err)
			}
			if Verify([]byte("tampered"), test.name, test.sum) == nil {
				t.Fatal("accepted wrong bytes")
			}
			if Verify(data, test.name, test.sum[:len(test.sum)-1]) == nil {
				t.Fatal("accepted malformed digest")
			}
		})
	}
	if Verify(data, "md5", strings.Repeat("0", 32)) == nil {
		t.Fatal("accepted weak algorithm")
	}
}

func TestLocalSourceChecksumOrder(t *testing.T) {
	root := t.TempDir()
	data := []byte("reviewed upstream test patch\n")
	if err := os.WriteFile(filepath.Join(root, "php84-test.patch"), data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha512.Sum512(data)
	archiveSum := strings.Repeat("a", 128)
	patchEntry := fmt.Sprintf("%x  php84-test.patch", sum)
	prefix := "pkgname=php84\npkgver=8.4.28\nsource=\"https://example.test/php-$pkgver.tar.xz $pkgname-test.patch\"\nsha512sums=\""
	archiveEntry := archiveSum + "  php-8.4.28.tar.xz"
	for _, tc := range []struct {
		name, entries string
		wantError     bool
	}{
		{"valid", archiveEntry + "\n" + patchEntry, false},
		{"reordered", patchEntry + "\n" + archiveEntry, true},
		{"missing", archiveEntry, true},
		{"tampered", archiveEntry + "\n" + archiveSum + "  php84-test.patch", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLocalSources(root, root, prefix+tc.entries+"\"\n")
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %v", err, tc.wantError)
			}
		})
	}
}

func TestOfficialMetadataParsing(t *testing.T) {
	algorithm, sum, err := RubyChecksum("- version: 4.0.1\n  sha512:\n    gz: abcdef\n    xz: deadbeef\n", "4.0.1")
	if err != nil || algorithm != "sha512" || sum != "abcdef" {
		t.Fatalf("%s %s %v", algorithm, sum, err)
	}
	if _, _, err = RubyChecksum("- version: 4.0.10\n  sha512:\n    gz: abcdef\n", "4.0.1"); err == nil {
		t.Fatal("matched different release")
	}
	for _, recipe := range []string{"pkgver=1.2.3\nsha256sums=\"SKIP  source.tar.gz\"\n", "pkgver=1.2.3\nsha256sums=\"abc  source.tar.gz\"\n"} {
		if _, err := SourcePins(recipe); err == nil {
			t.Fatal("accepted missing or malformed source pin")
		}
	}
	if _, err := SourcePins("pkgver=1.2.3\nsha256sums=\"" + strings.Repeat("a", 64) + "  source.tar.gz\"\n"); err != nil {
		t.Fatal(err)
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUpdaterVerifiesBeforeWriting(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			root := t.TempDir()
			target := "alpine/3.24/php/8.4"
			directory := filepath.Join(root, "packages", target)
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			inventory, _ := json.Marshal(map[string]Package{"php": {Target: target, Family: "php", Line: "8.4", SupportedUntil: "2099-12-31", Upstream: "https://www.php.net/supported-versions.php"}})
			os.WriteFile(filepath.Join(root, "packages.json"), inventory, 0644)
			recipe := "pkgname=php84\npkgver=8.4.99\npkgrel=2\nsource=\"https://www.php.net/distributions/php-8.4.98.tar.xz\"\nsha512sums=\"\n" + strings.Repeat("a", 128) + "  php-8.4.98.tar.xz\n\"\n"
			path := filepath.Join(directory, "APKBUILD")
			os.WriteFile(path, []byte(recipe), 0644)
			archive := []byte("official archive")
			signature := signTestArchive(t, directory, archive)
			sum := sha256.Sum256(archive)
			expected := hex.EncodeToString(sum[:])
			if bad {
				expected = strings.Repeat("0", 64)
			}
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				var body string
				switch r.URL.Host + r.URL.Path {
				case "www.php.net/releases/index.php":
					body = `{"source":[{"filename":"php-8.4.99.tar.xz","sha256":"` + expected + `"}]}`
				case "www.php.net/distributions/php-8.4.99.tar.xz.asc":
					body = string(signature)
				case "www.php.net/distributions/php-8.4.99.tar.xz":
					body = string(archive)
				default:
					t.Fatalf("unexpected download %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			err := (Updater{Client: client}).Update(root, "php")
			after, _ := os.ReadFile(path)
			if bad {
				if err == nil || string(after) != recipe {
					t.Fatal("bad download modified recipe")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(after), "pkgrel=0") || !strings.Contains(string(after), "php-8.4.99.tar.xz") {
				t.Fatal("version/revision not updated")
			}
			if err = CheckPins(root); err != nil {
				t.Fatal(err)
			}
			revision := strings.Replace(string(after), "pkgrel=0", "pkgrel=3", 1)
			os.WriteFile(path, []byte(revision), 0644)
			if err = (Updater{Client: client}).Update(root, "php"); err != nil {
				t.Fatal(err)
			}
			refreshed, _ := os.ReadFile(path)
			if !strings.Contains(string(refreshed), "pkgrel=3") {
				t.Fatal("same-version refresh reset packaging revision")
			}
			// Renovate may propose the next minor, but the updater must refuse it unchanged.
			next := strings.ReplaceAll(string(after), "pkgver=8.4.99", "pkgver=8.5.0")
			os.WriteFile(path, []byte(next), 0644)
			if err = (Updater{Client: client}).Update(root, "php"); err == nil {
				t.Fatal("minor line migration accepted")
			}
			unchanged, _ := os.ReadFile(path)
			if string(unchanged) != next {
				t.Fatal("rejected update changed files")
			}
		})
	}
}

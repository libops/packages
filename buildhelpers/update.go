package buildhelpers

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Package struct {
	Target              string          `json:"target"`
	Family              string          `json:"family"`
	Line                string          `json:"line"`
	SupportedUntil      string          `json:"supported_until"`
	SupportDeadlineNote string          `json:"support_deadline_note,omitempty"`
	Upstream            string          `json:"upstream"`
	SourceIdentity      *SourceIdentity `json:"source_identity,omitempty"`
}

type Updater struct {
	Client  *http.Client
	Context context.Context
}

func (u Updater) fetch(url string) ([]byte, error) {
	return FetchMetadata(u.baseContext(), u.Client, url, nil)
}

func Load(root, name string) (Package, error) {
	inventory, err := ReadInventory(root)
	if err != nil {
		return Package{}, err
	}
	for id, record := range inventory {
		if id == name || record.Target == name {
			return record.Package, ValidatePackage(record.Package)
		}
	}
	return Package{}, fmt.Errorf("unknown target %q", name)
}

func field(recipe, key string) (string, error) {
	match := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `=(\S+)$`).FindStringSubmatch(recipe)
	if match == nil {
		return "", fmt.Errorf("missing recipe field %s", key)
	}
	return match[1], nil
}
func setField(recipe, key, value string) string {
	return regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(key)+`=.*$`).ReplaceAllString(recipe, key+"="+value)
}
func replaceChecksum(recipe, filename, checksum string) (string, error) {
	pattern := regexp.MustCompile(`(?m)([a-f0-9]{128})  ` + regexp.QuoteMeta(filename) + `(?:"|$)`)
	match := pattern.FindStringSubmatchIndex(recipe)
	if match == nil {
		return "", fmt.Errorf("missing checksum for %s", filename)
	}
	return recipe[:match[2]] + checksum + recipe[match[3]:], nil
}

// SourcePins derives build arguments from the native recipe's source lock.
func SourcePins(recipe string) (map[string]string, error) {
	version, err := field(recipe, "pkgver")
	if err != nil {
		return nil, err
	}
	match := regexp.MustCompile(`(?m)^sha(256|512)sums="\s*([a-f0-9]+)  ([^\s"]+)`).FindStringSubmatch(recipe)
	if match == nil {
		return nil, fmt.Errorf("recipe must define a strong source checksum")
	}
	bits, _ := strconv.Atoi(match[1])
	if len(match[2]) != bits/4 {
		return nil, fmt.Errorf("malformed source digest")
	}
	return map[string]string{"SOURCE_VERSION": version, "SOURCE_ALGORITHM": "sha" + match[1], "SOURCE_CHECKSUM": match[2], "SOURCE_ARCHIVE": match[3]}, nil
}

func CheckPins(root string) error {
	inventory, err := ReadInventory(root)
	if err != nil {
		return err
	}
	for _, record := range inventory {
		p := record.Package
		if err := ValidatePackage(p); err != nil {
			return err
		}
		directory := filepath.Join(root, "packages", p.Target)
		recipe, err := os.ReadFile(filepath.Join(directory, "APKBUILD"))
		if err != nil {
			return err
		}
		_, err = SourcePins(string(recipe))
		if err != nil {
			return err
		}
		version, err := field(string(recipe), "pkgver")
		if err != nil {
			return err
		}
		if _, err = ReadSecurity(directory, version); err != nil {
			return err
		}
	}
	return nil
}

func (u Updater) Update(root, name string) error {
	p, err := Load(root, name)
	if err != nil {
		return err
	}
	if err = CheckSupport(p, time.Now()); err != nil {
		return err
	}
	directory := filepath.Join(root, "packages", p.Target)
	path := filepath.Join(directory, "APKBUILD")
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	recipe := string(original)
	version, err := field(recipe, "pkgver")
	if err != nil {
		return err
	}
	sourceMatch := regexp.MustCompile(`(?m)^sha512sums="\s*[a-f0-9]{128}  ([^\s"]+)`).FindStringSubmatch(recipe)
	if sourceMatch == nil {
		return fmt.Errorf("updater currently requires a native sha512sums source entry")
	}
	oldFilename := sourceMatch[1]

	if p.Family == "php" || p.Family == "python" || p.Family == "ruby" || p.Family == "go" {
		if !regexp.MustCompile(`^` + regexp.QuoteMeta(p.Line) + `\.[0-9]+$`).MatchString(version) {
			return fmt.Errorf("version %s must remain in supported line %s", version, p.Line)
		}
	}
	src, err := u.discoverSource(p, recipe, version)
	if err != nil {
		return err
	}
	filename, url, algorithm, expected := src.filename, src.url, src.algorithm, src.expected
	recipe = src.recipe

	if p.Family != "nginx" && p.Family != "python" && expected == "" {
		return fmt.Errorf("official metadata has no checksum for exact source %s", filename)
	}
	archive, err := u.download(url)
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	if p.Family == "nginx" || p.Family == "php" {
		signature, e := u.fetch(url + ".asc")
		if e != nil {
			return e
		}
		if e = verifySignature(u.baseContext(), directory, archive.Name(), signature); e != nil {
			return e
		}
	} else if p.Family == "python" {
		materials, e := u.fetch(url + ".sigstore")
		if e != nil {
			return e
		}
		if e = u.verifyPython(archive, materials, p.SourceIdentity); e != nil {
			return e
		}
	}
	if expected != "" {
		if _, err = archive.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err = VerifyReader(archive, algorithm, expected); err != nil {
			return err
		}
	}
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	sum := sha512.New()
	if _, err = io.Copy(sum, archive); err != nil {
		return err
	}

	recipe, err = replaceChecksum(recipe, oldFilename, hex.EncodeToString(sum.Sum(nil)))
	if err != nil {
		return err
	}
	recipe = strings.ReplaceAll(recipe, oldFilename, filename)
	if oldFilename != filename {
		recipe = setField(recipe, "pkgrel", "0")
	}
	return writeRecipe(directory, recipe)
}

func verifySignature(ctx context.Context, directory, archive string, signature []byte) error {
	temporary, err := os.MkdirTemp("", "packages-signature-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err = os.WriteFile(filepath.Join(temporary, "source.asc"), signature, 0600); err != nil {
		return err
	}
	// Public-key import and verification need no agent or Unix socket.
	args := []string{"--batch", "--no-autostart", "--homedir", temporary, "--import", filepath.Join(directory, "release-keys.asc")}
	command, cancel := CommandContext(ctx, time.Minute, "gpg", args...)
	defer cancel()
	command.Stderr = os.Stderr
	if err = command.Run(); err != nil {
		return err
	}
	command, cancel = CommandContext(ctx, time.Minute, "gpg", "--batch", "--no-autostart", "--homedir", temporary, "--verify", filepath.Join(temporary, "source.asc"), archive)
	defer cancel()
	command.Stderr = os.Stderr
	return command.Run()
}

func writeRecipe(directory, recipe string) error {
	if _, err := SourcePins(recipe); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "APKBUILD"), []byte(recipe), 0644); err != nil {
		return err
	}
	version, err := field(recipe, "pkgver")
	if err != nil {
		return err
	}
	return updateSecurity(directory, version)
}

func (u Updater) updateJava(directory, recipe string, p Package) error {
	return fmt.Errorf("Java automatic refresh disabled: source commit and documentation require independently verified provenance and human review")
}

// CheckSupport is scoped to an individual target. Catalogs remain readable after
// expiry, so unrelated builds, cleanup and retirement reporting keep working.
func CheckSupport(p Package, now time.Time) error {
	end, err := time.Parse("2006-01-02", p.SupportedUntil)
	if err != nil {
		return fmt.Errorf("%s needs a valid support deadline", p.Target)
	}
	if now.UTC().Format("2006-01-02") > end.Format("2006-01-02") {
		return fmt.Errorf("%s support deadline expired: retire or review upstream support", p.Target)
	}
	return nil
}

// download streams source archives to private temporary files; metadata has a
// separate small in-memory limit. Partial/error downloads are always removed.
func (u Updater) download(url string) (file *os.File, err error) {
	ctx := u.baseContext()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := u.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{URL: url, StatusCode: response.StatusCode}
	}
	file, err = os.CreateTemp("", "packages-source-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			file.Close()
			os.Remove(file.Name())
		}
	}()
	const limit = 1 << 30
	count, err := io.Copy(file, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return file, err
	}
	if count > limit {
		return file, fmt.Errorf("source exceeds download limit")
	}
	_, err = file.Seek(0, io.SeekStart)
	return file, err
}

func (u Updater) baseContext() context.Context {
	if u.Context != nil {
		return u.Context
	}
	return context.Background()
}

package buildhelpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Security records a reviewed classification for one exact upstream package version.
type Security struct {
	Version    string   `json:"version"`
	Status     string   `json:"status"`
	Advisories []string `json:"advisories"`
	References []string `json:"references"`
}

func ReadSecurity(directory, version string) (Security, error) {
	result := Security{Version: version, Status: "unknown", Advisories: []string{}, References: []string{}}
	data, err := os.ReadFile(filepath.Join(directory, "security.json"))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.Version != version {
		return result, fmt.Errorf("security classification is for a different version")
	}
	if result.Status != "unknown" && result.Status != "maintenance" && result.Status != "security" {
		return result, fmt.Errorf("invalid security status")
	}
	if result.Status != "unknown" && len(result.References) == 0 {
		return result, fmt.Errorf("reviewed security classification needs upstream references")
	}
	for _, id := range result.Advisories {
		if !regexp.MustCompile(`^(CVE-[0-9]{4}-[0-9]{4,}|GHSA-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4})$`).MatchString(id) {
			return result, fmt.Errorf("invalid advisory ID %s", id)
		}
	}
	for _, reference := range result.References {
		u, e := url.Parse(reference)
		if e != nil || u.Scheme != "https" || u.Host == "" {
			return result, fmt.Errorf("advisory references must be HTTPS URLs")
		}
	}
	return result, nil
}

func updateSecurity(directory, version string) error {
	existing, err := os.ReadFile(filepath.Join(directory, "security.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var old Security
	if len(existing) > 0 {
		if err = json.Unmarshal(existing, &old); err != nil {
			return err
		}
		if old.Version == version {
			_, err = ReadSecurity(directory, version)
			return err
		}
	}
	data, _ := json.MarshalIndent(Security{Version: version, Status: "unknown", Advisories: []string{}, References: []string{}}, "", "  ")
	return os.WriteFile(filepath.Join(directory, "security.json"), append(data, '\n'), 0644)
}

// APKInfo reads native APK v2 control metadata, including signed multi-gzip APKs.
// Unsupported formats fail explicitly rather than emitting incomplete release metadata.
func APKInfo(data []byte) (map[string][]string, error) {
	input := bytes.NewReader(data)
	for stream := 0; input.Len() > 0 && stream < 8; stream++ {
		gz, err := gzip.NewReader(input)
		if err != nil {
			return nil, fmt.Errorf("unsupported APK archive format: %w", err)
		}
		gz.Multistream(false)
		archive := tar.NewReader(gz)
		for {
			header, err := archive.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				gz.Close()
				return nil, err
			}
			if strings.TrimPrefix(header.Name, "./") == ".PKGINFO" {
				if header.Size > 1<<20 {
					gz.Close()
					return nil, fmt.Errorf("APK metadata too large")
				}
				metadata, err := io.ReadAll(io.LimitReader(archive, 1<<20))
				gz.Close()
				if err != nil {
					return nil, err
				}
				fields := map[string][]string{}
				for _, line := range strings.Split(string(metadata), "\n") {
					key, value, ok := strings.Cut(line, " = ")
					if ok {
						fields[key] = append(fields[key], value)
					}
				}
				for _, key := range []string{"pkgname", "pkgver", "arch"} {
					if len(fields[key]) != 1 {
						return nil, fmt.Errorf("missing/ambiguous APK field %s", key)
					}
				}
				return fields, nil
			}
		}
		if _, err = io.Copy(io.Discard, gz); err != nil {
			gz.Close()
			return nil, err
		}
		gz.Close()
	}
	return nil, fmt.Errorf("APK has no native .PKGINFO")
}

// WriteMetadata records delivered archive identities and corresponding-source hashes.
func WriteMetadata(apks, sources, destination, target, version string, security Security) error {
	files, err := filepath.Glob(filepath.Join(apks, "*.apk"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("cannot publish an empty package inventory")
	}
	if _, err := TargetID(target); err != nil {
		return err
	}
	var artifacts []any
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := APKInfo(data)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		name, revision, arch := info["pkgname"][0], info["pkgver"][0], info["arch"][0]
		artifacts = append(artifacts, map[string]any{"filename": filepath.Base(path), "sha256": digest, "name": name, "version": revision, "architecture": arch, "requires": info["depend"]})
	}
	bundles, err := filepath.Glob(filepath.Join(sources, "*.src.tar.gz"))
	if err != nil {
		return err
	}
	if len(bundles) == 0 {
		return fmt.Errorf("cannot publish without a corresponding-source bundle")
	}
	sourceArtifacts := []map[string]string{}
	for _, path := range bundles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		sourceArtifacts = append(sourceArtifacts, map[string]string{"filename": filepath.Base(path), "sha256": hex.EncodeToString(sum[:])})
	}
	release := map[string]any{"schema_version": 1, "sources": sourceArtifacts, "target": target, "version": version, "security": security, "artifacts": artifacts}
	if err = os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	for name, value := range map[string]any{"release.json": release} {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(destination, name), append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

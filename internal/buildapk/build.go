// Package buildapk runs the native Alpine build commands after validating pins.
package buildapk

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/config"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func Validate(recipe string, env func(string) string) error {
	if env("EXPECTED_BASE_IMAGE") == "" || env("PACKAGES_BASE_IMAGE") != env("EXPECTED_BASE_IMAGE") {
		return fmt.Errorf("utility image base differs from target distro/base digest")
	}
	version := env("SOURCE_VERSION")
	algorithm := env("SOURCE_ALGORITHM")
	digest := env("SOURCE_CHECKSUM")
	archive := env("SOURCE_ARCHIVE")
	if version == "" || digest == "" || archive == "" {
		return fmt.Errorf("Dockerfile must pin SOURCE_VERSION, source checksum and SOURCE_ARCHIVE")
	}
	length := 0
	switch algorithm {
	case "sha256":
		length = 64
	case "sha512":
		length = 128
	default:
		return fmt.Errorf("unsupported native checksum algorithm %q", algorithm)
	}
	if !regexp.MustCompile(fmt.Sprintf(`^[a-f0-9]{%d}$`, length)).MatchString(digest) {
		return fmt.Errorf("malformed source checksum pin")
	}
	if !regexp.MustCompile(`(?m)^pkgver=` + regexp.QuoteMeta(version) + `$`).MatchString(recipe) {
		return fmt.Errorf("source version pin differs from APKBUILD")
	}
	block := regexp.MustCompile(`(?ms)^` + algorithm + `sums="([^"]*)"`).FindStringSubmatch(recipe)
	if block == nil {
		return fmt.Errorf("missing native checksum block")
	}
	for _, line := range strings.Split(block[1], "\n") {
		if strings.TrimSpace(line) == digest+"  "+archive {
			return nil
		}
	}
	return fmt.Errorf("source checksum pin differs from APKBUILD")
}

func command(ctx context.Context, name string, args ...string) error {
	cmd, cancel := buildhelpers.CommandContext(ctx, 6*time.Hour, name, args...)
	defer cancel()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run(ctx context.Context, cfg config.Environment) error {
	return runBuild("/home/builder", cfg, func(name string, args ...string) error { return command(ctx, name, args...) })
}

func runBuild(home string, cfg config.Environment, execute func(string, ...string) error) error {
	original, err := os.ReadFile("APKBUILD")
	if err != nil {
		return err
	}
	if err = Validate(string(original), cfg.Get); err != nil {
		return err
	}
	// abuild-keygen otherwise uses an empty packager plus second-resolution time.
	// A random build identity prevents different carriers from sharing key filenames.
	id, err := buildhelpers.TargetID(cfg.Get("PACKAGE_TARGET"))
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(home, ".abuild"), 0700); err != nil {
		return err
	}
	config, err := os.OpenFile(filepath.Join(home, ".abuild", "abuild.conf"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintf(config, "\nPACKAGER=\"LibOps <%s-%x@keys.libops.invalid>\"\n", id, nonce)
	closeErr := config.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	defer func() {
		keys, _ := filepath.Glob(filepath.Join(home, ".abuild", "*.rsa"))
		for _, key := range keys {
			os.Remove(key)
		}
	}()
	if err = execute("abuild-keygen", "-ain"); err != nil {
		return err
	}
	if err = execute("abuild-apk", "update"); err != nil {
		return err
	}
	if err = execute("abuild", "fetch", "verify"); err != nil {
		return err
	}
	if err = os.WriteFile("APKBUILD", append(append([]byte{}, original...), []byte("\noptions=\"$options !check\"\n")...), 0644); err != nil {
		return err
	}
	defer os.WriteFile("APKBUILD", original, 0644)
	if err = execute("abuild", "-r", "-K"); err != nil {
		return err
	}
	if err = os.WriteFile("APKBUILD", original, 0644); err != nil {
		return err
	}
	if err = execute("abuild", "-r", "deps"); err != nil {
		return err
	}
	// Native source bundles include the exact upstream archives, APKBUILD, patches
	// and install scripts. Keep them alongside the distributable binary packages.
	if err = execute("abuild", "srcpkg"); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(home, "sources"), 0755); err != nil {
		return err
	}
	bundles, err := filepath.Glob(filepath.Join(home, "packages/src/*.src.tar.gz"))
	if err != nil {
		return err
	}
	if len(bundles) == 0 {
		return fmt.Errorf("build produced no corresponding-source bundle")
	}
	for _, bundle := range bundles {
		source, err := os.Open(bundle)
		if err != nil {
			return err
		}
		destination, err := os.Create(filepath.Join(filepath.Join(home, "sources"), filepath.Base(bundle)))
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(destination, source)
		source.Close()
		closeErr := destination.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err = os.MkdirAll(filepath.Join(home, "apks"), 0755); err != nil {
		return err
	}
	count := 0
	err = filepath.WalkDir(filepath.Join(home, "packages"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".apk") {
			return nil
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()
		dest, err := os.Create(filepath.Join(filepath.Join(home, "apks"), entry.Name()))
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dest, source)
		closeErr := dest.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		count++
		return nil
	})
	if err == nil && count == 0 {
		return fmt.Errorf("build produced no APKs")
	}
	if err != nil {
		return err
	}
	if err := writeIndex(filepath.Join(home, "apks"), execute); err != nil {
		return err
	}
	version := cfg.Get("SOURCE_VERSION")
	security, err := buildhelpers.ReadSecurity(".", version)
	if err != nil {
		return err
	}
	return buildhelpers.WriteMetadata(filepath.Join(home, "apks"), filepath.Join(home, "sources"), filepath.Join(home, "metadata"), cfg.Get("PACKAGE_TARGET"), version, security)
}

func writeIndex(directory string, execute func(string, ...string) error) error {
	apks, err := filepath.Glob(filepath.Join(directory, "*.apk"))
	if err != nil {
		return err
	}
	if len(apks) == 0 {
		return fmt.Errorf("cannot index an empty repository")
	}
	index := filepath.Join(directory, "APKINDEX.tar.gz")
	args := append([]string{"index", "-o", index}, apks...)
	if err := execute("apk", args...); err != nil {
		return err
	}
	if err := execute("abuild-sign", index); err != nil {
		return err
	}
	info, err := os.Stat(index)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("empty signed APK index")
	}
	return execute("apk", "verify", "--keys-dir", "/etc/apk/keys", index)
}

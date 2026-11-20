package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/libops/packages/internal/auth"
	"github.com/libops/packages/internal/catalog"
)

func env(cfg config.Environment, key, fallback string) string {
	if value := cfg.Get(key); value != "" {
		return value
	}
	return fallback
}
func environment(cfg config.Environment) map[string]string { return cfg.Copy() }

func run(ctx context.Context, input []byte, args ...string) error {
	cmd, cancel := buildhelpers.CommandContext(ctx, 6*time.Hour, args[0], args[1:]...)
	defer cancel()
	cmd.Stdin = os.Stdin
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func output(ctx context.Context, args ...string) (string, error) {
	cmd, cancel := buildhelpers.CommandContext(ctx, time.Minute, args[0], args[1:]...)
	defer cancel()
	data, err := cmd.Output()
	return strings.TrimSpace(string(data)), err
}
func SHA(ctx context.Context, cfg config.Environment) (string, error) {
	sha := cfg.Get("GITHUB_SHA")
	var err error
	if sha == "" {
		sha, err = output(ctx, "git", "rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
	}
	if !regexp.MustCompile(`^[a-f0-9]{7,64}$`).MatchString(sha) {
		return "", fmt.Errorf("expected Git commit SHA")
	}
	return sha, nil
}
func reference(ctx context.Context, cfg config.Environment, p catalog.Metadata) (string, error) {
	sha, err := SHA(ctx, cfg)
	if err != nil {
		return "", err
	}
	scope := "candidate"
	if run := cfg.Get("GITHUB_RUN_ID"); run != "" {
		attempt := cfg.Get("GITHUB_RUN_ATTEMPT")
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(run) || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(attempt) {
			return "", fmt.Errorf("invalid workflow run identity")
		}
		scope += "-run-" + run + "-" + attempt
	}
	if pr := cfg.Get("PR"); pr != "" {
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(pr) {
			return "", fmt.Errorf("PR must be positive")
		}
		scope = "pr-" + pr
	}
	return env(cfg, "REPOSITORY", "ghcr.io/libops") + "/" + p.Image + ":" + scope + "-" + p.ID + "-" + sha, nil
}
func result(cfg config.Environment, key, value string) error {
	fmt.Println(key + "=" + value)
	if path := cfg.Get("GITHUB_OUTPUT"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString(key + "=" + value + "\n")
		return err
	}
	return nil
}
func localReference(cfg config.Environment, p catalog.Metadata) string {
	return env(cfg, "REPOSITORY", "ghcr.io/libops") + "/" + p.Image + ":local-" + p.ID + "-" + env(cfg, "ARCH", runtime.GOARCH)
}
func bake(ctx context.Context, cfg config.Environment, c catalog.Catalog, p catalog.Metadata, publish, test bool) error {
	image := localReference(cfg, p)
	if publish {
		ref, err := reference(ctx, cfg, p)
		if err != nil {
			return err
		}
		image = ref + "-" + env(cfg, "ARCH", runtime.GOARCH)
	}
	values := environment(cfg)
	values["PUBLISH"] = "0"
	if publish {
		values["PUBLISH"] = "1"
	}
	values["IMAGE_REF"] = image
	plan, err := PlanFor(".", c, p.ID, values)
	if err != nil {
		return err
	}
	if test {
		target := plan["target"]["package"]
		target["target"] = "test"
		delete(target, "cache-to")
		delete(target, "tags")
		delete(target, "attest")
		target["output"] = []string{"type=cacheonly"}
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	args := []string{"docker", "buildx", "bake", "-f", "-", "package", "--progress", "plain"}
	if !test {
		if publish {
			args = append(args, "--push")
		} else {
			args = append(args, "--load")
		}
	}
	return run(ctx, data, args...)
}
func Task(ctx context.Context, cfg config.Environment, command string) error {
	repository := env(cfg, "REPOSITORY", "ghcr.io/libops")
	if command == "login" {
		return auth.Login(ctx, cfg, repository)
	}
	c, err := catalog.Load(".")
	if err != nil {
		return err
	}
	name := cfg.Get("TARGET")
	if name == "" {
		return fmt.Errorf("set TARGET=distro/release/software/line")
	}
	p, err := c.Metadata(".", name)
	if err != nil {
		return err
	}
	switch command {
	case "build":
		return bake(ctx, cfg, c, p, false, false)
	case "publish":
		if err = buildhelpers.CheckSupport(p.Package.Package, time.Now()); err != nil {
			return err
		}
		if err = auth.Login(ctx, cfg, repository); err != nil {
			return err
		}
		if err := publicationPreflight(ctx, cfg, p, repository); err != nil {
			return err
		}
		if err := validateDependencies(ctx, cfg, c, p, repository); err != nil {
			return err
		}
		return bake(ctx, cfg, c, p, true, false)
	case "manifest":
		if err = buildhelpers.CheckSupport(p.Package.Package, time.Now()); err != nil {
			return err
		}
		if err = auth.Login(ctx, cfg, repository); err != nil {
			return err
		}
		ref, err := reference(ctx, cfg, p)
		if err != nil {
			return err
		}
		args := []string{"docker", "buildx", "imagetools", "create", "-t", ref}
		profiles, err := c.Platforms(".", p.ID)
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			args = append(args, ref+"-"+profile.Arch)
		}
		if err = run(ctx, nil, args...); err != nil {
			return err
		}
		digest, err := output(ctx, "docker", "buildx", "imagetools", "inspect", ref, "--format", "{{.Manifest.Digest}}")
		if err != nil {
			return err
		}
		if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(digest) {
			return fmt.Errorf("missing published digest")
		}
		if releasing(cfg) {
			revision := repository + "/" + p.Image + ":" + p.RevisionTag
			existing, err := ReleaseDigest(ctx, cfg, revision)
			if err != nil {
				return err
			}
			if promotion(existing, digest) {
				if existing == "" {
					if err := run(ctx, nil, "docker", "buildx", "imagetools", "create", "-t", revision, ref+"@"+digest); err != nil {
						return err
					}
				}
				published, err := ReleaseDigest(ctx, cfg, revision)
				if err != nil {
					return err
				}
				if published != digest {
					return fmt.Errorf("release digest does not match candidate: %s", revision)
				}
				if err := updateAlias(ctx, cfg, p, repository, digest); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%s already exists with different bytes: bump pkgrel", revision)
			}
		}
		return result(cfg, "context", "docker-image://"+repository+"/"+p.Image+"@"+digest)
	case "test":
		if cfg.Get("CI") != "" || cfg.Get("PACKAGE_REF") != "" {
			if err = auth.Login(ctx, cfg, repository); err != nil {
				return err
			}
		}
		if err = bake(ctx, cfg, c, p, false, true); err != nil {
			return err
		}
		context := cfg.Get("PACKAGE_REF")
		if context == "" {
			context = "docker-image://" + localReference(cfg, p)
		}

		args := []string{"docker", "buildx", "build", "--progress", "plain", "--no-cache"}
		// --load places the local carrier in the current Docker daemon. Its default
		// builder can read it even when the source build used a docker-container driver.
		if cfg.Get("PACKAGE_REF") == "" {
			args = append(args, "--builder", "default")
		}
		args = append(args, "--build-context", "alpine=docker-image://"+p.Base, "--build-context", "packages="+context, "--build-arg", "BASE_IMAGE="+p.Base, "--build-arg", "PACKAGE="+p.Test, "-f", "test.Dockerfile", ".")
		return run(ctx, nil, args...)

	}
	return fmt.Errorf("unknown build task %q", command)
}
func Builder(ctx context.Context, cfg config.Environment, command string) error {
	c, err := catalog.Load(".")
	if err != nil {
		return err
	}
	if command == "builder-plan" {
		matrix := []map[string]string{}
		for _, base := range c.Bases() {
			profiles, err := c.BuilderPlatforms(".", base)
			if err != nil {
				return err
			}
			data, err := json.Marshal(profiles)
			if err != nil {
				return err
			}
			matrix = append(matrix, map[string]string{"target": base, "platforms": string(data)})
		}
		data, err := json.Marshal(matrix)
		if err == nil {
			fmt.Println(string(data))
		}
		return err
	}
	target := cfg.Get("TARGET")
	found := false
	for _, base := range c.Bases() {
		found = found || base == target
	}
	if !found {
		return fmt.Errorf("TARGET must be a registered distro/release")
	}
	profiles, err := c.BuilderPlatforms(".", target)
	if err != nil {
		return err
	}
	sha, err := SHA(ctx, cfg)
	if err != nil {
		return err
	}
	repository := env(cfg, "REPOSITORY", "ghcr.io/libops")
	version := strings.ReplaceAll(target, "/", "")
	ref := repository + "/package-builder:" + version + "-" + sha
	if command == "builder-build" {
		arch := env(cfg, "ARCH", runtime.GOARCH)
		platform := ""
		for _, profile := range profiles {
			if profile.Arch == arch {
				platform = profile.Platform
			}
		}
		if platform == "" {
			return fmt.Errorf("architecture %s is not enabled for builder %s", arch, target)
		}
		publish := cfg.Get("PUBLISH") == "1"
		mode := "--load"
		if publish {
			if err = auth.Login(ctx, cfg, repository); err != nil {
				return err
			}
			mode = "--push"
		}
		base, err := os.ReadFile(filepath.Join("packages", target, "base-image.txt"))
		if err != nil {
			return err
		}
		pin := strings.TrimSpace(string(base))
		return run(ctx, nil, "docker", "buildx", "build", "--progress", "plain", "--target", "tools", "--label", "org.opencontainers.image.source=https://github.com/"+env(cfg, "GITHUB_REPOSITORY", "libops/packages"), "--label", "org.opencontainers.image.revision="+sha, "--platform", platform, "--build-arg", "BASE_IMAGE="+pin, "--build-context", "alpine=docker-image://"+pin, "-t", ref+"-"+arch, mode, ".")
	}
	if command == "builder-manifest" {
		if err = auth.Login(ctx, cfg, repository); err != nil {
			return err
		}
		args := []string{"docker", "buildx", "imagetools", "create", "-t", ref, "-t", repository + "/package-builder:" + version}
		for _, profile := range profiles {
			args = append(args, ref+"-"+profile.Arch)
		}
		if err = run(ctx, nil, args...); err != nil {
			return err
		}

		digest, err := output(ctx, "docker", "buildx", "imagetools", "inspect", ref, "--format", "{{.Manifest.Digest}}")
		if err != nil {
			return err
		}
		if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(digest) {
			return fmt.Errorf("invalid builder digest")
		}
		return result(cfg, "digest", digest)
	}
	return fmt.Errorf("unknown builder task")
}

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/config"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/libops/packages/internal/catalog"
)

type Selection struct {
	Targets      []string
	LocalBuilder bool
}

func Plan(ctx context.Context, cfg config.Environment, root string) ([]string, error) {
	selection, err := Select(ctx, cfg, root)
	return selection.Targets, err
}
func Select(ctx context.Context, cfg config.Environment, root string) (Selection, error) {
	c, err := catalog.Load(root)
	if err != nil {
		return Selection{}, err
	}
	base := cfg.Get("BASE")
	all := cfg.Get("ALL") == "1" || cfg.Get("ALL") == "true" || cfg.Get("BUILD_ALL") == "true"
	git := func(args ...string) (string, error) {
		cmd, cancel := buildhelpers.CommandContext(ctx, time.Minute, "git", args...)
		defer cancel()
		cmd.Dir = root
		data, err := cmd.Output()
		return strings.TrimSpace(string(data)), err
	}
	if !all && base == "" {
		if prBase := cfg.Get("PR_BASE"); prBase != "" {
			base, err = git("merge-base", "HEAD", prBase)
		} else if name := cfg.Get("GITHUB_REF_NAME"); name != "" && name != "main" {
			base, err = git("merge-base", "HEAD", "origin/main")
		} else {
			base = cfg.Get("BEFORE")
		}
		if err != nil {
			return Selection{}, err
		}
	}
	if all || base == "" || strings.Trim(base, "0") == "" {
		return Selection{Targets: c.Names(), LocalBuilder: true}, nil
	}
	// A force-push can remove BEFORE from the history fetched by checkout.
	// Rebuild conservatively only for event-provided push history, never for a
	// caller's explicit BASE or a PR comparison.
	if cfg.Get("GITHUB_EVENT_NAME") == "push" && cfg.Get("BASE") == "" && cfg.Get("PR_BASE") == "" {
		if _, err := git("cat-file", "-e", base+"^{commit}"); err != nil {
			// GitHub retains recent force-pushed commits by object ID even
			// when no branch references them. Recover the actual comparison
			// before falling back, so amendments select only changed inputs.
			if _, fetchErr := git("fetch", "--no-tags", "origin", base); fetchErr != nil {
				fmt.Fprintln(os.Stderr, "Push comparison commit unavailable; selecting all packages.")
				return Selection{Targets: c.Names(), LocalBuilder: true}, nil
			}
		}
	}
	cmd, cancel := buildhelpers.CommandContext(ctx, time.Minute, "git", "diff", "--name-only", "-z", base, "HEAD")
	defer cancel()
	cmd.Dir = root
	files, err := cmd.Output()
	if err != nil {
		return Selection{}, fmt.Errorf("compare package inputs against %s: %w", base, err)
	}
	var previous catalog.Catalog
	old, cancelOld := buildhelpers.CommandContext(ctx, time.Minute, "git", "show", base+":packages.json")
	defer cancelOld()
	old.Dir = root
	if data, e := old.Output(); e == nil {
		if err = json.Unmarshal(data, &previous); err != nil {
			return Selection{}, err
		}
	}
	changed := strings.Split(string(files), "\x00")
	local, validation := false, false
	for _, file := range changed {
		local = local || catalog.HelperInput(file)
		validation = validation || catalog.ValidationInput(file) || file == "packages.json" || strings.HasSuffix(file, "/builder.json")
	}
	targets := c.Affected(changed, previous)
	if err := validateRevisions(ctx, root, base, c, previous, targets); err != nil {
		return Selection{}, err
	}
	// A small real package exercises the current helper in PRs. Main publishes
	// utility changes through builders.yml, without repeating the source matrix.
	if validation && cfg.Get("PR_BASE") != "" {
		for _, name := range c.Names() {
			if c[name].Family == "nginx" {
				found := false
				for _, selected := range targets {
					found = found || selected == name
				}
				if !found {
					targets = append(targets, name)
				}
				break
			}
		}
	}
	sort.Strings(targets)
	return Selection{Targets: targets, LocalBuilder: local}, nil
}
func CheckFormat(ctx context.Context) error {
	cmd, cancel := buildhelpers.CommandContext(ctx, time.Minute, "gofmt", "-l", "cmd", "internal", "buildhelpers")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	if len(output) > 0 {
		return fmt.Errorf("run gofmt on:\n%s", output)
	}
	// Repository-owned CI code is Go/shell. Documentation tooling may use Python.
	return filepath.WalkDir("ci", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".py") {
			return fmt.Errorf("CI Python is not allowed: %s", path)
		}
		return nil
	})
}

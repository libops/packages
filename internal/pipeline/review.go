package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/libops/packages/internal/config"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/catalog"
)

func newer(a, b string) bool {
	left := strings.Split(a, ".")
	right := strings.Split(b, ".")
	for i := range left {
		av, _ := strconv.Atoi(left[i])
		bv := 0
		if i < len(right) {
			bv, _ = strconv.Atoi(right[i])
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}
func Candidates(c catalog.Catalog, available map[string][]string) []string {
	result := []string{}
	for family, lines := range available {
		newest := ""
		for _, p := range c {
			if p.Family == family && (newest == "" || newer(p.Line, newest)) {
				newest = p.Line
			}
		}
		if newest == "" {
			continue
		}
		seen := map[string]bool{}
		for _, line := range lines {
			if newer(line, newest) && !seen[line] {
				result = append(result, family+" "+line)
				seen[line] = true
			}
		}
	}
	sort.Strings(result)
	return result
}
func Review(ctx context.Context, cfg config.Environment, report bool) error {
	c, err := catalog.Load(".")
	if err != nil {
		return err
	}
	var expired []string
	for _, name := range c.Names() {
		if err := buildhelpers.CheckSupport(c[name].Package, time.Now()); err != nil {
			expired = append(expired, c[name].Target+": "+err.Error())
		}
	}
	if len(expired) > 0 {
		body := "Review or retire these targets before further publication. Verify upstream support before extending conservative review deadlines.\n\n- " + strings.Join(expired, "\n- ") + "\n"
		fmt.Print(body)
		if report {
			if err := reportIssue(ctx, cfg, "Retire or review expired package support", body); err != nil {
				return err
			}
		}
	}
	client := &http.Client{Timeout: 60 * time.Second}
	fetch := func(endpoint string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "libops-packages-line-review")
		if req.URL.Host == "api.github.com" && cfg.Get("GITHUB_TOKEN") != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.Get("GITHUB_TOKEN"))
		}
		return buildhelpers.FetchMetadata(ctx, client, endpoint, req.Header)
	}
	available := map[string][]string{}
	add := func(family, version string) {
		if regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
			parts := strings.Split(version, ".")
			available[family] = append(available[family], strings.Join(parts[:2], "."))
		}
	}
	data, err := fetch("https://go.dev/dl/?mode=json")
	if err != nil {
		return err
	}
	var goReleases []struct {
		Version string
		Stable  bool
	}
	if err = json.Unmarshal(data, &goReleases); err != nil {
		return err
	}
	for _, r := range goReleases {
		if r.Stable {
			add("go", strings.TrimPrefix(r.Version, "go"))
		}
	}
	data, err = fetch("https://raw.githubusercontent.com/ruby/www.ruby-lang.org/master/_data/releases.yml")
	if err != nil {
		return err
	}
	rubyReleases, err := buildhelpers.ParseRubyReleases(data)
	if err != nil {
		return err
	}
	for _, release := range rubyReleases {
		add("ruby", release.Version)
	}
	for _, source := range []struct{ family, repo, prefix string }{{"php", "php/php-src", "php-"}, {"python", "python/cpython", "v"}} {
		for page := 1; page <= 10; page++ {
			data, err = fetch(fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=100&page=%d", source.repo, page))
			if err != nil {
				return err
			}
			var tags []struct{ Name string }
			if err = json.Unmarshal(data, &tags); err != nil {
				return err
			}
			for _, tag := range tags {
				add(source.family, strings.TrimPrefix(tag.Name, source.prefix))
			}
			if len(tags) < 100 {
				break
			}
		}
	}
	found := Candidates(c, available)
	if len(found) == 0 {
		fmt.Println("No newer stable runtime lines found; maintenance updates remain Renovate-owned.")
		return nil
	}
	body := "New stable runtime lines were detected. These are setup candidates, not automatic support approvals.\n\n- " + strings.Join(found, "\n- ") + "\n\nConfirm upstream support and distro/bootstrap compatibility, then add a dedicated directory, source pins, tests and inventory entry. Renovate must never migrate an existing line.\n"
	fmt.Print(body)
	if !report {
		return nil
	}
	return reportIssue(ctx, cfg, "Review new supported runtime lines", body)
}

func reportIssue(ctx context.Context, cfg config.Environment, title, body string) error {
	// gh is the platform CLI; arguments and body stdin are passed without shell parsing.
	repository := cfg.Get("GITHUB_REPOSITORY")
	if repository == "" {
		return fmt.Errorf("set GITHUB_REPOSITORY for reporting")
	}
	cmd, cancel := buildhelpers.CommandContext(ctx, time.Minute, "gh", "issue", "list", "--repo", repository, "--state", "open", "--search", title+" in:title", "--limit", "100", "--json", "number,title")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	var issues []struct {
		Number int
		Title  string
	}
	if err = json.Unmarshal(output, &issues); err != nil {
		return err
	}
	args := []string{"issue", "create", "--repo", repository, "--title", title, "--body-file", "-"}
	for _, issue := range issues {
		if issue.Title == title {
			args = []string{"issue", "edit", strconv.Itoa(issue.Number), "--repo", repository, "--body-file", "-"}
			break
		}
	}
	cmd, cancel = buildhelpers.CommandContext(ctx, time.Minute, "gh", args...)
	defer cancel()
	cmd.Stdin = bytes.NewBufferString(body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

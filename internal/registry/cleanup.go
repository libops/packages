// Package registry manages disposable PR artifacts without deleting live manifests.
package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/config"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Version struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Metadata struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}
type Descriptor struct {
	Digest string `json:"digest"`
}
type Manifest struct {
	Config struct {
		MediaType string `json:"mediaType"`
	} `json:"config"`
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []Descriptor `json:"manifests"`
	Subject       *Descriptor  `json:"subject"`
}

func owned(tag, pr string) bool {
	if strings.HasPrefix(pr, "run-") {
		return strings.HasPrefix(tag, "candidate-"+pr+"-")
	}
	return strings.HasPrefix(tag, "pr-"+pr+"-") || strings.HasPrefix(tag, "cache-pr-"+pr+"-")
}
func reachable(roots map[string]bool, read func(string) (Manifest, error)) (map[string]bool, error) {
	result := map[string]bool{}
	queue := []string{}
	for digest := range roots {
		queue = append(queue, digest)
	}
	for len(queue) > 0 {
		digest := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if result[digest] {
			continue
		}
		if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(digest) {
			return nil, fmt.Errorf("malformed manifest digest")
		}
		manifest, err := read(digest)
		if err != nil {
			return nil, err
		}
		result[digest] = true
		for _, child := range manifest.Manifests {
			queue = append(queue, child.Digest)
		}
		if manifest.Subject != nil {
			queue = append(queue, manifest.Subject.Digest)
		}
	}
	return result, nil
}

func Removable(versions []Version, pr string, read func(string) (Manifest, error)) ([]Version, error) {
	if !regexp.MustCompile(`^(?:[1-9][0-9]*|run-[1-9][0-9]*-[1-9][0-9]*)$`).MatchString(pr) {
		return nil, fmt.Errorf("invalid PR number")
	}
	return removable(versions, func(tag string) bool { return owned(tag, pr) }, read)
}
func removable(versions []Version, owns func(string) bool, read func(string) (Manifest, error)) ([]Version, error) {
	seen := map[int]bool{}
	for _, v := range versions {
		if v.ID <= 0 || seen[v.ID] || !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(v.Name) {
			return nil, fmt.Errorf("invalid package version inventory")
		}
		seen[v.ID] = true
	}
	own, live := map[string]bool{}, map[string]bool{}
	for _, version := range versions {
		for _, tag := range version.Metadata.Container.Tags {
			if owns(tag) {
				own[version.Name] = true
			} else {
				live[version.Name] = true
			}
		}
	}
	if len(own) == 0 {
		return nil, nil
	}
	memo := map[string]Manifest{}
	cached := func(digest string) (Manifest, error) {
		if result, ok := memo[digest]; ok {
			return result, nil
		}
		result, err := read(digest)
		if err == nil {
			memo[digest] = result
		}
		return result, err
	}
	disposable, err := reachable(own, cached)
	if err != nil {
		return nil, err
	}
	protected, err := reachable(live, cached)
	if err != nil {
		return nil, err
	}
	var result []Version
	for _, version := range versions {
		// Untagged children may gain references after the inventory snapshot.
		// GHCR has no atomic reachability check/delete; leave these to registry GC.
		if len(version.Metadata.Container.Tags) == 0 || !disposable[version.Name] || protected[version.Name] {
			continue
		}
		onlyOwn := true
		for _, tag := range version.Metadata.Container.Tags {
			if !owns(tag) {
				onlyOwn = false
			}
		}
		if onlyOwn {
			result = append(result, version)
		}
	}
	return result, nil
}

type api struct {
	ctx    context.Context
	token  string
	client *http.Client
}

func (a api) request(endpoint, method string, headers map[string]string, result any) (int, error) {
	req, err := http.NewRequestWithContext(a.ctx, method, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("User-Agent", "libops-packages-cleanup")
	req.Header.Set("Accept", "application/vnd.github+json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return buildhelpers.JSONResponse(a.client, req, result)
}

type completedRun struct {
	Status, Event, Path, Conclusion string
	RunAttempt                      int    `json:"run_attempt"`
	HeadBranch                      string `json:"head_branch"`
	Repository                      struct {
		FullName string `json:"full_name"`
	}
}

func (run completedRun) validate(repository string) error {
	if run.Status != "completed" || run.Conclusion != "success" || (run.Event != "push" && run.Event != "workflow_dispatch" && run.Event != "schedule") || run.HeadBranch != "main" || run.Path != ".github/workflows/push.yml" || !strings.EqualFold(run.Repository.FullName, repository) || run.RunAttempt < 1 {
		return fmt.Errorf("cleanup requires a successful completed main package workflow")
	}
	return nil
}

func Cleanup(ctx context.Context, cfg config.Environment) error {
	pr := cfg.Get("PR")
	run := cfg.Get("CLEANUP_RUN")
	cacheGC := cfg.Get("CACHE_GC") == "1"
	if run != "" {
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(run) {
			return fmt.Errorf("invalid cleanup run")
		}
	} else if !cacheGC && !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(pr) {
		return fmt.Errorf("PR must be a positive number")
	}
	repository := cfg.Get("GITHUB_REPOSITORY")
	if repository == "" {
		repository = "libops/packages"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repository) {
		return fmt.Errorf("invalid GitHub repository")
	}
	destination := cfg.Get("REPOSITORY")
	if destination == "" {
		destination = "ghcr.io/libops"
	}
	owner := strings.Split(repository, "/")[0]
	if destination != "ghcr.io/"+strings.ToLower(owner) {
		return fmt.Errorf("cleanup requires ghcr.io/<repository-owner>; other providers need native deletion/GC")
	}
	token := cfg.Get("GH_TOKEN")
	if token == "" {
		token = cfg.Get("GITHUB_TOKEN")
	}
	if token == "" {
		var err error
		token, err = promptToken(ctx, cfg)
		if err != nil {
			return err
		}
	}
	client := api{ctx: ctx, token: token, client: &http.Client{Timeout: 60 * time.Second}}
	root := "https://api.github.com/repos/" + repository
	var status int
	var err error
	if cacheGC {
		// Cache collection operates only on proven cache manifests after quarantine.
	} else if run != "" {
		var completed completedRun
		status, err = client.request(root+"/actions/runs/"+run, "GET", nil, &completed)
		if err != nil {
			return err
		}
		if status == 404 {
			return fmt.Errorf("cannot establish completed publisher")
		}
		if err := completed.validate(repository); err != nil {
			return err
		}
		pr = fmt.Sprintf("run-%s-%d", run, completed.RunAttempt)
	} else {
		var pull struct {
			State string
			Base  struct{ Ref string }
			Head  struct {
				Ref  string
				Repo struct{ ID int }
			}
		}
		status, err = client.request(root+"/pulls/"+pr, "GET", nil, &pull)
		if err != nil {
			return err
		}
		if status == 404 || pull.State != "closed" || pull.Base.Ref != "main" {
			return fmt.Errorf("cleanup requires a closed PR targeting main")
		}
		// Retry a closing-event race in Go, not a workflow shell state machine.
		deadline := time.Now().Add(5 * time.Minute)
		for {
			active := false
			for _, state := range []string{"queued", "in_progress", "waiting", "pending", "requested"} {
				for page := 1; page <= 10000; page++ {
					var batch struct {
						Runs []struct {
							Pulls      []struct{ Number int } `json:"pull_requests"`
							HeadBranch string                 `json:"head_branch"`
							HeadRepo   struct{ ID int }       `json:"head_repository"`
						} `json:"workflow_runs"`
					}
					status, err = client.request(fmt.Sprintf("%s/actions/workflows/push.yml/runs?event=pull_request&status=%s&per_page=100&page=%d", root, state, page), "GET", nil, &batch)
					if err != nil {
						return err
					}
					if status == 404 || batch.Runs == nil {
						return fmt.Errorf("cannot establish publisher state")
					}
					for _, run := range batch.Runs {
						for _, p := range run.Pulls {
							if fmt.Sprint(p.Number) == pr {
								active = true
							}
						}
						if run.HeadBranch == pull.Head.Ref && run.HeadRepo.ID == pull.Head.Repo.ID {
							active = true
						}
					}
					if len(batch.Runs) < 100 {
						break
					}
					if page == 10000 {
						return fmt.Errorf("workflow pagination limit exceeded")
					}
				}
			}
			if !active {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("PR publisher still active; retry cleanup later")
			}
			fmt.Println("PR publisher still active; waiting before cleanup")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(30 * time.Second):
			}
		}

	}
	var account struct{ Type string }
	if _, err = client.request("https://api.github.com/users/"+owner, "GET", nil, &account); err != nil {
		return err
	}
	if account.Type != "User" && account.Type != "Organization" {
		return fmt.Errorf("cannot establish package owner type")
	}
	scope := "users/" + owner
	if account.Type == "Organization" {
		scope = "orgs/" + owner
	}
	packages, err := buildhelpers.ReadInventory(".")
	if err != nil {
		return err
	}
	images := map[string]bool{}
	for _, p := range packages {
		parts := strings.Split(p.Target, "/")
		if len(parts) != 4 {
			return fmt.Errorf("invalid package target")
		}
		images[parts[2]] = true
	}
	dry := cfg.Get("DRY_RUN") != "0"
	for image := range images {
		endpoint := "https://api.github.com/" + scope + "/packages/container/" + url.PathEscape(image)
		var pkg struct {
			Repository struct {
				FullName string `json:"full_name"`
			}
		}
		status, err = client.request(endpoint, "GET", nil, &pkg)
		if err != nil {
			return err
		}
		if status == 404 {
			continue
		}
		if !strings.EqualFold(pkg.Repository.FullName, repository) {
			return fmt.Errorf("%s: package must be linked to %s", image, repository)
		}
		var versions []Version
		for page := 1; page <= 10000; page++ {
			var batch []Version
			status, err = client.request(fmt.Sprintf("%s/versions?per_page=100&page=%d", endpoint, page), "GET", nil, &batch)
			if err != nil {
				return err
			}
			if status == 404 || batch == nil {
				return fmt.Errorf("cannot establish complete package version inventory")
			}
			versions = append(versions, batch...)
			if len(batch) < 100 {
				break
			}
			if page == 10000 {
				return fmt.Errorf("package pagination limit exceeded")
			}
		}
		hasOwn := false
		for _, v := range versions {
			for _, tag := range v.Metadata.Container.Tags {
				hasOwn = hasOwn || owned(tag, pr)
			}
		}
		if !hasOwn && !cacheGC {
			continue
		}
		credentials := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
		var access struct{ Token string }
		_, err = client.request("https://ghcr.io/token?service=ghcr.io&scope="+url.QueryEscape("repository:"+strings.ToLower(owner)+"/"+image+":pull"), "GET", map[string]string{"Authorization": "Basic " + credentials}, &access)
		if err != nil {
			return err
		}
		if access.Token == "" {
			return fmt.Errorf("missing registry access token")
		}
		read := func(digest string) (Manifest, error) {
			var manifest Manifest
			status, err := client.request("https://ghcr.io/v2/"+strings.ToLower(owner)+"/"+image+"/manifests/"+digest, "GET", map[string]string{"Authorization": "Bearer " + access.Token, "Accept": "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"}, &manifest)
			if status == 404 {
				return manifest, fmt.Errorf("cannot prove reachability of %s", digest)
			}
			if err == nil && (manifest.SchemaVersion != 2 || manifest.MediaType == "") {
				return manifest, fmt.Errorf("invalid manifest response for %s", digest)
			}
			return manifest, err
		}
		if cacheGC {
			if err := collectCaches(ctx, cfg, client, endpoint, owner, image, token, versions, read); err != nil {
				return err
			}
			continue
		}
		candidates, err := Removable(versions, pr, read)
		if err != nil {
			return err
		}
		for _, version := range candidates {
			if dry {
				fmt.Printf("Would delete %s@%s\n", image, version.Name)
				continue
			}
			var fresh Version
			path := fmt.Sprintf("%s/versions/%d", endpoint, version.ID)
			status, err = client.request(path, "GET", nil, &fresh)
			if err != nil {
				return err
			}
			if status == 404 {
				continue
			}
			if err := validateFresh(version, fresh, pr); err != nil {
				return err
			}
			fmt.Printf("Deleting %s@%s\n", image, version.Name)
			if _, err = client.request(path, "DELETE", nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateFresh fails closed if a package version changed after planning.
func validateFresh(planned, fresh Version, pr string) error {
	if len(fresh.Metadata.Container.Tags) == 0 {
		return fmt.Errorf("package version lost tags before deletion")
	}
	if fresh.ID != planned.ID || fresh.Name != planned.Name {
		return fmt.Errorf("package version identity changed before deletion")
	}
	for _, tag := range fresh.Metadata.Container.Tags {
		if !owned(tag, pr) {
			return fmt.Errorf("package version acquired a protected tag before deletion")
		}
	}
	return nil
}

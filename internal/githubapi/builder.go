// Package githubapi records verified builder digests through repository-scoped PRs.
package githubapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/libops/packages/internal/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/auth"
)

func PinBuilder(ctx context.Context, cfg config.Environment, target, digest string, pr bool) error {
	if !regexp.MustCompile(`^(alpine|debian|ubuntu)/[0-9]+(?:\.[0-9]+)?$`).MatchString(target) {
		return fmt.Errorf("invalid builder target")
	}
	if !regexp.MustCompile(`^sha256:[a-f0-9]{64}$`).MatchString(digest) {
		return fmt.Errorf("builder manifest must have a SHA-256 digest")
	}
	if _, err := os.Stat(filepath.Join("packages", target, "base-image.txt")); err != nil {
		return err
	}
	destination := cfg.Get("REPOSITORY")
	if destination == "" {
		destination = "ghcr.io/libops"
	}
	if _, err := auth.Host(destination); err != nil {
		return err
	}
	base, err := os.ReadFile(filepath.Join("packages", target, "base-image.txt"))
	if err != nil {
		return err
	}
	record, err := json.MarshalIndent(map[string]string{"image": destination + "/package-builder:" + strings.ReplaceAll(target, "/", "") + "@" + digest, "base": strings.TrimSpace(string(base))}, "", "  ")
	if err != nil {
		return err
	}
	content := string(record) + "\n"
	path := filepath.ToSlash(filepath.Join("packages", target, "builder.json"))
	if !pr {
		return os.WriteFile(path, []byte(content), 0644)
	}
	repository := cfg.Get("GITHUB_REPOSITORY")
	token := cfg.Get("GH_TOKEN")
	if repository == "" || token == "" {
		return fmt.Errorf("set GITHUB_REPOSITORY and GH_TOKEN (GitHub App installation token)")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repository) {
		return fmt.Errorf("invalid GitHub repository")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	request := func(method, path string, payload any, output any) (int, error) {
		var body io.Reader
		if payload != nil {
			data, err := json.Marshal(payload)
			if err != nil {
				return 0, err
			}
			body = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com/repos/"+repository+path, body)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "libops-packages")
		return buildhelpers.JSONResponse(client, req, output)
	}
	branch := "update-builder-" + strings.NewReplacer("/", "-", ".", "-").Replace(target)
	var ref struct{ Object struct{ SHA string } }
	status, err := request("GET", "/git/ref/heads/"+branch, nil, &ref)
	if err != nil {
		return err
	}
	if status == 404 {
		if _, err = request("GET", "/git/ref/heads/main", nil, &ref); err != nil {
			return err
		}
		if ref.Object.SHA == "" {
			return fmt.Errorf("main branch has no commit")
		}
		if _, err = request("POST", "/git/refs", map[string]string{"ref": "refs/heads/" + branch, "sha": ref.Object.SHA}, nil); err != nil {
			return err
		}
	}
	var existing struct {
		SHA     string
		Content string
	}
	_, err = request("GET", "/contents/"+path+"?ref="+branch, nil, &existing)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	current, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(existing.Content, "\n", ""))
	if err != nil {
		return fmt.Errorf("invalid existing builder pin content: %w", err)
	}
	payload := map[string]string{"message": "[minor] Update " + target + " package builder", "content": encoded, "branch": branch}
	if existing.SHA != "" {
		payload["sha"] = existing.SHA
	}
	if string(current) != content {
		if _, err = request("PUT", "/contents/"+path, payload, nil); err != nil {
			return err
		}
	}
	var pulls []struct{ Number int }
	if _, err = request("GET", "/pulls?state=open&head="+strings.Split(repository, "/")[0]+":"+branch, nil, &pulls); err != nil {
		return err
	}
	if len(pulls) > 0 {
		return nil
	}
	_, err = request("POST", "/pulls", map[string]string{"title": "Update " + target + " package builder", "head": branch, "base": "main", "body": "Pin the separately published builder by immutable digest. Package builds consume this exact utility/toolchain image; changes are selected within this distro release. Upstream/package tests remain required before merge."}, nil)
	return err
}

package pipeline

import (
	"context"
	"fmt"
	"github.com/libops/packages/internal/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReviewLinesPaginationErrorsAndIssueDeduplication(t *testing.T) {
	for _, scenario := range []string{"new issue", "existing issue", "upstream failure"} {
		t.Run(scenario, func(t *testing.T) {
			root, err := filepath.Abs("../..")
			if err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			dir := t.TempDir()
			script := `#!/bin/bash
if [[ "$2" == list ]]; then
 if [[ "$REVIEW_SCENARIO" == "existing issue" ]]; then
  echo '[{"number":42,"title":"Review new supported runtime lines"}]'
 else
  echo '[]'
 fi
else
 printf '%s\n' "$@" > "$REVIEW_CAPTURE.args"
 cat > "$REVIEW_CAPTURE.body"
fi
`
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
			t.Setenv("REVIEW_SCENARIO", scenario)
			t.Setenv("REVIEW_CAPTURE", filepath.Join(dir, "capture"))
			t.Setenv("GITHUB_REPOSITORY", "libops/packages")
			t.Setenv("GITHUB_TOKEN", "secret")
			pages := 0
			old := http.DefaultTransport
			defer func() { http.DefaultTransport = old }()
			http.DefaultTransport = reviewTransport(func(r *http.Request) (*http.Response, error) {
				body := "[]"
				status := 200
				if r.URL.Host != "api.github.com" && r.Header.Get("Authorization") != "" {
					t.Fatal("sent GitHub token to external upstream")
				}
				switch r.URL.Host {
				case "go.dev":
					body = `[{"version":"go1.28.0","stable":true},{"version":"go1.29rc1","stable":false}]`
				case "raw.githubusercontent.com":
					body = "- version: 4.0.0\n"
				case "api.github.com":
					if r.Header.Get("Authorization") != "Bearer secret" {
						t.Fatal("missing authenticated GitHub request")
					}
					if strings.Contains(r.URL.Path, "php/php-src") {
						pages++
						if r.URL.Query().Get("page") == "1" {
							body = "[" + strings.TrimSuffix(strings.Repeat(`{"name":"php-8.4.1"},`, 100), ",") + "]"
						} else {
							body = `[{"name":"php-8.7.0"},{"name":"php-8.8.0RC1"}]`
						}
					}
				default:
					t.Fatal(r.URL)
				}
				if scenario == "upstream failure" {
					status = 503
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			err = Review(context.Background(), config.Snapshot(), true)
			if scenario == "upstream failure" {
				if err == nil {
					t.Fatal("ignored upstream failure")
				}
				if _, err := os.Stat(filepath.Join(dir, "capture.args")); !os.IsNotExist(err) {
					t.Fatal("mutated issue after fetch failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			args, err := os.ReadFile(filepath.Join(dir, "capture.args"))
			if err != nil {
				t.Fatal(err)
			}
			want := "create"
			if scenario == "existing issue" {
				want = "edit\n42"
			}
			if !strings.Contains(string(args), want) {
				t.Fatalf("wrong issue operation: %s", args)
			}
			body, err := os.ReadFile(filepath.Join(dir, "capture.body"))
			if err != nil {
				t.Fatal(err)
			}
			if pages != 2 || !strings.Contains(string(body), "php 8.7") || !strings.Contains(string(body), "go 1.28") || strings.Contains(string(body), "8.8") {
				t.Fatal(fmt.Sprintf("unexpected candidates/pages %d: %s", pages, body))
			}
		})
	}
}

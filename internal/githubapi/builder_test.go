package githubapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/libops/packages/internal/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBuilderPinPRLifecycle(t *testing.T) {
	for _, scenario := range []string{"new branch", "existing PR", "current pin missing PR", "write fails", "invalid existing content"} {
		t.Run(scenario, func(t *testing.T) {
			t.Chdir(t.TempDir())
			dir := "packages/alpine/3.24"
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			base := "alpine:3.24.1@sha256:" + strings.Repeat("a", 64)
			if err := os.WriteFile(filepath.Join(dir, "base-image.txt"), []byte(base), 0644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("REPOSITORY", "ghcr.io/libops")
			t.Setenv("GITHUB_REPOSITORY", "libops/packages")
			t.Setenv("GH_TOKEN", "secret")
			digest := "sha256:" + strings.Repeat("b", 64)
			if err := PinBuilder(context.Background(), config.Snapshot(), "alpine/3.24", digest, false); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(dir, "builder.json"))
			if err != nil {
				t.Fatal(err)
			}
			writes, creates, branches := 0, 0, 0
			old := http.DefaultTransport
			defer func() { http.DefaultTransport = old }()
			http.DefaultTransport = transport(func(r *http.Request) (*http.Response, error) {
				status := 200
				body := "{}"
				switch {
				case strings.Contains(r.URL.Path, "/git/ref/heads/"):
					body = `{"object":{"sha":"main-sha"}}`
					if scenario == "new branch" && !strings.HasSuffix(r.URL.Path, "/main") {
						status = 404
					}
				case strings.HasSuffix(r.URL.Path, "/git/refs"):
					branches++
				case strings.Contains(r.URL.Path, "/contents/"):
					if r.Method == "PUT" {
						writes++
						if scenario == "write fails" {
							status = 403
						}
					} else {
						current := "old"
						if scenario == "current pin missing PR" {
							current = string(content)
						}
						encoded := base64.StdEncoding.EncodeToString([]byte(current))
						if scenario == "invalid existing content" {
							encoded = "%notbase64"
						}
						body = fmt.Sprintf(`{"sha":"existing-sha","content":%q}`, encoded)
					}
				case strings.HasSuffix(r.URL.Path, "/pulls"):
					if r.Method == "POST" {
						creates++
						var payload map[string]string
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Fatal(err)
						}
						if payload["base"] != "main" {
							t.Fatal(payload)
						}
					} else {
						body = "[]"
						if scenario == "existing PR" {
							body = `[{"number":12}]`
						}
					}
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			err = PinBuilder(context.Background(), config.Snapshot(), "alpine/3.24", digest, true)
			bad := scenario == "write fails" || scenario == "invalid existing content"
			if (err != nil) != bad {
				t.Fatalf("error: %v", err)
			}
			if bad && creates != 0 {
				t.Fatal("created PR after API failure")
			}
			if !bad && scenario != "existing PR" && creates != 1 {
				t.Fatal("missing PR", creates)
			}
			if scenario == "existing PR" && creates != 0 {
				t.Fatal("duplicate PR")
			}
			if scenario == "current pin missing PR" && writes != 0 {
				t.Fatal("rewrote current pin")
			}
			if scenario == "new branch" && branches != 1 {
				t.Fatal("missing branch")
			}
		})
	}
}
func TestRejectUnsafeBuilderPin(t *testing.T) {
	for _, tc := range []struct{ target, digest string }{{"../main", "sha256:" + strings.Repeat("a", 64)}, {"alpine/3.24", "sha256:bad"}} {
		if PinBuilder(context.Background(), config.Snapshot(), tc.target, tc.digest, false) == nil {
			t.Fatal("accepted", tc)
		}
	}
}

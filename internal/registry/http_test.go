package registry

import (
	"context"
	"fmt"
	"github.com/libops/packages/internal/config"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCleanupHTTPNeverDeletesProtectedVersions(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		dry, late, malformed, missing bool
		wantDeletes                   int
		wantErr                       bool
	}{
		{name: "delete closed PR only", wantDeletes: 1},
		{name: "dry run", dry: true},
		{name: "late release tag", late: true, wantErr: true},
		{name: "malformed manifest", malformed: true, wantErr: true},
		{name: "publisher status missing", missing: true, wantErr: true},
		{name: "wrong package association", wantErr: true},
		{name: "already deleted"},
		{name: "fresh identity mismatch", wantErr: true},
		{name: "version pagination", wantDeletes: 1},
		{name: "versions unavailable", wantErr: true},
		{name: "null versions", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("packages.json", []byte(`{"php":{"target":"alpine/3.24/php/8.4"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			for k, v := range map[string]string{"PR": "12", "GITHUB_REPOSITORY": "libops/packages", "REPOSITORY": "ghcr.io/libops", "GH_TOKEN": "test-secret", "DRY_RUN": "0"} {
				t.Setenv(k, v)
			}
			if tc.dry {
				t.Setenv("DRY_RUN", "1")
			}
			deletes := 0
			old := http.DefaultTransport
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "ghcr.io" {
					if r.URL.Path == "/token" {
						if r.URL.Query().Get("scope") != "repository:libops/php:pull" {
							t.Fatalf("wrong registry scope: %s", r.URL)
						}
					} else if !strings.HasPrefix(r.URL.Path, "/v2/libops/php/manifests/") {
						t.Fatalf("unexpected image repository reached: %s", r.URL)
					}
				}
				if strings.Contains(r.URL.Path, "/packages/container/") && !strings.HasPrefix(r.URL.EscapedPath(), "/orgs/libops/packages/container/php") {
					t.Fatalf("unscoped package API path: %s", r.URL)
				}
				body := ""
				status := 200
				switch {
				case strings.HasSuffix(r.URL.Path, "/pulls/12"):
					body = `{"state":"closed","base":{"ref":"main"},"head":{"ref":"feature","repo":{"id":1}}}`
				case strings.Contains(r.URL.Path, "/actions/workflows/"):
					body = `{"workflow_runs":[]}`
					if tc.missing {
						status = 404
					}
				case r.URL.Path == "/users/libops":
					body = `{"type":"Organization"}`
				case r.URL.Path == "/orgs/libops/packages/container/php":
					body = `{"repository":{"full_name":"libops/packages"}}`
					if tc.name == "wrong package association" {
						body = `{"repository":{"full_name":"another/project"}}`
					}
				case strings.HasSuffix(r.URL.Path, "/versions"):
					body = fmt.Sprintf(`[{"id":1,"name":%q,"metadata":{"container":{"tags":["pr-12-build"]}}},{"id":2,"name":%q,"metadata":{"container":{"tags":["8.4.28-alpine3.24"]}}}]`, digest(1), digest(2))
					if tc.name == "version pagination" && r.URL.Query().Get("page") == "1" {
						items := []string{}
						for i := 3; i < 103; i++ {
							items = append(items, fmt.Sprintf(`{"id":%d,"name":%q,"metadata":{"container":{"tags":["release-%d"]}}}`, i, digest(i), i))
						}
						body = "[" + strings.Join(items, ",") + "]"
					}
					if tc.name == "versions unavailable" {
						status = 404
					}
					if tc.name == "null versions" {
						body = "null"
					}
				case r.URL.Path == "/token":
					body = `{"token":"registry-secret"}`
				case strings.Contains(r.URL.Path, "/manifests/"):
					body = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`
					if tc.malformed {
						body = `{}`
					}
				case strings.HasSuffix(r.URL.Path, "/versions/1"):
					if tc.name == "already deleted" {
						status = 404
						break
					}
					if r.Method == "DELETE" {
						deletes++
						status = 204
					} else {
						tag := "pr-12-build"
						if tc.late {
							tag = "release"
						}
						body = fmt.Sprintf(`{"id":1,"name":%q,"metadata":{"container":{"tags":[%q]}}}`, digest(1), tag)
						if tc.name == "fresh identity mismatch" {
							body = `{}`
						}
					}
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			defer func() { http.DefaultTransport = old }()
			err := Cleanup(context.Background(), config.Snapshot())
			if (err != nil) != tc.wantErr {
				t.Fatalf("error: %v", err)
			}
			if deletes != tc.wantDeletes {
				t.Fatalf("deleted %d, expected %d", deletes, tc.wantDeletes)
			}
		})
	}
}

func TestCleanupRejectsDifferentOwnerBeforeNetwork(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("unsafe request: %s", r.URL)
		return nil, fmt.Errorf("unexpected network")
	})
	defer func() { http.DefaultTransport = old }()
	for _, mode := range []config.Environment{{"PR": "12"}, {"CACHE_GC": "1"}, {"CLEANUP_RUN": "123"}} {
		mode["REPOSITORY"] = "ghcr.io/another-owner"
		mode["GITHUB_REPOSITORY"] = "libops/packages"
		if err := Cleanup(context.Background(), mode); err == nil {
			t.Fatal("accepted unrelated owner namespace")
		}
	}
}

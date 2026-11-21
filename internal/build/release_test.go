package build

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/libops/packages/internal/catalog"
	"github.com/libops/packages/internal/config"
)

func TestReleaseDigestStatus(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, status := range []int{200, 404, 401, 403, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(200)
					return
				}
				w.Header().Set("Docker-Content-Digest", digest)
				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				w.Header().Set("Content-Length", "123")
				w.WriteHeader(status)
			}))
			defer server.Close()
			got, err := ReleaseDigest(context.Background(), config.Environment{"HOME": t.TempDir()}, strings.TrimPrefix(server.URL, "http://")+"/php:8.4-r0")
			if status == 200 && (err != nil || got != digest) {
				t.Fatalf("%q %v", got, err)
			}
			if status == 404 && (err != nil || got != "") {
				t.Fatalf("%q %v", got, err)
			}
			if status != 200 && status != 404 && err == nil {
				t.Fatal("must fail closed")
			}
		})
	}
}
func TestPromotionAndAlias(t *testing.T) {
	if !promotion("", "new") || !promotion("same", "same") || promotion("old", "new") {
		t.Fatal("revision protection")
	}
	for _, tc := range []struct {
		revision string
		tags     []string
		want     bool
	}{
		{"0", []string{"8.4.28-r0-alpine3.24"}, true},
		{"1", []string{"8.4.28-r0-alpine3.24"}, true},
		{"0", []string{"8.4.28-r1-alpine3.24"}, false},
		{"2", []string{"8.4.28-r10-alpine3.24"}, false},
		{"0", []string{"8.4.29-r9-alpine3.24", "8.4.28-r9-alpine3.23", "candidate-other"}, true},
	} {
		if got := latestRevision("8.4.28", tc.revision, "8.4.28-r"+tc.revision+"-alpine3.24", tc.tags); got != tc.want {
			t.Fatalf("%+v: %t", tc, got)
		}
	}
}

func TestPublicationBootstrapAndFailures(t *testing.T) {
	for _, status := range []int{404, 403, 401, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					return
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			c, err := catalog.Load("../..")
			if err != nil {
				t.Fatal(err)
			}
			p, err := c.Metadata("../..", "alpine-3-24-nginx-1-30")
			if err != nil {
				t.Fatal(err)
			}
			cfg := config.Environment{"HOME": t.TempDir(), "RELEASE": "1"}
			repository := strings.TrimPrefix(server.URL, "http://") + "/libops"
			err = publicationPreflight(context.Background(), cfg, p, repository)
			if (err != nil) != (status == 500) {
				t.Fatalf("status %d: %v", status, err)
			}
			if status == 403 || status == 401 {
				// Deferring bootstrap lookup must never authorize final promotion.
				if _, err := ReleaseDigest(context.Background(), cfg, repository+"/nginx:"+p.RevisionTag); err == nil {
					t.Fatal("denied final promotion was accepted")
				}
			}
		})
	}
}

func TestReleaseDependenciesRejectDisposableCandidates(t *testing.T) {
	t.Chdir("../..")
	c, err := catalog.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Metadata(".", "alpine-3-24-nginx-1-30")
	if err != nil {
		t.Fatal(err)
	}
	parent := "alpine-3-24-php-8-4"
	p.Dependencies = []string{parent}
	digest := "sha256:" + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			return
		}
		w.Header().Set("Docker-Content-Digest", digest)
		w.Header().Set("Content-Type", "application/vnd.oci.image.index.v1+json")
		w.Header().Set("Content-Length", "123")
	}))
	defer server.Close()
	repository := strings.TrimPrefix(server.URL, "http://") + "/libops"
	cfg := config.Environment{"HOME": t.TempDir(), "RELEASE": "1"}
	for _, value := range []string{"", "docker-image://" + repository + "/php@" + digest} {
		cfg["CONTEXTS"] = parent + "=" + value
		if err := validateDependencies(context.Background(), cfg, c, p, repository); err != nil {
			t.Fatal(err)
		}
	}
	cfg["CONTEXTS"] = parent + "=docker-image://" + repository + "/php@sha256:" + strings.Repeat("b", 64)
	if err := validateDependencies(context.Background(), cfg, c, p, repository); err == nil {
		t.Fatal("accepted disposable parent digest")
	}
	cfg["PR"] = "1"
	if err := validateDependencies(context.Background(), cfg, c, p, repository); err != nil {
		t.Fatal("PR candidates cannot be released but may be tested together", err)
	}
	if !latestRevision(p.Version, p.Revision, p.RevisionTag, []string{p.RevisionTag}) {
		t.Fatal("catalog revision format rejected by alias logic")
	}
}

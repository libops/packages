package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/libops/packages/internal/config"
)

func cacheFixture() Manifest {
	m := Manifest{SchemaVersion: 2, MediaType: "application/vnd.oci.image.manifest.v1+json"}
	m.Config.MediaType = cacheMediaType
	return m
}
func TestCacheQuarantineAndProtection(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	old := now.Add(-cacheGrace - time.Hour)
	versions := []Version{
		version(1), // proven untagged cache, quarantine only
		version(2), // untagged application: never touch
		version(3, retiredCacheTag(digest(3), old)),
		version(4, retiredCacheTag(digest(4), old), "cache-main-target-arm64"),
		version(5, retiredCacheTag(digest(5), now.Add(-time.Hour))),
		version(6, "release"),
		version(7), // cache referenced by release
		version(8, retiredCacheTag(digest(8), old)), // forged marker on application
	}
	graph := map[string]Manifest{digest(1): cacheFixture(), digest(3): cacheFixture(), digest(4): cacheFixture(), digest(5): cacheFixture(), digest(7): cacheFixture(), digest(6): {Manifests: []Descriptor{{digest(7)}}}}
	read := func(d string) (Manifest, error) { return graph[d], nil }
	quarantine, remove, err := cachePlan(versions, now, read)
	if err != nil {
		t.Fatal(err)
	}
	if len(quarantine) != 1 || quarantine[0].ID != 1 || len(remove) != 1 || remove[0].ID != 3 {
		t.Fatalf("unsafe cache plan: quarantine=%+v remove=%+v", quarantine, remove)
	}
	if _, _, err := cachePlan(versions, now, func(string) (Manifest, error) { return Manifest{}, fmt.Errorf("unavailable") }); err == nil {
		t.Fatal("ignored unreadable graph")
	}
	for _, fresh := range []Version{version(3), version(3, "release"), version(3, retiredCacheTag(digest(3), now)), version(9, retiredCacheTag(digest(3), old))} {
		if err := validateRetiredCache(versions[2], fresh, now); err == nil {
			t.Fatal("allowed deletion after tags/identity changed")
		}
	}
	if expiredCacheTag(retiredCacheTag(digest(3), old), digest(4), now) || expiredCacheTag("cache-retired-invalid", digest(3), now) {
		t.Fatal("accepted ambiguous retirement marker")
	}
}
func TestRetiredCacheDeleteRechecksTags(t *testing.T) {
	for _, protect := range []bool{false, true} {
		t.Run(fmt.Sprint(protect), func(t *testing.T) {
			now := time.Now().UTC()
			v := version(1, retiredCacheTag(digest(1), now.Add(-cacheGrace-time.Hour)))
			deletes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "DELETE" {
					deletes++
					w.WriteHeader(204)
					return
				}
				fresh := v
				if protect {
					fresh.Metadata.Container.Tags = append(append([]string{}, fresh.Metadata.Container.Tags...), "8.4-r0-alpine3.24")
				}
				json.NewEncoder(w).Encode(fresh)
			}))
			defer server.Close()
			api := api{ctx: context.Background(), token: "fixture", client: server.Client()}
			read := func(string) (Manifest, error) { return cacheFixture(), nil }
			err := collectCaches(context.Background(), config.Environment{"DRY_RUN": "0"}, api, server.URL, "unused", "php", "unused", []Version{v}, read)
			if protect && (err == nil || deletes != 0) {
				t.Fatalf("deleted protected cache: %d %v", deletes, err)
			}
			if !protect && (err != nil || deletes != 1) {
				t.Fatalf("cache not collected: %d %v", deletes, err)
			}
		})
	}
}
func TestCacheDryRunDoesNotWrite(t *testing.T) {
	api := api{ctx: context.Background(), client: &http.Client{Transport: roundTripFail{t}}}
	read := func(string) (Manifest, error) { return cacheFixture(), nil }
	if err := collectCaches(context.Background(), config.Environment{}, api, "unused", "unused", "php", "unused", []Version{version(1)}, read); err != nil {
		t.Fatal(err)
	}
}

type roundTripFail struct{ t *testing.T }

func (r roundTripFail) RoundTrip(request *http.Request) (*http.Response, error) {
	r.t.Errorf("dry run requested %s %s", request.Method, request.URL)
	return nil, fmt.Errorf("unexpected network")
}
func TestCacheRejectsImageAndIndexTypes(t *testing.T) {
	for _, media := range []string{"application/vnd.oci.image.config.v1+json", "application/vnd.docker.container.image.v1+json", ""} {
		m := cacheFixture()
		m.Config.MediaType = media
		if cacheManifest(m) {
			t.Fatal("application classified as cache")
		}
	}
	m := cacheFixture()
	m.MediaType = "application/vnd.oci.image.index.v1+json"
	if cacheManifest(m) {
		t.Fatal("unknown index classified as cache")
	}
	if expiredCacheTag("cache-retired-1-"+strings.Repeat("b", 64), digest(1), time.Now()) {
		t.Fatal("wrong digest accepted")
	}
}

func TestQuarantineWritesOnlyVerifiedCacheBytes(t *testing.T) {
	for _, isCache := range []bool{true, false} {
		t.Run(fmt.Sprint(isCache), func(t *testing.T) {
			manifest := cacheFixture()
			if !isCache {
				manifest.Config.MediaType = "application/vnd.oci.image.config.v1+json"
			}
			payload, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(payload)
			digest := fmt.Sprintf("sha256:%x", sum)
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					return
				}
				if r.Method == "PUT" {
					writes++
					body, err := io.ReadAll(r.Body)
					if err != nil || !bytes.Equal(body, payload) {
						t.Error("changed cache manifest bytes")
					}
					w.Header().Set("Docker-Content-Digest", digest)
					w.WriteHeader(201)
					return
				}
				w.Header().Set("Docker-Content-Digest", digest)
				w.Header().Set("Content-Type", manifest.MediaType)
				w.Write(payload)
			}))
			defer server.Close()
			repo := strings.TrimPrefix(server.URL, "http://") + "/cache"
			err = quarantineCache(context.Background(), repo, digest, retiredCacheTag(digest, time.Now()), authn.Anonymous)
			if isCache && (err != nil || writes != 1) {
				t.Fatalf("quarantine failed: %d %v", writes, err)
			}
			if !isCache && (err == nil || writes != 0) {
				t.Fatalf("quarantined an application: %d %v", writes, err)
			}
		})
	}
}

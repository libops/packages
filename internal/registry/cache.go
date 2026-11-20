package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/libops/packages/internal/config"
)

const cacheMediaType = "application/vnd.buildkit.cacheconfig.v0"
const cacheGrace = 7 * 24 * time.Hour

func cacheManifest(m Manifest) bool {
	return m.SchemaVersion == 2 && m.MediaType == string(types.OCIManifestSchema1) && m.Config.MediaType == cacheMediaType && len(m.Manifests) == 0 && m.Subject == nil
}
func retiredCacheTag(digest string, now time.Time) string {
	return fmt.Sprintf("cache-retired-%d-%s", now.Unix(), strings.TrimPrefix(digest, "sha256:"))
}
func expiredCacheTag(tag, digest string, now time.Time) bool {
	if !strings.HasPrefix(tag, "cache-retired-") {
		return false
	}
	stamp, suffix, ok := strings.Cut(strings.TrimPrefix(tag, "cache-retired-"), "-")
	if !ok || suffix != strings.TrimPrefix(digest, "sha256:") {
		return false
	}
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	return err == nil && seconds > 0 && !time.Unix(seconds, 0).After(now.Add(-cacheGrace))
}

// Quarantine only proven, unreachable BuildKit cache manifests. Keeping a dated
// tag establishes a full grace period; GHCR created_at does not date tag removal.
// Image manifests, indexes and unknown untagged artifacts are never collected.
func cachePlan(versions []Version, now time.Time, read func(string) (Manifest, error)) ([]Version, []Version, error) {
	var quarantine []Version
	live := map[string]bool{}
	for _, v := range versions {
		if len(v.Metadata.Container.Tags) > 0 {
			live[v.Name] = true
		}
	}
	protected, err := reachable(live, read)
	if err != nil {
		return nil, nil, err
	}
	for _, v := range versions {
		if len(v.Metadata.Container.Tags) > 0 || protected[v.Name] {
			continue
		}
		m, err := read(v.Name)
		if err != nil {
			return nil, nil, err
		}
		if cacheManifest(m) {
			quarantine = append(quarantine, v)
		}
	}
	owns := func(tag string) bool {
		for _, v := range versions {
			if expiredCacheTag(tag, v.Name, now) {
				return true
			}
		}
		return false
	}
	expired, err := removable(versions, owns, read)
	if err != nil {
		return nil, nil, err
	}
	var remove []Version
	for _, v := range expired {
		m, err := read(v.Name)
		if err != nil {
			return nil, nil, err
		}
		if cacheManifest(m) {
			remove = append(remove, v)
		}
	}
	return quarantine, remove, nil
}
func collectCaches(ctx context.Context, cfg config.Environment, client api, endpoint, owner, image, token string, versions []Version, read func(string) (Manifest, error)) error {
	now := time.Now().UTC()
	quarantine, remove, err := cachePlan(versions, now, read)
	if err != nil {
		return err
	}
	dry := cfg.Get("DRY_RUN") != "0"
	for _, v := range quarantine {
		fmt.Printf("Quarantine cache %s@%s\n", image, v.Name)
		if dry {
			continue
		}
		var fresh Version
		status, err := client.request(fmt.Sprintf("%s/versions/%d", endpoint, v.ID), "GET", nil, &fresh)
		if err != nil {
			return err
		}
		if status == 404 {
			continue
		}
		if fresh.ID != v.ID || fresh.Name != v.Name || len(fresh.Metadata.Container.Tags) != 0 {
			return fmt.Errorf("cache gained tags or changed identity before quarantine")
		}
		repo := "ghcr.io/" + strings.ToLower(owner) + "/" + image
		if err := quarantineCache(ctx, repo, v.Name, retiredCacheTag(v.Name, now), &authn.Basic{Username: "x-access-token", Password: token}); err != nil {
			return err
		}
	}
	for _, v := range remove {
		fmt.Printf("Delete retired cache %s@%s\n", image, v.Name)
		if dry {
			continue
		}
		var fresh Version
		path := fmt.Sprintf("%s/versions/%d", endpoint, v.ID)
		status, err := client.request(path, "GET", nil, &fresh)
		if err != nil {
			return err
		}
		if status == 404 {
			continue
		}
		if err := validateRetiredCache(v, fresh, now); err != nil {
			return err
		}
		if _, err := client.request(path, "DELETE", nil, nil); err != nil {
			return err
		}
	}
	return nil
}
func validateRetiredCache(planned, fresh Version, now time.Time) error {
	if planned.ID != fresh.ID || planned.Name != fresh.Name || len(fresh.Metadata.Container.Tags) == 0 {
		return fmt.Errorf("retired cache identity/tags changed")
	}
	for _, tag := range fresh.Metadata.Container.Tags {
		if !expiredCacheTag(tag, fresh.Name, now) {
			return fmt.Errorf("retired cache acquired protected or recent tag")
		}
	}
	return nil
}

func quarantineCache(ctx context.Context, repository, digest, tagName string, credentials authn.Authenticator) error {
	ctx, stop := context.WithTimeout(ctx, time.Minute)
	defer stop()
	ref, err := name.ParseReference(repository+"@"+digest, name.StrictValidation)
	if err != nil {
		return err
	}
	options := []remote.Option{remote.WithContext(ctx), remote.WithAuth(credentials)}
	descriptor, err := remote.Get(ref, options...)
	if err != nil {
		return err
	}
	var actual Manifest
	if err := json.Unmarshal(descriptor.Manifest, &actual); err != nil {
		return err
	}
	if !cacheManifest(actual) || descriptor.Digest.String() != digest {
		return fmt.Errorf("cache manifest changed before quarantine")
	}
	tag, err := name.NewTag(repository+":"+tagName, name.StrictValidation)
	if err != nil {
		return err
	}
	return remote.Tag(tag, descriptor, options...)
}

package build

import (
	"context"
	"errors"
	"fmt"
	"github.com/libops/packages/internal/catalog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/libops/packages/internal/auth"
	"github.com/libops/packages/internal/config"
)

// ReleaseDigest distinguishes a missing manifest from denied access or outages.
// It never treats an authentication or transport failure as an absent release.
func ReleaseDigest(ctx context.Context, cfg config.Environment, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	parsed, err := name.ParseReference(ref, name.StrictValidation)
	if err != nil {
		return "", err
	}
	credentials, err := auth.Credentials(ctx, cfg, parsed.Context().RegistryStr())
	if err != nil {
		return "", err
	}
	descriptor, err := remote.Head(parsed, remote.WithContext(ctx), remote.WithAuth(authn.FromConfig(credentials)))
	if err != nil {
		var status *transport.Error
		if errors.As(err, &status) && status.StatusCode == http.StatusNotFound {
			return "", nil
		}
		return "", fmt.Errorf("inspect release %s: %w", ref, err)
	}
	return descriptor.Digest.String(), nil
}

// promotion preserves immutable revisions while permitting validation rebuilds.
// Re-running promotion of the identical manifest is successful.
func promotion(existing, candidate string) bool { return existing == "" || existing == candidate }

// Version aliases follow the highest published packaging revision. An older
// queued commit may validate, but cannot roll an alias back to an earlier rN.
func updateAlias(ctx context.Context, cfg config.Environment, p catalog.Metadata, repository, digest string) error {
	repo, err := name.NewRepository(repository+"/"+p.Image, name.StrictValidation)
	if err != nil {
		return err
	}
	credentials, err := auth.Credentials(ctx, cfg, repo.RegistryStr())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	tags, err := remote.List(repo, remote.WithContext(ctx), remote.WithAuth(authn.FromConfig(credentials)))
	if err != nil {
		return err
	}
	if !latestRevision(p.Version, p.Revision, p.RevisionTag, tags) {
		return nil
	}
	return run(ctx, nil, "docker", "buildx", "imagetools", "create", "-t", repository+"/"+p.Image+":"+p.Tag, repository+"/"+p.Image+"@"+digest)
}
func latestRevision(version, revision, revisionTag string, tags []string) bool {
	current, err := strconv.Atoi(strings.TrimPrefix(revision, version+"-r"))
	if err != nil {
		return false
	}
	prefix := version + "-r"
	suffix := strings.TrimPrefix(revisionTag, prefix+strconv.Itoa(current))
	for _, tag := range tags {
		if !strings.HasPrefix(tag, prefix) || !strings.HasSuffix(tag, suffix) {
			continue
		}
		value := strings.TrimSuffix(strings.TrimPrefix(tag, prefix), suffix)
		n, err := strconv.Atoi(value)
		if err == nil && n > current {
			return false
		}
	}
	return true
}

// A lookup denied before the first push is unresolved, not proof of absence.
// The final promotion check, after candidate publication, remains mandatory.
func publicationPreflight(ctx context.Context, cfg config.Environment, p catalog.Metadata, repository string) error {
	if !releasing(cfg) {
		return nil
	}
	existing, err := ReleaseDigest(ctx, cfg, repository+"/"+p.Image+":"+p.RevisionTag)
	var status *transport.Error
	if errors.As(err, &status) && (status.StatusCode == http.StatusUnauthorized || status.StatusCode == http.StatusForbidden) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing != "" {
		return fmt.Errorf("%s already exists: bump pkgrel before rebuilding", p.RevisionTag)
	}
	return nil
}
func validateDependencies(ctx context.Context, cfg config.Environment, c catalog.Catalog, p catalog.Metadata, repository string) error {
	if !releasing(cfg) {
		return nil
	}
	overrides := map[string]string{}
	for _, override := range strings.Fields(cfg.Get("CONTEXTS")) {
		target, value, ok := strings.Cut(override, "=")
		if !ok {
			return fmt.Errorf("invalid dependency context")
		}
		overrides[target] = value
	}
	for _, target := range p.Dependencies {
		parent, err := c.Metadata(".", target)
		if err != nil {
			return err
		}
		digest, err := ReleaseDigest(ctx, cfg, repository+"/"+parent.Image+":"+parent.RevisionTag)
		if err != nil {
			return err
		}
		value := overrides[target]
		if digest == "" || value != "" && value != "docker-image://"+repository+"/"+parent.Image+"@"+digest {
			return fmt.Errorf("release dependency %s must use its published revision digest", target)
		}
	}

	return nil
}

func releasing(cfg config.Environment) bool {
	return cfg.Get("VALIDATE_ONLY") != "1" && cfg.Get("PR") == "" && (cfg.Get("GITHUB_REF") == "refs/heads/main" || cfg.Get("RELEASE") == "1")
}

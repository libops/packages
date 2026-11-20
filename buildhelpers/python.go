package buildhelpers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"io"
	"os"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

type SourceIdentity struct {
	Issuer    string `json:"issuer"`
	Subject   string `json:"subject"`
	Reference string `json:"reference"`
}

type tufFetcher struct{ Updater }

func (f tufFetcher) DownloadFile(url string, maximum int64, _ time.Duration) ([]byte, error) {
	data, err := f.fetch(url)
	if err != nil {
		var status *HTTPStatusError
		if errors.As(err, &status) {
			return nil, &metadata.ErrDownloadHTTP{URL: status.URL, StatusCode: status.StatusCode}
		}
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("TUF metadata exceeds signed length limit")
	}
	return data, nil
}

func (u Updater) verifyPython(archive *os.File, materials []byte, identity *SourceIdentity) error {
	if identity == nil || identity.Issuer == "" || identity.Subject == "" {
		return fmt.Errorf("Python requires a reviewed release-manager Sigstore identity")
	}
	ctx := u.baseContext()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	u.Context = ctx
	options := tuf.DefaultOptions().WithContext(ctx)
	options.Fetcher = tufFetcher{u}
	trusted, err := root.FetchTrustedRootWithOptions(options)
	if err != nil {
		return err
	}
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	digest := sha256.New()
	if _, err = io.Copy(digest, archive); err != nil {
		return err
	}
	return verifyPythonBundle(materials, trusted, identity, digest.Sum(nil))
}
func verifyPythonBundle(materials []byte, trusted root.TrustedMaterial, identity *SourceIdentity, digest []byte) error {
	var entity bundle.Bundle
	if err := json.Unmarshal(materials, &entity); err != nil {
		return err
	}
	policy, err := verify.NewShortCertificateIdentity(identity.Issuer, "", identity.Subject, "")
	if err != nil {
		return err
	}
	verifier, err := verify.NewVerifier(trusted, verify.WithSignedCertificateTimestamps(1), verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1))
	if err != nil {
		return err
	}
	_, err = verifier.Verify(&entity, verify.NewPolicy(verify.WithArtifactDigest("sha256", digest), verify.WithCertificateIdentity(policy)))
	return err
}

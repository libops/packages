package buildhelpers

import (
	"encoding/hex"
	"os"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/root"
)

func TestPythonSigstoreAuthenticatesPublisherAndArtifact(t *testing.T) {
	materials, err := os.ReadFile("testdata/python-3.15.2.sigstore")
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := root.NewTrustedRootFromPath("testdata/sigstore-trusted-root.json")
	if err != nil {
		t.Fatal(err)
	}
	sum, _ := hex.DecodeString("8b5536d280142ecf6cdfe15d12a21ba1c65e300bbf7b28a8d0cdb849575797de")
	identity := SourceIdentity{Issuer: "https://github.com/login/oauth", Subject: "hugo@python.org"}
	if err := verifyPythonBundle(materials, trusted, &identity, sum); err != nil {
		t.Fatal(err)
	}
	wrong := identity
	wrong.Subject = "attacker@example.invalid"
	if verifyPythonBundle(materials, trusted, &wrong, sum) == nil {
		t.Fatal("accepted wrong publisher")
	}
	wrong = identity
	wrong.Issuer = "https://accounts.google.com"
	if verifyPythonBundle(materials, trusted, &wrong, sum) == nil {
		t.Fatal("accepted wrong issuer")
	}
	sum[0] ^= 0xff
	if verifyPythonBundle(materials, trusted, &identity, sum) == nil {
		t.Fatal("accepted altered archive")
	}
	if verifyPythonBundle([]byte("{}"), trusted, &identity, sum) == nil {
		t.Fatal("accepted empty verification bundle")
	}
	if (Updater{}).verifyPython(nil, nil, nil) == nil {
		t.Fatal("accepted missing reviewed identity")
	}
}

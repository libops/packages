package buildhelpers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func signTestArchive(t *testing.T, directory string, archive []byte) []byte {
	t.Helper()
	// macOS TMPDIR plus testing's long test names exceed Unix socket limits.
	base := ""
	if runtime.GOOS == "darwin" {
		base = "/tmp"
	}
	home, err := os.MkdirTemp(base, "pkg-gpg-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd, cancel := CommandContext(context.Background(), time.Minute, "gpgconf", "--homedir", home, "--kill", "gpg-agent")
		defer cancel()
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("stop test agent: %v %s", err, output)
		}
		if err := os.RemoveAll(home); err != nil {
			t.Error(err)
		}
	})
	run := func(args ...string) []byte {
		t.Helper()
		cmd, cancel := CommandContext(context.Background(), time.Minute, "gpg", append([]string{"--homedir", home, "--batch"}, args...)...)
		defer cancel()
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("gpg: %v %s", err, output)
		}
		return output
	}
	run("--pinentry-mode", "loopback", "--passphrase", "", "--quick-generate-key", "Package verification test <test@example.invalid>", "ed25519", "sign", "0")
	// Export only public material into the updater's expected pinned-key path.
	key := run("--armor", "--export")
	if err := os.WriteFile(filepath.Join(directory, "release-keys.asc"), key, 0644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(home, "source")
	if err := os.WriteFile(source, archive, 0600); err != nil {
		t.Fatal(err)
	}
	return run("--armor", "--detach-sign", "--output", "-", source)
}
func TestDetachedSignatureRejectsTampering(t *testing.T) {
	directory := t.TempDir()
	data := []byte("upstream source")
	signature := signTestArchive(t, directory, data)
	source := filepath.Join(directory, "source")
	os.WriteFile(source, data, 0600)
	if err := verifySignature(context.Background(), directory, source, signature); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(source, []byte("tampered"), 0600)
	if verifySignature(context.Background(), directory, source, signature) == nil {
		t.Fatal("accepted a tampered archive")
	}
	if verifySignature(context.Background(), directory, source, []byte("invalid")) == nil {
		t.Fatal("accepted an invalid signature")
	}
}

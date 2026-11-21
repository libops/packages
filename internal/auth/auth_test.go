package auth

import (
	"bufio"
	"context"
	"errors"
	"github.com/libops/packages/internal/config"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryHosts(t *testing.T) {
	for repo, want := range map[string]string{"ghcr.io/libops": "ghcr.io", "libops": "docker.io", "docker.io/libops": "docker.io", "localhost:5000/libops": "localhost:5000", "us-docker.pkg.dev/project/repo": "us-docker.pkg.dev"} {
		got, err := Host(repo)
		if err != nil || got != want {
			t.Fatalf("%s: %s %v", repo, got, err)
		}
	}
	for _, repo := range []string{"https://ghcr.io/libops", "user:password@ghcr.io/libops", "ghcr.io/libops;bad"} {
		if _, err := Host(repo); err == nil {
			t.Fatal(repo)
		}
	}
}
func TestLoginUsesStdinAndFailsClosedWithoutCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("CI", "true")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "")
	t.Setenv("REGISTRY_USER", "")
	t.Setenv("REGISTRY_PASSWORD", "")
	if err := Login(context.Background(), config.Snapshot(), "ghcr.io/libops"); err == nil {
		t.Fatal("missing CI credentials accepted")
	}
	script := "#!/bin/bash\nprintf '%s\\n' \"$@\" > \"$AUTH_CAPTURE.args\"\ncat > \"$AUTH_CAPTURE.stdin\"\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("AUTH_CAPTURE", filepath.Join(dir, "capture"))
	t.Setenv("REGISTRY_USER", "bot")
	t.Setenv("REGISTRY_PASSWORD", "secret-value")
	if err := Login(context.Background(), config.Snapshot(), "ghcr.io/libops"); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(filepath.Join(dir, "capture.args"))
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := os.ReadFile(filepath.Join(dir, "capture.stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "secret-value") || !strings.Contains(string(args), "--password-stdin") || string(stdin) != "secret-value\n" {
		t.Fatal("credentials did not use stdin")
	}
}

func TestPromptReadCanBeCancelled(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readLine(ctx, bufio.NewReader(reader)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRegistryCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, host, body, user, password, token string
		cfg                                     config.Environment
		bad                                     bool
	}{
		{name: "anonymous", host: "ghcr.io"},
		{name: "explicit", host: "ghcr.io", cfg: config.Environment{"REGISTRY_USER": "u", "REGISTRY_PASSWORD": "p"}, user: "u", password: "p"},
		{name: "github", host: "ghcr.io", cfg: config.Environment{"GITHUB_ACTOR": "bot", "GITHUB_TOKEN": "p"}, user: "bot", password: "p"},
		{name: "github token cannot escape", host: "example.org", cfg: config.Environment{"GITHUB_ACTOR": "bot", "GITHUB_TOKEN": "p"}},
		{name: "docker auth", host: "example.org", body: `{"auths":{"example.org":{"auth":"dTpw"}}}`, user: "u", password: "p"},
		{name: "docker hub alias", host: "index.docker.io", body: `{"auths":{"https://index.docker.io/v1/":{"auth":"dTpw"}}}`, user: "u", password: "p"},
		{name: "identity", host: "example.org", body: `{"auths":{"example.org":{"identitytoken":"token"}}}`, token: "token"},
		{name: "invalid JSON", host: "example.org", body: `{`, bad: true},
		{name: "invalid helper", host: "example.org", body: `{"credsStore":"../bad"}`, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.body != "" {
				if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(tc.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Environment{"DOCKER_CONFIG": dir}
			for key, value := range tc.cfg {
				cfg[key] = value
			}
			got, err := Credentials(context.Background(), cfg, tc.host)
			if (err != nil) != tc.bad {
				t.Fatal(err)
			}
			if !tc.bad && (got.Username != tc.user || got.Password != tc.password || got.IdentityToken != tc.token) {
				t.Fatal("incorrect credential selection")
			}
		})
	}
}
func TestCredentialHelper(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"credHelpers":{"example.org":"fixture"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "docker-credential-fixture")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nread host\ntest \"$host\" = example.org || exit 1\necho '{\"Username\":\"<token>\",\"Secret\":\"opaque\"}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Environment{"DOCKER_CONFIG": dir}
	got, err := Credentials(context.Background(), cfg, "example.org")
	if err != nil || got.IdentityToken != "opaque" {
		t.Fatalf("helper: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Credentials(ctx, cfg, "example.org"); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestCredentialHelperNotFoundFallsBack(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		inline, bad  bool
	}{
		{name: "anonymous", script: "echo 'credentials not found in native keychain'; exit 1"},
		{name: "inline", script: "echo 'credentials not found in native keychain'; exit 1", inline: true},
		{name: "helper broken", script: "echo 'keychain locked'; exit 1", bad: true},
		{name: "invalid output", script: "echo 'not JSON'", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
			body := `{"credHelpers":{"example.org":"fixture"}}`
			if tc.inline {
				body = `{"credHelpers":{"example.org":"fixture"},"auths":{"example.org":{"auth":"dTpw"}}}`
			}
			if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "docker-credential-fixture"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			got, err := Credentials(context.Background(), config.Environment{"DOCKER_CONFIG": dir}, "example.org")
			if (err != nil) != tc.bad {
				t.Fatalf("unexpected result: %v", err)
			}
			if tc.inline && (got.Username != "u" || got.Password != "p") {
				t.Fatal("inline fallback missing")
			}
			if !tc.inline && !tc.bad && (got.Username != "" || got.Password != "") {
				t.Fatal("anonymous fallback missing")
			}
		})
	}
}

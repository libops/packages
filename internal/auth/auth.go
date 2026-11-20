// Package auth uses supplied or existing registry credentials and secure local prompts.
package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	helpercredentials "github.com/docker/docker-credential-helpers/credentials"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/libops/packages/buildhelpers"
	"github.com/libops/packages/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func Host(repository string) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9.:-]*(?:/[a-z0-9][a-z0-9._-]*)*$`).MatchString(repository) {
		return "", fmt.Errorf("REPOSITORY must be a registry/namespace without scheme, auth or tag")
	}
	first := strings.Split(repository, "/")[0]
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return first, nil
	}
	return "docker.io", nil
}
func Prompt(ctx context.Context, cfg config.Environment, label string, secret bool, reader *bufio.Reader) (string, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if strings.EqualFold(cfg.Get("CI"), "true") || cfg.Get("CI") == "1" || info.Mode()&os.ModeCharDevice == 0 {
		return "", fmt.Errorf("%s: credentials missing; provide environment variables or a Docker credential helper", label)
	}
	if secret {
		quiet, cancel := buildhelpers.CommandContext(ctx, time.Minute, "stty", "-echo")
		defer cancel()
		quiet.Stdin = os.Stdin
		if err = quiet.Run(); err != nil {
			return "", fmt.Errorf("cannot hide terminal input; supply credentials through environment")
		}
		defer func() {
			restore, cancel := buildhelpers.CommandContext(context.WithoutCancel(ctx), 5*time.Second, "stty", "echo")
			defer cancel()
			restore.Stdin = os.Stdin
			restore.Run()
			fmt.Fprintln(os.Stderr)
		}()
	}
	fmt.Fprint(os.Stderr, label+": ")
	value, err := readLine(ctx, reader)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", label)
	}
	return value, nil
}
func Login(ctx context.Context, cfg config.Environment, repository string) error {
	host, err := Host(repository)
	if err != nil {
		return err
	}
	user, password := cfg.Get("REGISTRY_USER"), cfg.Get("REGISTRY_PASSWORD")
	if host == "ghcr.io" {
		if user == "" {
			user = cfg.Get("GITHUB_ACTOR")
		}
		if password == "" {
			password = cfg.Get("GITHUB_TOKEN")
		}
	}
	if user == "" || password == "" {
		credentials, err := Credentials(ctx, cfg, host)
		if err != nil {
			return err
		}
		if credentials.IdentityToken != "" || credentials.RegistryToken != "" || credentials.Auth != "" || credentials.Username != "" && credentials.Password != "" {
			return nil
		}
	}
	reader := bufio.NewReader(os.Stdin)
	if user == "" {
		user, err = Prompt(ctx, cfg, host+" REGISTRY_USER", false, reader)
		if err != nil {
			return err
		}
	}
	if password == "" {
		password, err = Prompt(ctx, cfg, host+" REGISTRY_PASSWORD", true, reader)
		if err != nil {
			return err
		}
	}
	cmd, cancel := buildhelpers.CommandContext(ctx, time.Minute, "docker", "login", host, "--username", user, "--password-stdin")
	defer cancel()
	cmd.Stdin = strings.NewReader(password + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Interactive reads must release the terminal-restoration defer on Ctrl-C.
func readLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	type answer struct {
		value string
		err   error
	}
	ready := make(chan answer, 1)
	go func() { value, err := reader.ReadString('\n'); ready <- answer{value, err} }()
	select {
	case result := <-ready:
		return result.value, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Credentials reads explicit credentials or Docker's configured credentials.
// Public planning may use anonymous access; helper execution remains bounded.
func Credentials(ctx context.Context, cfg config.Environment, host string) (authn.AuthConfig, error) {
	user, password := cfg.Get("REGISTRY_USER"), cfg.Get("REGISTRY_PASSWORD")
	if host == "ghcr.io" {
		if user == "" {
			user = cfg.Get("GITHUB_ACTOR")
		}
		if password == "" {
			password = cfg.Get("GITHUB_TOKEN")
		}
	}
	if user != "" && password != "" {
		return authn.AuthConfig{Username: user, Password: password}, nil
	}
	directory := cfg.Get("DOCKER_CONFIG")
	if directory == "" {
		directory = filepath.Join(cfg.Get("HOME"), ".docker")
	}
	data, err := os.ReadFile(filepath.Join(directory, "config.json"))
	if os.IsNotExist(err) {
		return authn.AuthConfig{}, nil
	}
	if err != nil {
		return authn.AuthConfig{}, err
	}
	var record struct {
		Auths   map[string]authn.AuthConfig `json:"auths"`
		Helpers map[string]string           `json:"credHelpers"`
		Store   string                      `json:"credsStore"`
	}
	if err = json.Unmarshal(data, &record); err != nil {
		return authn.AuthConfig{}, err
	}
	key := host
	if host == "index.docker.io" || host == "docker.io" {
		key = "https://index.docker.io/v1/"
	}
	helper := record.Helpers[key]
	if helper == "" {
		helper = record.Helpers[host]
	}
	if helper == "" {
		helper = record.Store
	}
	if helper != "" {
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(helper) {
			return authn.AuthConfig{}, fmt.Errorf("invalid credential helper")
		}
		cmd, stop := buildhelpers.CommandContext(ctx, time.Minute, "docker-credential-"+helper, "get")
		defer stop()
		cmd.Stdin = strings.NewReader(key + "\n")
		output, err := cmd.Output()
		if err != nil {
			if ctx.Err() == nil && helpercredentials.IsErrCredentialsNotFoundMessage(string(output)) {
				if value, ok := record.Auths[key]; ok {
					return value, nil
				}
				return record.Auths[host], nil
			}
			return authn.AuthConfig{}, fmt.Errorf("registry credential helper: %w", err)
		}
		var result struct{ Username, Secret string }
		if err = json.Unmarshal(output, &result); err != nil {
			return authn.AuthConfig{}, err
		}
		if result.Username == "<token>" {
			return authn.AuthConfig{IdentityToken: result.Secret}, nil
		}
		return authn.AuthConfig{Username: result.Username, Password: result.Secret}, nil
	}
	if value, ok := record.Auths[key]; ok {
		return value, nil
	}
	return record.Auths[host], nil
}

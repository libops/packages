package buildhelpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Update struct {
	Datasource string `json:"datasource"`
	Package    string `json:"package"`
	Allowed    string `json:"allowed"`
	Extract    string `json:"extract,omitempty"`
	Versioning string `json:"versioning,omitempty"`
	Tag        bool   `json:"tag,omitempty"`
}
type InventoryPackage struct {
	Package
	Test          string   `json:"test"`
	Dependencies  []string `json:"dependencies"`
	Architectures []string `json:"architectures"`
	Update        Update   `json:"update"`
}

var targetPattern = regexp.MustCompile(`^(alpine|debian|ubuntu)/[0-9]+(?:\.[0-9]+)?/[a-z][a-z0-9]*/[0-9]+(?:\.[0-9]+)*$`)

func TargetID(target string) (string, error) {
	if !targetPattern.MatchString(target) {
		return "", fmt.Errorf("invalid target %q", target)
	}
	return strings.NewReplacer("/", "-", ".", "-").Replace(target), nil
}
func ReadInventory(root string) (map[string]InventoryPackage, error) {
	data, err := os.ReadFile(filepath.Join(root, "packages.json"))
	if err != nil {
		return nil, err
	}
	var inventory map[string]InventoryPackage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&inventory); err != nil {
		return nil, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("inventory must contain one JSON object: %v", err)
	}
	for _, p := range inventory {
		if _, err := TargetID(p.Target); err != nil {
			return nil, err
		}
	}
	return inventory, nil
}
func ValidatePackage(p Package) error {
	if _, err := TargetID(p.Target); err != nil {
		return err
	}
	if strings.Split(p.Target, "/")[3] != p.Line {
		return fmt.Errorf("release line differs from target directory")
	}
	if !strings.HasPrefix(p.Upstream, "https://") {
		return fmt.Errorf("target needs an upstream support reference")
	}
	if _, err := time.Parse("2006-01-02", p.SupportedUntil); err != nil {
		return fmt.Errorf("%s needs a valid support deadline: %w", p.Target, err)
	}
	return nil
}

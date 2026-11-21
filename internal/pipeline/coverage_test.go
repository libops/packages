package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageFloors(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{"mode: set\ngithub.com/libops/packages/internal/buildapk/build.go:1.1,2.1 70 1\ngithub.com/libops/packages/internal/buildapk/build.go:3.1,4.1 30 0\ngithub.com/libops/packages/buildhelpers/update.go:1.1,2.1 70 1\ngithub.com/libops/packages/buildhelpers/update.go:3.1,4.1 30 0\n", true},
		{"mode: set\n", false},
		{"mode: count\n", false},
		{"mode: set\ngithub.com/libops/packages/internal/buildapk/build.go:1.1,2.1 100 0\ngithub.com/libops/packages/buildhelpers/update.go:1.1,2.1 100 1\n", false},
		{"mode: set\nmalformed\n", false},
	} {
		path := filepath.Join(t.TempDir(), "coverage.out")
		os.WriteFile(path, []byte(tc.body), 0600)
		if err := CheckCoverage(path); (err == nil) != tc.ok {
			t.Fatalf("%v: %s", err, tc.body)
		}
	}
}

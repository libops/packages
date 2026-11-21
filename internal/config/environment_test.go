package config

import "testing"

func TestSnapshotIsStableAndCopiesAreIndependent(t *testing.T) {
	t.Setenv("PACKAGES_CONFIG_TEST", "first")
	cfg := Snapshot()
	t.Setenv("PACKAGES_CONFIG_TEST", "second")
	if cfg.Get("PACKAGES_CONFIG_TEST") != "first" {
		t.Fatal("configuration changed after capture")
	}
	values := cfg.Copy()
	values["PACKAGES_CONFIG_TEST"] = "third"
	if cfg.Get("PACKAGES_CONFIG_TEST") != "first" {
		t.Fatal("consumer changed shared configuration")
	}
}

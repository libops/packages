package registry

import (
	"fmt"
	"testing"
)

func digest(n int) string { return fmt.Sprintf("sha256:%064x", n) }
func version(n int, tags ...string) Version {
	v := Version{ID: n, Name: digest(n)}
	v.Metadata.Container.Tags = tags
	return v
}
func TestProtectLiveAndSharedManifests(t *testing.T) {
	versions := []Version{version(1, "pr-12-target-sha"), version(2), version(3), version(4, "8.4.28-alpine3.24"), version(5, "pr-123-target-sha"), version(6, "pr-12-shared", "8.4.28-r0-alpine3.24"), version(7, "cache-pr-12-target-sha"), version(8)}
	graph := map[string]Manifest{digest(1): {Manifests: []Descriptor{{digest(2)}, {digest(3)}}}, digest(4): {Manifests: []Descriptor{{digest(3)}}}}
	read := func(d string) (Manifest, error) { return graph[d], nil }
	remove, err := Removable(versions, "12", read)
	if err != nil {
		t.Fatal(err)
	}
	found := map[int]bool{}
	for _, v := range remove {
		found[v.ID] = true
	}
	if len(found) != 2 || !found[1] || !found[7] {
		t.Fatalf("unsafe selection: %#v", found)
	}
	for _, id := range []int{2, 3, 4, 5, 6, 8} {
		if found[id] {
			t.Fatal("deleted protected/unknown version", id)
		}
	}
	if owned("pr-123-target", "12") {
		t.Fatal("matched a different PR number")
	}
	if _, err = Removable(versions, "12", func(string) (Manifest, error) { return Manifest{}, fmt.Errorf("registry unavailable") }); err == nil {
		t.Fatal("guessed reachability after read failure")
	}
	if remove, err = Removable(nil, "12", read); err != nil || len(remove) != 0 {
		t.Fatal("not idempotent")
	}
}

func TestCleanupRejectsAmbiguousInventory(t *testing.T) {
	for _, tc := range []struct {
		name, pr string
		versions []Version
	}{
		{"empty PR", "", []Version{version(1, "pr--x")}},
		{"prefix injection", "12-", []Version{version(1, "pr-12-x")}},
		{"zero ID", "12", []Version{version(0, "pr-12-x")}},
		{"duplicate ID", "12", []Version{version(1, "pr-12-x"), version(1, "release")}},
		{"invalid digest", "12", []Version{{ID: 1, Name: "unknown"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Removable(tc.versions, tc.pr, func(string) (Manifest, error) { t.Fatal("read before inventory validation"); return Manifest{}, nil }); err == nil {
				t.Fatal("accepted ambiguous inventory")
			}
		})
	}
}

func TestAttestationsAndCycles(t *testing.T) {
	versions := []Version{version(1, "pr-12-build"), version(2), version(3, "stable"), version(4), version(5, "cache-pr-12-build")}
	graph := map[string]Manifest{
		digest(1): {Manifests: []Descriptor{{digest(2)}, {digest(4)}}},
		digest(2): {Subject: &Descriptor{digest(1)}},
		digest(3): {Manifests: []Descriptor{{digest(4)}}},
		digest(4): {Subject: &Descriptor{digest(3)}},
	}
	got, err := Removable(versions, "12", func(d string) (Manifest, error) { return graph[d], nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range got {
		if v.ID == 3 || v.ID == 4 {
			t.Fatalf("deleted shared attestation/subject: %d", v.ID)
		}
	}
	if len(got) != 2 {
		t.Fatalf("unexpected candidates: %v", got)
	}
}

func TestMalformedChildFailsClosed(t *testing.T) {
	got, err := Removable([]Version{version(1, "pr-12-build")}, "12", func(string) (Manifest, error) { return Manifest{Manifests: []Descriptor{{"bad"}}}, nil })
	if err == nil || len(got) > 0 {
		t.Fatal("accepted malformed child")
	}
}

func TestFreshVersionProtectsLateTags(t *testing.T) {
	planned := version(1, "pr-12-build")
	for _, fresh := range []Version{version(1, "pr-12-build", "latest"), version(1, "pr-123-build"), version(2, "pr-12-build"), {}} {
		if validateFresh(planned, fresh, "12") == nil {
			t.Fatal("accepted changed or protected version", fresh)
		}
	}
	if err := validateFresh(planned, version(1, "pr-12-build", "cache-pr-12-build"), "12"); err != nil {
		t.Fatal(err)
	}
}

func TestCompletedRunCandidateIsolation(t *testing.T) {
	versions := []Version{
		version(1, "candidate-run-123-1-target-sha"),
		version(2, "candidate-run-123-2-target-sha"),
		version(3, "candidate-run-1234-1-target-sha"),
		version(4, "candidate-run-123-1-target-sha", "8.4-r0-alpine3.24"),
		version(5, "cache-main-target-arm64"),
		version(6),
		version(7, "candidate-run-123-1-other-sha"),
		version(8, "candidate-legacy-sha"),
	}
	graph := map[string]Manifest{digest(4): {Manifests: []Descriptor{{digest(7)}}}}
	got, err := Removable(versions, "run-123-1", func(d string) (Manifest, error) { return graph[d], nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("deleted protected or another run: %+v", got)
	}
	if err := validateFresh(versions[0], version(1, "candidate-run-123-1-target-sha", "release"), "run-123-1"); err == nil {
		t.Fatal("promotion race not protected")
	}
}

func TestCompletedPublisherIdentity(t *testing.T) {
	valid := completedRun{Status: "completed", Conclusion: "success", Event: "push", Path: ".github/workflows/push.yml", RunAttempt: 1, HeadBranch: "main"}
	valid.Repository.FullName = "libops/packages"
	if err := valid.validate("libops/packages"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*completedRun){
		func(r *completedRun) { r.Status = "in_progress" },
		func(r *completedRun) { r.Status = "queued" },
		func(r *completedRun) { r.Conclusion = "failure" },
		func(r *completedRun) { r.Conclusion = "cancelled" },
		func(r *completedRun) { r.Event = "pull_request" },
		func(r *completedRun) { r.Event = "pull_request_target" },
		func(r *completedRun) { r.Path = ".github/workflows/unrelated.yml" },
		func(r *completedRun) { r.HeadBranch = "feature" },
		func(r *completedRun) { r.Repository.FullName = "someone/packages" },
		func(r *completedRun) { r.RunAttempt = 0 },
	} {
		run := valid
		mutate(&run)
		if err := run.validate("libops/packages"); err == nil {
			t.Fatalf("accepted untrusted publisher: %+v", run)
		}
	}
	valid.Event = "workflow_dispatch"
	if err := valid.validate("libops/packages"); err != nil {
		t.Fatal(err)
	}
	planned := version(1, "candidate-run-123-1-target-sha")
	if err := validateFresh(planned, version(1), "run-123-1"); err == nil {
		t.Fatal("must not delete newly untagged version")
	}
}

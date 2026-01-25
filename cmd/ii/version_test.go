package main

import "testing"

func TestVersionString(t *testing.T) {
	prevChangeID := buildChangeID
	prevCommitID := buildCommitID
	t.Cleanup(func() {
		buildChangeID = prevChangeID
		buildCommitID = prevCommitID
	})

	buildChangeID = "change123"
	buildCommitID = "commit456"

	got := versionString()
	want := "change_id change123\ncommit_id commit456"
	if got != want {
		t.Fatalf("expected version string %q, got %q", want, got)
	}
}

func TestRootCommandHasVersion(t *testing.T) {
	if rootCmd.Version == "" {
		t.Fatal("expected root command version to be set")
	}
}

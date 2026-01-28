package job

import (
	"testing"

	"github.com/amonks/incrementum/internal/config"
)

func TestResolveOpencodeAgentForPurposePrefersOverride(t *testing.T) {
	cfg := &config.Config{Job: config.Job{Agent: "default", ImplementationModel: "impl"}}

	got := resolveOpencodeAgentForPurpose(cfg, "override", "implement")

	if got != "override" {
		t.Fatalf("expected override, got %q", got)
	}
}

func TestResolveOpencodeAgentForPurposeUsesPerTaskModel(t *testing.T) {
	cfg := &config.Config{Job: config.Job{
		Agent:               "default",
		ImplementationModel: "impl",
		CodeReviewModel:     "review",
		ProjectReviewModel:  "project",
	}}

	cases := []struct {
		name    string
		purpose string
		want    string
	}{
		{name: "implement", purpose: "implement", want: "impl"},
		{name: "review", purpose: "review", want: "review"},
		{name: "project-review", purpose: "project-review", want: "project"},
	}

	for _, tc := range cases {
		if got := resolveOpencodeAgentForPurpose(cfg, "", tc.purpose); got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestResolveOpencodeAgentForPurposeFallsBackToAgent(t *testing.T) {
	cfg := &config.Config{Job: config.Job{Agent: "default"}}

	got := resolveOpencodeAgentForPurpose(cfg, "", "review")

	if got != "default" {
		t.Fatalf("expected default, got %q", got)
	}
}

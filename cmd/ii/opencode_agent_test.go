package main

import (
	"testing"

	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

func TestResolveOpencodeAgentUsesFlagValue(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	if err := cmd.Flags().Set("agent", "flag-agent"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Setenv(opencode.AgentEnvVar, "env-agent")

	got := resolveOpencodeAgent(cmd, "flag-agent", "config-agent")

	if got != "flag-agent" {
		t.Fatalf("expected flag-agent, got %q", got)
	}
}

func TestResolveOpencodeAgentUsesEnvWhenFlagUnset(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	t.Setenv(opencode.AgentEnvVar, "env-agent")

	got := resolveOpencodeAgent(cmd, "", "config-agent")

	if got != "env-agent" {
		t.Fatalf("expected env-agent, got %q", got)
	}
}

func TestResolveOpencodeAgentHonorsEmptyFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	if err := cmd.Flags().Set("agent", ""); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Setenv(opencode.AgentEnvVar, "env-agent")

	got := resolveOpencodeAgent(cmd, "", "config-agent")

	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestResolveOpencodeAgentUsesConfigWhenUnset(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")

	got := resolveOpencodeAgent(cmd, "", "config-agent")

	if got != "config-agent" {
		t.Fatalf("expected config-agent, got %q", got)
	}
}

func TestResolveOpencodeAgentDefaultsEmpty(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")

	got := resolveOpencodeAgent(cmd, "", "")

	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestResolveOpencodeAgentOverrideUsesFlagValue(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	if err := cmd.Flags().Set("agent", "flag-agent"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Setenv(opencode.AgentEnvVar, "env-agent")

	got := resolveOpencodeAgentOverride(cmd, "flag-agent")

	if got != "flag-agent" {
		t.Fatalf("expected flag-agent, got %q", got)
	}
}

func TestResolveOpencodeAgentOverrideUsesEnvWhenFlagUnset(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	t.Setenv(opencode.AgentEnvVar, "env-agent")

	got := resolveOpencodeAgentOverride(cmd, "")

	if got != "env-agent" {
		t.Fatalf("expected env-agent, got %q", got)
	}
}

func TestResolveOpencodeAgentOverrideHonorsEmptyFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")
	if err := cmd.Flags().Set("agent", ""); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Setenv(opencode.AgentEnvVar, "env-agent")

	got := resolveOpencodeAgentOverride(cmd, "")

	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestResolveOpencodeAgentOverrideDefaultsEmpty(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("agent", "", "")

	got := resolveOpencodeAgentOverride(cmd, "")

	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

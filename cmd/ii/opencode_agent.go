package main

import (
	"os"
	"strings"

	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

func resolveOpencodeAgent(cmd *cobra.Command, flagValue string) string {
	if cmd != nil && cmd.Flags().Changed("agent") {
		return flagValue
	}

	envValue := strings.TrimSpace(os.Getenv(opencode.AgentEnvVar))
	if envValue != "" {
		return envValue
	}
	return ""
}

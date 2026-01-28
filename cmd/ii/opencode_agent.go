package main

import (
	"os"
	"strings"

	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

func resolveOpencodeAgent(cmd *cobra.Command, flagValue, configAgent string) string {
	if cmd != nil && cmd.Flags().Changed("agent") {
		return flagValue
	}

	return firstTrimmed(os.Getenv(opencode.AgentEnvVar), configAgent)
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

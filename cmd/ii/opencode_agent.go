package main

import (
	"os"

	internalstrings "github.com/amonks/incrementum/internal/strings"
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
		trimmed := internalstrings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

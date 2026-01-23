package main

import "github.com/spf13/cobra"

func hasChangedFlags(cmd *cobra.Command, flags ...string) bool {
	for _, flag := range flags {
		if cmd.Flags().Changed(flag) {
			return true
		}
	}
	return false
}

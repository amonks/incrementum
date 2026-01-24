package main

import (
	"os"
	"time"

	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

var opencodeTailCmd = &cobra.Command{
	Use:   "tail <session-id>",
	Short: "Stream opencode session logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpencodeTail,
}

func init() {
	opencodeCmd.AddCommand(opencodeTailCmd)
}

func runOpencodeTail(cmd *cobra.Command, args []string) error {
	store, err := opencode.Open()
	if err != nil {
		return err
	}

	repoPath, err := getOpencodeRepoPath()
	if err != nil {
		return err
	}

	return store.Tail(cmd.Context(), repoPath, args[0], os.Stdout, time.Second)
}

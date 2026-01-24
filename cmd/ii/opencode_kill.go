package main

import (
	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

var opencodeKillCmd = &cobra.Command{
	Use:   "kill <session-id>",
	Short: "Terminate an opencode session",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpencodeKill,
}

func init() {
	opencodeCmd.AddCommand(opencodeKillCmd)
}

func runOpencodeKill(cmd *cobra.Command, args []string) error {
	store, err := opencode.Open()
	if err != nil {
		return err
	}

	repoPath, err := opencode.RepoPathForWorkingDir()
	if err != nil {
		return err
	}

	session, err := store.Kill(repoPath, args[0])
	if err != nil {
		return err
	}

	return exitFromOpencodeSession(session)
}

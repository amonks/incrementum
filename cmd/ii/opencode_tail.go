package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	internalopencode "github.com/amonks/incrementum/internal/opencode"
	"github.com/amonks/incrementum/workspace"
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
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getOpencodeRepoPath()
	if err != nil {
		return err
	}

	session, err := pool.FindOpencodeSession(repoPath, args[0])
	if err != nil {
		return err
	}

	storage, err := opencodeStorage()
	if err != nil {
		return err
	}

	return opencodeLogTail(cmd.Context(), storage, session.ID, os.Stdout, time.Second)
}

func opencodeLogTail(ctx context.Context, storage internalopencode.Storage, sessionID string, writer io.Writer, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}

	seen := make(map[string]struct{})
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		entries, err := storage.SessionLogEntries(sessionID)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if _, ok := seen[entry.ID]; ok {
				continue
			}
			if _, writeErr := writer.Write([]byte(entry.Text)); writeErr != nil {
				return fmt.Errorf("write opencode log: %w", writeErr)
			}
			seen[entry.ID] = struct{}{}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

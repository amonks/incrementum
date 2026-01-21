package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	logPath, err := opencodeSessionLogPath(pool, repoPath, args[0])
	if err != nil {
		return err
	}

	return opencodeLogTail(cmd.Context(), logPath, os.Stdout, time.Second)
}

func opencodeLogTail(ctx context.Context, logPath string, writer io.Writer, interval time.Duration) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open opencode log: %w", err)
	}
	defer file.Close()

	if interval <= 0 {
		interval = time.Second
	}

	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		count, err := file.Read(buffer)
		if count > 0 {
			if _, writeErr := writer.Write(buffer[:count]); writeErr != nil {
				return fmt.Errorf("write opencode log: %w", writeErr)
			}
		}

		if err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("read opencode log: %w", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(interval):
			}
		}
	}
}

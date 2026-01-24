package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

var opencodeRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Start a new opencode session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOpencodeRun,
}

func init() {
	opencodeCmd.AddCommand(opencodeRunCmd)
}

func runOpencodeRun(cmd *cobra.Command, args []string) error {
	store, err := opencode.Open()
	if err != nil {
		return err
	}

	repoPath, err := opencode.RepoPathForWorkingDir()
	if err != nil {
		return err
	}

	prompt, err := resolveOpencodePrompt(args, os.Stdin)
	if err != nil {
		return err
	}

	handle, err := store.Run(opencode.RunOptions{
		RepoPath:  repoPath,
		WorkDir:   repoPath,
		Prompt:    prompt,
		StartedAt: time.Now(),
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	})
	if err != nil {
		return err
	}

	drainDone := opencode.DrainEvents(handle.Events)
	result, err := handle.Wait()
	<-drainDone
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return exitError{code: result.ExitCode}
	}
	return nil
}

func resolveOpencodePrompt(args []string, reader io.Reader) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read prompt from stdin: %w", err)
	}

	prompt := strings.TrimSuffix(string(data), "\n")
	prompt = strings.TrimSuffix(prompt, "\r")
	return prompt, nil
}

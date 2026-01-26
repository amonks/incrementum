package main

import (
	"fmt"
	"io"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

func openTodoStoreWithOptions(cmd *cobra.Command, args []string, opts todo.OpenOptions) (*todo.Store, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, err
	}

	opts.Purpose = todoStorePurpose(cmd, args)
	return todo.Open(repoPath, opts)
}

func openTodoStore(cmd *cobra.Command, args []string) (*todo.Store, error) {
	return openTodoStoreWithOptions(cmd, args, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
}

func openTodoStoreReadOnly(cmd *cobra.Command, args []string) (*todo.Store, error) {
	return openTodoStoreWithOptions(cmd, args, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		ReadOnly:        true,
	})
}

func todoStorePurpose(cmd *cobra.Command, args []string) string {
	parts := []string{cmd.CommandPath()}
	parts = append(parts, args...)
	value := strings.Join(parts, " ")
	value = internalstrings.NormalizeWhitespace(value)
	if value == "" {
		return "todo store"
	}
	return fmt.Sprintf("todo store (%s)", value)
}

func resolveDescriptionFromStdin(description string, reader io.Reader) (string, error) {
	if description != "-" {
		return description, nil
	}

	input, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read description from stdin: %w", err)
	}

	value := strings.TrimRight(string(input), "\r\n")
	return value, nil
}

package job

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunTestCommands executes test commands sequentially in a directory.
func RunTestCommands(dir string, commands []string) ([]TestCommandResult, error) {
	results := make([]TestCommandResult, 0, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			return results, fmt.Errorf("test command is required")
		}

		cmd := exec.Command("/bin/bash", "-lc", command)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		exitCode := 0
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return results, fmt.Errorf("run test command %q: %w", command, err)
			}
			exitCode = exitErr.ExitCode()
		}

		results = append(results, TestCommandResult{
			Command:  command,
			ExitCode: exitCode,
		})
	}

	return results, nil
}

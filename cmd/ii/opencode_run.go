package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var opencodeRunCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Start a new opencode session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOpencodeRun,
}

var opencodeRunAttach bool

func init() {
	opencodeCmd.AddCommand(opencodeRunCmd)

	opencodeRunCmd.Flags().BoolVar(&opencodeRunAttach, "attach", true, "Attach to the opencode session")
}

func runOpencodeRun(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	daemon, err := pool.FindOpencodeDaemon(repoPath)
	if err != nil {
		return err
	}
	if daemon.Status != workspace.OpencodeDaemonRunning {
		return fmt.Errorf("opencode daemon is not running")
	}

	prompt, err := resolveOpencodePrompt(args, os.Stdin)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	sessionID, logPath, err := opencodeRunLogPath(pool, repoPath, prompt, startedAt)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("create opencode log dir: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open opencode log: %w", err)
	}
	defer logFile.Close()

	attachURL := workspace.DaemonAttachURL(daemon)
	runArgs := []string{"run", "--attach", attachURL, prompt}

	runCmd := exec.Command("opencode", runArgs...)
	runCmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	runCmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	runCmd.Stdin = os.Stdin

	if err := runCmd.Start(); err != nil {
		return fmt.Errorf("start opencode run: %w", err)
	}

	session, err := pool.CreateOpencodeSession(repoPath, prompt, logPath, startedAt)
	if err != nil {
		_ = runCmd.Process.Kill()
		_ = runCmd.Wait()
		return err
	}
	if session.ID != sessionID {
		return fmt.Errorf("opencode session id mismatch")
	}

	fmt.Println(session.ID)
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

func opencodeRunLogPath(pool *workspace.Pool, repoPath, prompt string, startedAt time.Time) (string, string, error) {
	logDir, err := opencodeLogDir(pool, repoPath)
	if err != nil {
		return "", "", err
	}

	sessionID := workspace.GenerateOpencodeSessionID(prompt, startedAt)
	return sessionID, filepath.Join(logDir, sessionID+".log"), nil
}

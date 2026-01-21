package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var opencodeServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the opencode server",
	Args:  cobra.NoArgs,
	RunE:  runOpencodeServe,
}

var opencodeServeHost string
var opencodeServePort int

func init() {
	opencodeCmd.AddCommand(opencodeServeCmd)

	opencodeServeCmd.Flags().StringVar(&opencodeServeHost, "host", "", "Host for the opencode server")
	opencodeServeCmd.Flags().IntVar(&opencodeServePort, "port", 0, "Port for the opencode server")
}

func runOpencodeServe(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	logPath, err := opencodeDaemonLogPath(pool, repoPath)
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

	serveArgs := []string{"serve"}
	if opencodeServeHost != "" {
		serveArgs = append(serveArgs, "--host", opencodeServeHost)
	}
	if opencodeServePort != 0 {
		serveArgs = append(serveArgs, "--port", strconv.Itoa(opencodeServePort))
	}

	serveCmd := exec.Command("opencode", serveArgs...)
	serveCmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	serveCmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	serveCmd.Stdin = os.Stdin

	startedAt := time.Now()
	if err := serveCmd.Start(); err != nil {
		return fmt.Errorf("start opencode serve: %w", err)
	}

	if _, err := pool.RecordOpencodeDaemon(repoPath, serveCmd.Process.Pid, opencodeServeHost, opencodeServePort, logPath, startedAt); err != nil {
		_ = serveCmd.Process.Kill()
		_ = serveCmd.Wait()
		return err
	}

	waitErr := serveCmd.Wait()
	stoppedAt := time.Now()
	_, stopErr := pool.StopOpencodeDaemon(repoPath, stoppedAt)
	if stopErr != nil {
		if waitErr != nil {
			return errors.Join(waitErr, stopErr)
		}
		return stopErr
	}
	if waitErr != nil {
		return fmt.Errorf("opencode serve: %w", waitErr)
	}
	return nil
}

func opencodeDaemonLogPath(pool *workspace.Pool, repoPath string) (string, error) {
	logDir, err := opencodeLogDir(pool, repoPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(logDir, "daemon.log"), nil
}

func opencodeLogDir(pool *workspace.Pool, repoPath string) (string, error) {
	repoSlug, err := pool.RepoSlug(repoPath)
	if err != nil {
		return "", err
	}

	baseDir, err := opencodeBaseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, repoSlug), nil
}

func opencodeBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".local", "share", "incrementum", "opencode"), nil
}

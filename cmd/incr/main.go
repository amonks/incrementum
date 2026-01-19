// Package main implements the incr CLI tool.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "incr",
	Short: "Incrementum - tools for incremental development",
}

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage a pool of jujutsu workspaces",
}

var acquireCmd = &cobra.Command{
	Use:   "acquire",
	Short: "Acquire an available workspace or create a new one",
	RunE:  runAcquire,
}

var releaseCmd = &cobra.Command{
	Use:   "release [path]",
	Short: "Release an acquired workspace back to the pool",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRelease,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces for the current repo",
	RunE:  runList,
}

var renewCmd = &cobra.Command{
	Use:   "renew [path]",
	Short: "Extend the TTL for an acquired workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRenew,
}

var (
	acquireRev string
	acquireTTL time.Duration
	listJSON   bool
)

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(acquireCmd, releaseCmd, listCmd, renewCmd)

	acquireCmd.Flags().StringVar(&acquireRev, "rev", "@", "Revision to check out")
	acquireCmd.Flags().DurationVar(&acquireTTL, "ttl", workspace.DefaultTTL, "Lease duration before auto-expiry")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
}

func runAcquire(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{
		Rev: acquireRev,
		TTL: acquireTTL,
	})
	if err != nil {
		return fmt.Errorf("acquire workspace: %w", err)
	}

	fmt.Println(wsPath)
	return nil
}

func runRelease(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	wsPath, err := resolvePath(args)
	if err != nil {
		return err
	}

	return pool.Release(wsPath)
}

func runList(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	items, err := pool.List(repoPath)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	if listJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	if len(items) == 0 {
		fmt.Println("No workspaces found for this repository.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tCHANGE\tTTL\tPATH")
	for _, item := range items {
		ttl := "-"
		if item.Status == workspace.StatusAcquired && item.TTLRemaining > 0 {
			ttl = item.TTLRemaining.Truncate(time.Second).String()
		}
		changeID := item.CurrentChangeID
		if len(changeID) > 12 {
			changeID = changeID[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.Name, item.Status, changeID, ttl, item.Path)
	}
	return w.Flush()
}

func runRenew(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	wsPath, err := resolvePath(args)
	if err != nil {
		return err
	}

	return pool.Renew(wsPath)
}

// getRepoPath returns the jj repository root for the current directory.
func getRepoPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	root, err := workspace.RepoRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("not in a jj repository: %w", err)
	}

	return root, nil
}

// resolvePath returns the workspace path from args or current directory.
func resolvePath(args []string) (string, error) {
	if len(args) > 0 {
		path := args[0]
		if !filepath.IsAbs(path) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("get working directory: %w", err)
			}
			path = filepath.Join(cwd, path)
		}
		return path, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return cwd, nil
}

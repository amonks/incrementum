package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage a pool of jujutsu workspaces",
}

var workspaceAcquireCmd = &cobra.Command{
	Use:   "acquire",
	Short: "Acquire an available workspace or create a new one",
	RunE:  runWorkspaceAcquire,
}

var workspaceReleaseCmd = &cobra.Command{
	Use:   "release [name]",
	Short: "Release an acquired workspace back to the pool",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceRelease,
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces for the current repo",
	RunE:  runWorkspaceList,
}

var workspaceRenewCmd = &cobra.Command{
	Use:   "renew [name]",
	Short: "Extend the TTL for an acquired workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWorkspaceRenew,
}

var workspaceDestroyAllCmd = &cobra.Command{
	Use:   "destroy-all",
	Short: "Destroy all workspaces for the current repository",
	RunE:  runWorkspaceDestroyAll,
}

var (
	workspaceAcquireRev string
	workspaceAcquireTTL time.Duration
	workspaceListJSON   bool
)

func init() {
	rootCmd.AddCommand(workspaceCmd)
	workspaceCmd.AddCommand(workspaceAcquireCmd, workspaceReleaseCmd, workspaceListCmd, workspaceRenewCmd, workspaceDestroyAllCmd)

	workspaceAcquireCmd.Flags().StringVar(&workspaceAcquireRev, "rev", "@", "Revision to check out")
	workspaceAcquireCmd.Flags().DurationVar(&workspaceAcquireTTL, "ttl", workspace.DefaultTTL, "Lease duration before auto-expiry")
	workspaceListCmd.Flags().BoolVar(&workspaceListJSON, "json", false, "Output as JSON")
}

func runWorkspaceAcquire(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	wsPath, err := pool.Acquire(repoPath, workspace.AcquireOptions{
		Rev: workspaceAcquireRev,
		TTL: workspaceAcquireTTL,
	})
	if err != nil {
		return fmt.Errorf("acquire workspace: %w", err)
	}

	fmt.Println(wsPath)
	return nil
}

func runWorkspaceRelease(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	wsName, err := resolveWorkspaceName(args, pool)
	if err != nil {
		return err
	}

	return pool.ReleaseByName(repoPath, wsName)
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
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

	if workspaceListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	if len(items) == 0 {
		fmt.Println("No workspaces found for this repository.")
		return nil
	}

	fmt.Print(formatWorkspaceTable(items, nil))
	return nil
}

func runWorkspaceRenew(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	wsName, err := resolveWorkspaceName(args, pool)
	if err != nil {
		return err
	}

	return pool.RenewByName(repoPath, wsName)
}

func runWorkspaceDestroyAll(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	return pool.DestroyAll(repoPath)
}

func formatWorkspaceTable(items []workspace.Info, highlight func(string) string) string {
	if highlight == nil {
		highlight = func(value string) string { return value }
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		ttl := "-"
		if item.Status == workspace.StatusAcquired && item.TTLRemaining > 0 {
			ttl = item.TTLRemaining.Truncate(time.Second).String()
		}
		rows = append(rows, []string{
			highlight(item.Name),
			string(item.Status),
			ttl,
			truncateTableCell(item.Path),
		})
	}

	return formatTable([]string{"NAME", "STATUS", "TTL", "PATH"}, rows)
}

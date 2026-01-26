package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/swarmtui"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/swarm"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

var swarmCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Manage parallel job runs",
}

var swarmServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the swarm server",
	RunE:  runSwarmServe,
}

var swarmDoCmd = &cobra.Command{
	Use:   "do [todo-id]",
	Short: "Start a swarm job for a new or existing todo",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSwarmDo,
}

var swarmKillCmd = &cobra.Command{
	Use:   "kill <job-id>",
	Short: "Stop a running job",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwarmKill,
}

var swarmTailCmd = &cobra.Command{
	Use:   "tail <job-id>",
	Short: "Stream job logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwarmTail,
}

var swarmLogsCmd = &cobra.Command{
	Use:   "logs <job-id>",
	Short: "Show job logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwarmLogs,
}

var swarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List swarm jobs",
	RunE:  runSwarmList,
}

var swarmClientCmd = &cobra.Command{
	Use:   "client",
	Short: "Launch the swarm TUI client",
	RunE:  runSwarmClient,
}

var (
	swarmAddr       string
	swarmPath       string
	swarmListAll    bool
	swarmListStatus string
	swarmListJSON   bool
	swarmAgent      string
)

func init() {
	rootCmd.AddCommand(swarmCmd)
	swarmCmd.AddCommand(swarmServeCmd, swarmDoCmd, swarmKillCmd, swarmTailCmd, swarmLogsCmd, swarmListCmd, swarmClientCmd)
	addDescriptionFlagAliases(swarmDoCmd)

	swarmServeCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
	swarmServeCmd.Flags().StringVar(&swarmAgent, "agent", "", "Opencode agent")

	swarmDoCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
	swarmDoCmd.Flags().StringVar(&swarmPath, "path", "", "Repository path")
	swarmDoCmd.Flags().StringVar(&jobDoTitle, "title", "", "Todo title")
	swarmDoCmd.Flags().StringVarP(&jobDoType, "type", "t", "task", "Todo type (task, bug, feature)")
	swarmDoCmd.Flags().IntVarP(&jobDoPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	swarmDoCmd.Flags().StringVarP(&jobDoDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	swarmDoCmd.Flags().StringArrayVar(&jobDoDeps, "deps", nil, "Dependencies in format <id> (e.g., abc123)")
	swarmDoCmd.Flags().BoolVarP(&jobDoEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	swarmDoCmd.Flags().BoolVar(&jobDoNoEdit, "no-edit", false, "Do not open $EDITOR")

	swarmKillCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
	swarmTailCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
	swarmLogsCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
	swarmListCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
	swarmListCmd.Flags().StringVar(&swarmListStatus, "status", "", "Filter by status")
	swarmListCmd.Flags().BoolVar(&swarmListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(swarmListCmd, &swarmListAll)

	swarmClientCmd.Flags().StringVar(&swarmAddr, "addr", "", "Swarm server address")
}

func runSwarmServe(cmd *cobra.Command, _ []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(repoPath, swarmAddr)
	if err != nil {
		return err
	}
	if err := logSwarmServeAddr(addr); err != nil {
		return err
	}
	opencodeAgent := resolveOpencodeAgent(cmd, swarmAgent)
	server, err := swarm.NewServer(swarm.ServerOptions{
		RepoPath:      repoPath,
		JobRunOptions: job.RunOptions{OpencodeAgent: opencodeAgent},
	})
	if err != nil {
		return err
	}
	return server.Serve(addr)
}

func logSwarmServeAddr(addr string) error {
	normalized, err := normalizeSwarmServeAddr(addr)
	if err != nil {
		return err
	}
	fmt.Printf("Swarm server listening on %s\n", normalized)
	return nil
}

func normalizeSwarmServeAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("parse swarm addr: %w", err)
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, port), nil
}

func runSwarmDo(cmd *cobra.Command, args []string) error {
	if err := resolveDescriptionFlag(cmd, &jobDoDescription, os.Stdin); err != nil {
		return err
	}

	hasCreateFlags := hasTodoCreateFlags(cmd)
	if len(args) > 0 && (hasCreateFlags || jobDoEdit || jobDoNoEdit) {
		return fmt.Errorf("todo id cannot be combined with todo creation flags")
	}
	path, err := resolveRepoPath(swarmPath)
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(path, swarmAddr)
	if err != nil {
		return err
	}
	client := swarm.NewClient(addr)

	todoID := ""
	if len(args) > 0 {
		todoID = args[0]
	} else {
		createdID, err := createTodoForSwarm(cmd, path, hasCreateFlags)
		if err != nil {
			return err
		}
		todoID = createdID
	}
	jobID, err := client.Do(cmd.Context(), todoID)
	if err != nil {
		return err
	}
	fmt.Printf("Doing job %s\n", jobID)
	return streamSwarmEvents(cmd.Context(), client, jobID)
}

func runSwarmKill(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(repoPath, swarmAddr)
	if err != nil {
		return err
	}
	client := swarm.NewClient(addr)
	return client.Kill(cmd.Context(), args[0])
}

func runSwarmTail(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(repoPath, swarmAddr)
	if err != nil {
		return err
	}
	client := swarm.NewClient(addr)
	return streamSwarmEvents(cmd.Context(), client, args[0])
}

func runSwarmLogs(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(repoPath, swarmAddr)
	if err != nil {
		return err
	}
	client := swarm.NewClient(addr)
	events, err := client.Logs(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	formatter := job.NewEventFormatter()
	for _, event := range events {
		if err := appendAndPrintEvent(formatter, event); err != nil {
			return err
		}
	}
	return nil
}

func runSwarmList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(repoPath, swarmAddr)
	if err != nil {
		return err
	}
	client := swarm.NewClient(addr)

	jobs, err := client.List(cmd.Context(), job.ListFilter{IncludeAll: true})
	if err != nil {
		return err
	}
	allJobs := jobs

	if swarmListStatus != "" {
		status := job.Status(strings.ToLower(swarmListStatus))
		filtered := make([]job.Job, 0, len(jobs))
		for _, item := range jobs {
			if item.Status == status {
				filtered = append(filtered, item)
			}
		}
		jobs = filtered
	} else if !swarmListAll {
		filtered := make([]job.Job, 0, len(jobs))
		for _, item := range jobs {
			if item.Status == job.StatusActive {
				filtered = append(filtered, item)
			}
		}
		jobs = filtered
	}

	if swarmListJSON {
		return encodeJSONToStdout(jobs)
	}

	if len(jobs) == 0 {
		fmt.Println(jobEmptyListMessage(len(allJobs), swarmListStatus, swarmListAll))
		return nil
	}

	jobPrefixLengths := jobIDPrefixLengths(allJobs)
	if len(jobPrefixLengths) == 0 {
		jobPrefixLengths = nil
	}

	todoPrefixLengths, todoTitles, err := jobTodoTableInfo(repoPath, todoStorePurpose(cmd, args))
	if err != nil {
		return err
	}

	fmt.Print(formatJobTable(TableFormatOptions{
		Jobs:              jobs,
		Highlight:         ui.HighlightID,
		Now:               time.Now(),
		TodoPrefixLengths: todoPrefixLengths,
		TodoTitles:        todoTitles,
		JobPrefixLengths:  jobPrefixLengths,
	}))
	return nil
}

func runSwarmClient(cmd *cobra.Command, _ []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}
	addr, err := swarm.ResolveAddr(repoPath, swarmAddr)
	if err != nil {
		return err
	}
	client := swarm.NewClient(addr)
	return swarmtui.Run(cmd.Context(), client)
}

func createTodoForSwarm(cmd *cobra.Command, repoPath string, hasCreateFlags bool) (string, error) {
	return createTodoFromJobFlags(cmd, hasCreateFlags, func() (*todo.Store, error) {
		return openTodoStoreForPath(cmd, repoPath)
	})
}

func openTodoStoreForPath(cmd *cobra.Command, repoPath string) (*todo.Store, error) {
	return todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
		Purpose:         todoStorePurpose(cmd, nil),
	})
}

func resolveRepoPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return getRepoPath()
	}
	return resolveRepoRoot(path)
}

func streamSwarmEvents(parent context.Context, client *swarm.Client, jobID string) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	go func() {
		select {
		case <-interrupts:
			cancel()
		case <-ctx.Done():
		}
	}()

	formatter := job.NewEventFormatter()
	events, errs := client.Tail(ctx, jobID)
	for event := range events {
		if err := appendAndPrintEvent(formatter, event); err != nil {
			return err
		}
	}

	if err := <-errs; err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	return nil
}

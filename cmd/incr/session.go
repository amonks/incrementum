package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/ui"
	sessionpkg "github.com/amonks/incrementum/session"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage todo sessions",
}

var sessionStartCmd = &cobra.Command{
	Use:   "start <todo-id>",
	Short: "Start a new session for a todo",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionStart,
}

var sessionDoneCmd = &cobra.Command{
	Use:   "done [todo-id]",
	Short: "Mark a session completed",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionDone,
}

var sessionFailCmd = &cobra.Command{
	Use:   "fail [todo-id]",
	Short: "Mark a session failed",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionFail,
}

var sessionRunCmd = &cobra.Command{
	Use:   "run <todo-id> -- <cmd> [args...]",
	Short: "Run a command in a session workspace",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSessionRun,
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions",
	RunE:  runSessionList,
}

var (
	sessionStartTopic string
	sessionStartRev   string
	sessionRunRev     string
	sessionListJSON   bool
)

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionStartCmd, sessionDoneCmd, sessionFailCmd, sessionRunCmd, sessionListCmd)

	sessionStartCmd.Flags().StringVar(&sessionStartTopic, "topic", "", "Session topic")
	sessionStartCmd.Flags().StringVar(&sessionStartRev, "rev", "@", "Revision to check out")
	sessionRunCmd.Flags().StringVar(&sessionRunRev, "rev", "@", "Revision to check out")
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "Output as JSON")
}

func runSessionStart(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := sessionpkg.Open(repoPath, sessionpkg.OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: true, PromptToCreate: true},
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	result, err := manager.Start(args[0], sessionpkg.StartOptions{Topic: sessionStartTopic, Rev: sessionStartRev})
	if err != nil {
		return err
	}

	if err := os.Chdir(result.WorkspacePath); err != nil {
		return fmt.Errorf("change directory: %w", err)
	}

	fmt.Printf("Started session %s for %s in %s\n", result.Session.ID, ui.HighlightID(result.Session.TodoID, 0), result.Session.WorkspaceName)
	return nil
}

func runSessionDone(cmd *cobra.Command, args []string) error {
	return runSessionFinalize(args, todo.StatusDone, sessionpkg.StatusCompleted)
}

func runSessionFail(cmd *cobra.Command, args []string) error {
	return runSessionFinalize(args, todo.StatusOpen, sessionpkg.StatusFailed)
}

func runSessionFinalize(args []string, todoStatus todo.Status, sessionStatus sessionpkg.Status) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := sessionpkg.Open(repoPath, sessionpkg.OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: true, PromptToCreate: true},
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	var workspacePath string
	if len(args) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		workspacePath = cwd
	}

	finalizeTodo := ""
	if len(args) > 0 {
		finalizeTodo = args[0]
	}

	var finalized *sessionpkg.Session
	switch sessionStatus {
	case sessionpkg.StatusCompleted:
		finalized, err = manager.Done(finalizeTodo, sessionpkg.FinalizeOptions{WorkspaceName: workspacePath})
	case sessionpkg.StatusFailed:
		finalized, err = manager.Fail(finalizeTodo, sessionpkg.FinalizeOptions{WorkspaceName: workspacePath})
	default:
		return fmt.Errorf("unsupported session status: %s", sessionStatus)
	}
	if err != nil {
		return err
	}

	if err := os.Chdir(repoPath); err != nil {
		return fmt.Errorf("change directory: %w", err)
	}

	fmt.Printf("Session %s marked %s\n", finalized.ID, finalized.Status)
	return nil
}

func runSessionRun(cmd *cobra.Command, args []string) error {
	dash := cmd.ArgsLenAtDash()
	if dash == -1 {
		return fmt.Errorf("command separator -- is required")
	}
	if dash == 0 {
		return fmt.Errorf("todo id is required")
	}
	if dash >= len(args) {
		return fmt.Errorf("command is required after --")
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := sessionpkg.Open(repoPath, sessionpkg.OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: true, PromptToCreate: true},
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	cmdArgs := args[dash:]
	result, err := manager.Run(args[0], sessionpkg.RunOptions{Command: cmdArgs, Rev: sessionRunRev})
	if err != nil {
		return err
	}

	if err := os.Chdir(repoPath); err != nil {
		return fmt.Errorf("change directory: %w", err)
	}

	_ = result
	return nil
}

func runSessionList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := sessionpkg.Open(repoPath, sessionpkg.OpenOptions{
		Todo: todo.OpenOptions{CreateIfMissing: true, PromptToCreate: true},
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	sessions, err := manager.List()
	if err != nil {
		return err
	}

	if sessionListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	fmt.Print(formatSessionTable(sessions, ui.HighlightID, time.Now()))
	return nil
}

func formatSessionTable(sessions []sessionpkg.Session, highlight func(string, int) string, now time.Time) string {
	rows := make([][]string, 0, len(sessions))

	ids := make([]string, 0, len(sessions))
	for _, item := range sessions {
		ids = append(ids, item.TodoID)
	}
	prefixLengths := ui.UniqueIDPrefixLengths(ids)

	for _, item := range sessions {
		prefixLen := prefixLengths[strings.ToLower(item.TodoID)]
		id := highlight(item.TodoID, prefixLen)
		topic := item.Topic
		if topic == "" {
			topic = "-"
		}
		if len(topic) > 50 {
			topic = topic[:47] + "..."
		}
		age := sessionpkg.Age(item, now).Truncate(time.Second).String()
		exit := "-"
		if item.ExitCode != nil {
			exit = strconv.Itoa(*item.ExitCode)
		}

		row := []string{
			id,
			string(item.Status),
			item.WorkspaceName,
			age,
			topic,
			exit,
		}
		rows = append(rows, row)
	}

	return formatTable([]string{"TODO", "STATUS", "WORKSPACE", "AGE", "TOPIC", "EXIT"}, rows)
}

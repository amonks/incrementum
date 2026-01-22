package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/editor"
	"github.com/amonks/incrementum/internal/listflags"
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
	Use:   "start [todo-id]",
	Short: "Start a new session for a todo",
	Args:  cobra.MaximumNArgs(1),
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

var sessionOpen = sessionpkg.Open

var (
	sessionStartTopic  string
	sessionStartRev    string
	sessionStartTitle  string
	sessionStartType   string
	sessionStartDesc   string
	sessionStartDeps   []string
	sessionStartEdit   bool
	sessionStartNoEdit bool

	sessionStartPriority int
	sessionRunRev        string
	sessionListJSON      bool
	sessionListStatus    string
	sessionListAll       bool
)

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionStartCmd, sessionDoneCmd, sessionFailCmd, sessionRunCmd, sessionListCmd)

	sessionStartCmd.Flags().StringVar(&sessionStartTopic, "topic", "", "Session topic")
	sessionStartCmd.Flags().StringVar(&sessionStartRev, "rev", "@", "Revision to check out")
	sessionStartCmd.Flags().StringVar(&sessionStartTitle, "title", "", "Todo title")
	sessionStartCmd.Flags().StringVarP(&sessionStartType, "type", "t", "task", "Todo type (task, bug, feature)")
	sessionStartCmd.Flags().IntVarP(&sessionStartPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	sessionStartCmd.Flags().StringVarP(&sessionStartDesc, "description", "d", "", "Description (use '-' to read from stdin)")
	sessionStartCmd.Flags().StringVar(&sessionStartDesc, "desc", "", "Description (use '-' to read from stdin)")
	sessionStartCmd.Flags().StringArrayVar(&sessionStartDeps, "deps", nil, "Dependencies in format type:id (e.g., blocks:abc123)")
	sessionStartCmd.Flags().BoolVarP(&sessionStartEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	sessionStartCmd.Flags().BoolVar(&sessionStartNoEdit, "no-edit", false, "Do not open $EDITOR")
	sessionRunCmd.Flags().StringVar(&sessionRunRev, "rev", "@", "Revision to check out")
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "Output as JSON")
	sessionListCmd.Flags().StringVar(&sessionListStatus, "status", "", "Filter by status")
	listflags.AddAllFlag(sessionListCmd, &sessionListAll)
}

func runSessionStart(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
		desc, err := resolveDescriptionFromStdin(sessionStartDesc, os.Stdin)
		if err != nil {
			return err
		}
		sessionStartDesc = desc
	}

	hasCreateFlags := sessionStartHasCreateFlags(cmd)
	if len(args) > 0 && (hasCreateFlags || sessionStartEdit || sessionStartNoEdit) {
		return fmt.Errorf("todo id cannot be combined with todo creation flags")
	}

	todoID := ""
	if len(args) > 0 {
		todoID = args[0]
	} else {
		createdID, err := createTodoForSessionStart(cmd, hasCreateFlags)
		if err != nil {
			return err
		}
		todoID = createdID
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := sessionpkg.Open(repoPath, sessionpkg.OpenOptions{
		Todo:             todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
		AllowMissingTodo: true,
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	result, err := manager.Start(todoID, sessionpkg.StartOptions{Topic: sessionStartTopic, Rev: sessionStartRev})
	if err != nil {
		return err
	}

	fmt.Println(result.WorkspacePath)
	return nil
}

func createTodoForSessionStart(cmd *cobra.Command, hasCreateFlags bool) (string, error) {
	useEditor := shouldUseSessionStartEditor(hasCreateFlags, sessionStartEdit, sessionStartNoEdit, editor.IsInteractive())
	if useEditor {
		data := editor.DefaultCreateData()
		if cmd.Flags().Changed("title") {
			data.Title = sessionStartTitle
		}
		if cmd.Flags().Changed("type") {
			data.Type = sessionStartType
		}
		if cmd.Flags().Changed("priority") {
			data.Priority = sessionStartPriority
		}
		if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
			data.Description = sessionStartDesc
		}

		parsed, err := editor.EditTodoWithData(data)
		if err != nil {
			return "", err
		}

		store, err := openTodoStore()
		if err != nil {
			return "", err
		}
		defer store.Release()

		opts := parsed.ToCreateOptions()
		opts.Dependencies = sessionStartDeps

		created, err := store.Create(parsed.Title, opts)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	}

	if sessionStartTitle == "" {
		return "", fmt.Errorf("title is required (use --edit to open editor)")
	}

	store, err := openTodoStore()
	if err != nil {
		return "", err
	}
	defer store.Release()

	created, err := store.Create(sessionStartTitle, todo.CreateOptions{
		Type:         todo.TodoType(sessionStartType),
		Priority:     sessionStartPriorityValue(cmd),
		Description:  sessionStartDesc,
		Dependencies: sessionStartDeps,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func sessionStartHasCreateFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("title") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("priority") ||
		cmd.Flags().Changed("description") ||
		cmd.Flags().Changed("desc") ||
		cmd.Flags().Changed("deps")
}

func shouldUseSessionStartEditor(hasCreateFlags bool, editFlag bool, noEditFlag bool, interactive bool) bool {
	if editFlag {
		return true
	}
	if noEditFlag {
		return false
	}
	if hasCreateFlags {
		return false
	}
	return interactive
}

func sessionStartPriorityValue(cmd *cobra.Command) *int {
	if cmd.Flags().Changed("priority") {
		return todo.PriorityPtr(sessionStartPriority)
	}
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

	manager, err := sessionpkg.Open(repoPath, sessionListOpenOptions())
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
		finalized, err = manager.Done(finalizeTodo, sessionpkg.FinalizeOptions{WorkspacePath: workspacePath})
	case sessionpkg.StatusFailed:
		finalized, err = manager.Fail(finalizeTodo, sessionpkg.FinalizeOptions{WorkspacePath: workspacePath})
	default:
		return fmt.Errorf("unsupported session status: %s", sessionStatus)
	}
	if err != nil {
		return err
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
		if result != nil {
			fmt.Printf("Session %s marked %s (exit %d)\n", result.Session.ID, result.Session.Status, result.ExitCode)
		}
		return err
	}

	fmt.Printf("Session %s marked %s (exit %d)\n", result.Session.ID, result.Session.Status, result.ExitCode)
	return nil
}

func runSessionList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := sessionOpen(repoPath, sessionListOpenOptions())
	if err != nil {
		return err
	}
	defer manager.Close()

	filter := sessionpkg.ListFilter{IncludeAll: sessionListAll}
	if sessionListStatus != "" {
		status := sessionpkg.Status(sessionListStatus)
		filter.Status = &status
	}

	sessions, err := manager.List(filter)
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

	todoPrefixLengths, err := manager.TodoIDPrefixLengths()
	if err != nil {
		return err
	}

	fmt.Print(formatSessionTable(sessions, ui.HighlightID, time.Now(), todoPrefixLengths))
	return nil
}

func sessionListOpenOptions() sessionpkg.OpenOptions {
	return sessionpkg.OpenOptions{
		Todo:             todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false},
		AllowMissingTodo: true,
	}
}

func formatSessionTable(sessions []sessionpkg.Session, highlight func(string, int) string, now time.Time, todoPrefixLengths map[string]int) string {
	rows := make([][]string, 0, len(sessions))

	sessionIDs := make([]string, 0, len(sessions))
	todoIDs := make([]string, 0, len(sessions))
	for _, item := range sessions {
		sessionIDs = append(sessionIDs, item.ID)
		todoIDs = append(todoIDs, item.TodoID)
	}
	sessionPrefixLengths := ui.UniqueIDPrefixLengths(sessionIDs)
	if todoPrefixLengths == nil {
		todoPrefixLengths = ui.UniqueIDPrefixLengths(todoIDs)
	}

	for _, item := range sessions {
		sessionPrefixLen := sessionPrefixLengths[strings.ToLower(item.ID)]
		sessionID := highlight(item.ID, sessionPrefixLen)
		todoPrefixLen := 0
		if length, ok := todoPrefixLengths[strings.ToLower(item.TodoID)]; ok {
			todoPrefixLen = length
		}
		id := highlight(item.TodoID, todoPrefixLen)
		topic := item.Topic
		if topic == "" {
			topic = "-"
		}
		topic = truncateTableCell(topic)
		age := formatSessionAge(item, now)
		exit := "-"
		if item.ExitCode != nil {
			exit = strconv.Itoa(*item.ExitCode)
		}

		row := []string{
			sessionID,
			id,
			string(item.Status),
			item.WorkspaceName,
			age,
			topic,
			exit,
		}
		rows = append(rows, row)
	}

	return formatTable([]string{"SESSION", "TODO", "STATUS", "WORKSPACE", "AGE", "TOPIC", "EXIT"}, rows)
}

func formatSessionAge(item sessionpkg.Session, now time.Time) string {
	if item.Status == sessionpkg.StatusActive {
		if item.StartedAt.IsZero() {
			return "-"
		}
		return ui.FormatDurationShort(sessionpkg.Age(item, now))
	}

	if item.DurationSeconds > 0 {
		return ui.FormatDurationShort(sessionpkg.Age(item, now))
	}

	if !item.CompletedAt.IsZero() && !item.StartedAt.IsZero() {
		return ui.FormatDurationShort(sessionpkg.Age(item, now))
	}

	return "-"
}

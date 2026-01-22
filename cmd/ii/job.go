package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/ui"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/workspace"
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage opencode jobs",
}

var jobShowCmd = &cobra.Command{
	Use:   "show <job-id>",
	Short: "Show job details",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobShow,
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs",
	RunE:  runJobList,
}

var jobLogsCmd = &cobra.Command{
	Use:   "logs <job-id>",
	Short: "Show job logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runJobLogs,
}

var jobOpen = jobpkg.Open

var (
	jobListJSON   bool
	jobListStatus string
	jobListAll    bool
)

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobShowCmd, jobListCmd, jobLogsCmd)

	jobListCmd.Flags().BoolVar(&jobListJSON, "json", false, "Output as JSON")
	jobListCmd.Flags().StringVar(&jobListStatus, "status", "", "Filter by status")
	listflags.AddAllFlag(jobListCmd, &jobListAll)
}

func runJobShow(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := jobOpen(repoPath, jobpkg.OpenOptions{})
	if err != nil {
		return err
	}

	item, err := manager.Find(args[0])
	if err != nil {
		return err
	}

	jobPrefixLengths, err := jobShowPrefixLengths(manager)
	if err != nil {
		return err
	}

	todoTitle, todoPrefixLengths, err := jobShowTodoInfo(repoPath, item.TodoID)
	if err != nil {
		return err
	}

	jobHighlight := jobLogHighlighter(jobPrefixLengths, ui.HighlightID)
	todoHighlight := todoLogHighlighter(todoPrefixLengths, ui.HighlightID)
	printJobDetail(item, todoTitle, jobHighlight, todoHighlight)
	return nil
}

func runJobList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := jobOpen(repoPath, jobpkg.OpenOptions{})
	if err != nil {
		return err
	}

	filter := jobpkg.ListFilter{IncludeAll: jobListAll}
	if jobListStatus != "" {
		status := jobpkg.Status(jobListStatus)
		filter.Status = &status
	}

	jobs, err := manager.List(filter)
	if err != nil {
		return err
	}

	if jobListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jobs)
	}

	allJobs := jobs
	if jobListStatus != "" || !jobListAll {
		allJobs, err = manager.List(jobpkg.ListFilter{IncludeAll: true})
		if err != nil {
			return err
		}
	}

	if len(jobs) == 0 {
		fmt.Println(jobEmptyListMessage(len(allJobs), jobListStatus, jobListAll))
		return nil
	}

	jobPrefixLengths := jobIDPrefixLengths(allJobs)
	if len(jobPrefixLengths) == 0 {
		jobPrefixLengths = nil
	}

	todoPrefixLengths, err := jobTodoPrefixLengths(repoPath)
	if err != nil {
		return err
	}

	fmt.Print(formatJobTable(jobs, ui.HighlightID, time.Now(), todoPrefixLengths, jobPrefixLengths))
	return nil
}

func runJobLogs(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	manager, err := jobOpen(repoPath, jobpkg.OpenOptions{})
	if err != nil {
		return err
	}

	item, err := manager.Find(args[0])
	if err != nil {
		return err
	}

	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	snapshot, err := jobLogSnapshot(pool, repoPath, item)
	if err != nil {
		return err
	}

	fmt.Print(snapshot)
	return nil
}

func jobIDPrefixLengths(jobs []jobpkg.Job) map[string]int {
	ids := make([]string, 0, len(jobs))
	for _, item := range jobs {
		ids = append(ids, item.ID)
	}
	return ui.UniqueIDPrefixLengths(ids)
}

func jobTodoPrefixLengths(repoPath string) (map[string]int, error) {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		if errors.Is(err, todo.ErrNoTodoStore) {
			return nil, nil
		}
		return nil, err
	}
	defer store.Release()

	index, err := store.IDIndex()
	if err != nil {
		return nil, err
	}
	return index.PrefixLengths(), nil
}

func formatJobTable(jobs []jobpkg.Job, highlight func(string, int) string, now time.Time, todoPrefixLengths map[string]int, jobPrefixLengths map[string]int) string {
	rows := make([][]string, 0, len(jobs))

	jobIDs := make([]string, 0, len(jobs))
	todoIDs := make([]string, 0, len(jobs))
	for _, item := range jobs {
		jobIDs = append(jobIDs, item.ID)
		todoIDs = append(todoIDs, item.TodoID)
	}

	if jobPrefixLengths == nil {
		jobPrefixLengths = ui.UniqueIDPrefixLengths(jobIDs)
	}

	todoFallback := ui.UniqueIDPrefixLengths(todoIDs)
	if todoPrefixLengths == nil {
		todoPrefixLengths = todoFallback
	} else {
		for _, id := range todoIDs {
			if _, ok := todoPrefixLengths[strings.ToLower(id)]; !ok {
				todoPrefixLengths = todoFallback
				break
			}
		}
	}

	for _, item := range jobs {
		jobPrefixLen := jobPrefixLengths[strings.ToLower(item.ID)]
		jobID := highlight(item.ID, jobPrefixLen)
		todoPrefixLen := 0
		if length, ok := todoPrefixLengths[strings.ToLower(item.TodoID)]; ok {
			todoPrefixLen = length
		}
		todoID := highlight(item.TodoID, todoPrefixLen)
		age := formatJobAge(item, now)

		row := []string{
			jobID,
			todoID,
			string(item.Stage),
			string(item.Status),
			age,
		}
		rows = append(rows, row)
	}

	return formatTable([]string{"JOB", "TODO", "STAGE", "STATUS", "AGE"}, rows)
}

func formatJobAge(item jobpkg.Job, now time.Time) string {
	ageValue, ok := jobpkg.AgeData(item, now)
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(ageValue)
}

func printJobDetail(item jobpkg.Job, todoTitle string, highlightJob func(string) string, highlightTodo func(string) string) {
	todoLine := highlightTodo(item.TodoID)
	if todoTitle != "" {
		todoLine = fmt.Sprintf("%s - %s", todoLine, todoTitle)
	}

	fmt.Printf("ID:      %s\n", highlightJob(item.ID))
	fmt.Printf("Todo:    %s\n", todoLine)
	fmt.Printf("Session: %s\n", item.SessionID)
	fmt.Printf("Stage:   %s\n", item.Stage)
	fmt.Printf("Status:  %s\n", item.Status)

	if len(item.OpencodeSessions) > 0 {
		fmt.Printf("\nOpencode Sessions:\n")
		for _, session := range item.OpencodeSessions {
			fmt.Printf("- %s: %s\n", session.Purpose, session.ID)
		}
	}

	if item.Feedback != "" {
		fmt.Printf("\nFeedback:\n%s\n", item.Feedback)
	}
}

func jobLogHighlighter(prefixLengths map[string]int, highlight func(string, int) string) func(string) string {
	if prefixLengths == nil {
		prefixLengths = map[string]int{}
	}
	return func(id string) string {
		if id == "" {
			return id
		}
		prefixLen, ok := prefixLengths[strings.ToLower(id)]
		if !ok {
			return highlight(id, 0)
		}
		return highlight(id, prefixLen)
	}
}

func jobShowPrefixLengths(manager *jobpkg.Manager) (map[string]int, error) {
	allJobs, err := manager.List(jobpkg.ListFilter{IncludeAll: true})
	if err != nil {
		return nil, err
	}
	if len(allJobs) == 0 {
		return nil, nil
	}
	return jobIDPrefixLengths(allJobs), nil
}

func jobShowTodoInfo(repoPath string, todoID string) (string, map[string]int, error) {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		if errors.Is(err, todo.ErrNoTodoStore) {
			return "", nil, nil
		}
		return "", nil, err
	}
	defer store.Release()

	index, err := store.IDIndex()
	if err != nil {
		return "", nil, err
	}
	prefixLengths := index.PrefixLengths()

	todos, err := store.Show([]string{todoID})
	if err != nil {
		if errors.Is(err, todo.ErrTodoNotFound) {
			return "", prefixLengths, nil
		}
		return "", nil, err
	}
	if len(todos) == 0 {
		return "", prefixLengths, nil
	}

	return todos[0].Title, prefixLengths, nil
}

type jobLogEntry struct {
	Purpose string
	Session workspace.OpencodeSession
	Log     string
}

func jobLogSnapshot(pool *workspace.Pool, repoPath string, item jobpkg.Job) (string, error) {
	if len(item.OpencodeSessions) == 0 {
		return "", nil
	}

	entries := make([]jobLogEntry, 0, len(item.OpencodeSessions))
	for _, session := range item.OpencodeSessions {
		entry, err := jobLogEntryForSession(pool, repoPath, session)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Session.StartedAt.Equal(entries[j].Session.StartedAt) {
			return entries[i].Session.ID < entries[j].Session.ID
		}
		return entries[i].Session.StartedAt.Before(entries[j].Session.StartedAt)
	})

	var builder strings.Builder
	for i, entry := range entries {
		if i > 0 {
			builder.WriteString("\n")
		}
		fmt.Fprintf(&builder, "==> %s (%s)\n", entry.Purpose, entry.Session.ID)
		builder.WriteString(entry.Log)
		if !strings.HasSuffix(entry.Log, "\n") {
			builder.WriteString("\n")
		}
	}

	return builder.String(), nil
}

func jobLogEntryForSession(pool *workspace.Pool, repoPath string, session jobpkg.OpencodeSession) (jobLogEntry, error) {
	opencodeSession, err := pool.FindOpencodeSession(repoPath, session.ID)
	if err != nil {
		return jobLogEntry{}, err
	}
	if opencodeSession.LogPath == "" {
		return jobLogEntry{}, fmt.Errorf("opencode session log path missing")
	}

	logSnapshot, err := opencodeLogSnapshot(opencodeSession.LogPath)
	if err != nil {
		return jobLogEntry{}, err
	}

	return jobLogEntry{
		Purpose: session.Purpose,
		Session: opencodeSession,
		Log:     logSnapshot,
	}, nil
}

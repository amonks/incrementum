package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/amonks/incrementum/internal/editor"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
)

var jobDoCmd = &cobra.Command{
	Use:   "do [todo-id]",
	Short: "Run a job for a todo",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runJobDo,
}

var jobRun = jobpkg.Run

var (
	jobDoTitle       string
	jobDoType        string
	jobDoPriority    int
	jobDoDescription string
	jobDoDeps        []string
	jobDoEdit        bool
	jobDoNoEdit      bool
)

func init() {
	jobCmd.AddCommand(jobDoCmd)
	addDescriptionFlagAliases(jobDoCmd)

	jobDoCmd.Flags().StringVar(&jobDoTitle, "title", "", "Todo title")
	jobDoCmd.Flags().StringVarP(&jobDoType, "type", "t", "task", "Todo type (task, bug, feature)")
	jobDoCmd.Flags().IntVarP(&jobDoPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	jobDoCmd.Flags().StringVarP(&jobDoDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	jobDoCmd.Flags().StringArrayVar(&jobDoDeps, "deps", nil, "Dependencies in format <id> (e.g., abc123)")
	jobDoCmd.Flags().BoolVarP(&jobDoEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	jobDoCmd.Flags().BoolVar(&jobDoNoEdit, "no-edit", false, "Do not open $EDITOR")
}

func runJobDo(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("description") {
		desc, err := resolveDescriptionFromStdin(jobDoDescription, os.Stdin)
		if err != nil {
			return err
		}
		jobDoDescription = desc
	}

	hasCreateFlags := jobDoHasCreateFlags(cmd)
	if len(args) > 0 && (hasCreateFlags || jobDoEdit || jobDoNoEdit) {
		return fmt.Errorf("todo id cannot be combined with todo creation flags")
	}

	todoID := ""
	if len(args) > 0 {
		todoID = args[0]
	} else {
		createdID, err := createTodoForJob(cmd, hasCreateFlags)
		if err != nil {
			return err
		}
		todoID = createdID
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	logger := jobpkg.NewConsoleLogger(os.Stdout)
	reporter := newJobStageReporter(logger)
	onStageChange := reporter.OnStageChange
	onStart := func(info jobpkg.StartInfo) {
		printJobStart(info)
	}

	result, err := jobRun(repoPath, todoID, jobpkg.RunOptions{OnStart: onStart, OnStageChange: onStageChange, Logger: logger})
	if err != nil {
		return err
	}

	if len(result.CommitLog) > 0 {
		fmt.Printf("\n%s\n", formatCommitMessagesOutput(result.CommitLog))
	} else if strings.TrimSpace(result.CommitMessage) != "" {
		fmt.Printf("\n%s\n", formatCommitMessageOutput(result.CommitMessage))
	}
	return nil
}

func formatCommitMessagesOutput(entries []jobpkg.CommitLogEntry) string {
	var out strings.Builder
	out.WriteString("Commit messages:\n")
	for _, entry := range entries {
		out.WriteString("\n")
		label := "Commit"
		if strings.TrimSpace(entry.ID) != "" {
			label = fmt.Sprintf("Commit %s", entry.ID)
		}
		out.WriteString(label)
		out.WriteString(":\n\n")
		message := strings.TrimRight(entry.Message, "\r\n")
		if strings.TrimSpace(message) == "" {
			message = "-"
		}
		out.WriteString(message)
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func formatCommitMessageOutput(message string) string {
	message = strings.TrimRight(message, "\r\n")
	if strings.TrimSpace(message) == "" {
		message = "-"
	}
	return fmt.Sprintf("Commit message:\n\n%s", message)
}

func printJobStart(info jobpkg.StartInfo) {
	fmt.Printf("Doing job %s\n", info.JobID)
	fmt.Printf("Workdir: %s\n", info.Workdir)
	fmt.Println("Todo:")
	indent := strings.Repeat(" ", jobDocumentIndent)
	fmt.Printf("%sID: %s\n", indent, info.Todo.ID)
	fmt.Printf("%sTitle: %s\n", indent, info.Todo.Title)
	fmt.Printf("%sType: %s\n", indent, info.Todo.Type)
	fmt.Printf("%sPriority: %d (%s)\n", indent, info.Todo.Priority, todo.PriorityName(info.Todo.Priority))
	fmt.Printf("%sDescription:\n", indent)
	description := reflowJobText(info.Todo.Description, jobLineWidth-jobSubdocumentIndent)
	fmt.Printf("%s\n\n", indentBlock(description, jobSubdocumentIndent))
}

func jobDoHasCreateFlags(cmd *cobra.Command) bool {
	return hasChangedFlags(cmd, "title", "type", "priority", "description", "deps")
}

func createTodoForJob(cmd *cobra.Command, hasCreateFlags bool) (string, error) {
	useEditor := shouldUseEditor(hasCreateFlags, jobDoEdit, jobDoNoEdit, editor.IsInteractive())
	if useEditor {
		data := editor.DefaultCreateData()
		if cmd.Flags().Changed("title") {
			data.Title = jobDoTitle
		}
		if cmd.Flags().Changed("type") {
			data.Type = jobDoType
		}
		if cmd.Flags().Changed("priority") {
			data.Priority = jobDoPriority
		}
		if cmd.Flags().Changed("description") {
			data.Description = jobDoDescription
		}

		parsed, err := editor.EditTodoWithData(data)
		if err != nil {
			return "", err
		}

		store, err := openTodoStore(cmd, nil)
		if err != nil {
			return "", err
		}
		defer store.Release()

		opts := parsed.ToCreateOptions()
		opts.Dependencies = jobDoDeps
		created, err := store.Create(parsed.Title, opts)
		if err != nil {
			return "", err
		}
		return created.ID, nil
	}

	if jobDoTitle == "" {
		return "", fmt.Errorf("title is required (use --edit to open editor)")
	}

	store, err := openTodoStore(cmd, nil)
	if err != nil {
		return "", err
	}
	defer store.Release()

	created, err := store.Create(jobDoTitle, todo.CreateOptions{
		Type:         todo.TodoType(jobDoType),
		Priority:     jobDoPriorityValue(cmd),
		Description:  jobDoDescription,
		Dependencies: jobDoDeps,
	})
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func jobDoPriorityValue(cmd *cobra.Command) *int {
	if cmd.Flags().Changed("priority") {
		return todo.PriorityPtr(jobDoPriority)
	}
	return nil
}

type jobStageReporter struct {
	logger  *jobpkg.ConsoleLogger
	started bool
}

func newJobStageReporter(logger *jobpkg.ConsoleLogger) *jobStageReporter {
	return &jobStageReporter{logger: logger}
}

func (reporter *jobStageReporter) OnStageChange(stage jobpkg.Stage) {
	if reporter.started {
		fmt.Println()
	}
	reporter.started = true
	fmt.Println(stageMessage(stage))
	if reporter.logger != nil {
		reporter.logger.ResetSpacing()
	}
}

func stageMessage(stage jobpkg.Stage) string {
	switch stage {
	case jobpkg.StageImplementing:
		return "Running implementation prompt:"
	case jobpkg.StageTesting:
		return "Implementation prompt complete; running tests:"
	case jobpkg.StageReviewing:
		return "Tests passed; doing review:"
	case jobpkg.StageCommitting:
		return "Review complete; committing changes:"
	default:
		return fmt.Sprintf("Stage: %s", stage)
	}
}

const (
	jobLineWidth         = 80
	jobDocumentIndent    = 4
	jobSubdocumentIndent = 8
)

func reflowJobText(value string, width int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	paragraphs := splitJobParagraphs(value)
	wrapped := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		normalized := internalstrings.NormalizeWhitespace(paragraph)
		if normalized == "" {
			continue
		}
		wrapped = append(wrapped, wordwrap.String(normalized, width))
	}
	if len(wrapped) == 0 {
		return "-"
	}
	return strings.Join(wrapped, "\n\n")
}

func splitJobParagraphs(value string) []string {
	lines := strings.Split(value, "\n")
	var paragraphs []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, " "))
		current = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

func indentBlock(value string, spaces int) string {
	value = strings.TrimRight(value, "\r\n")
	if spaces <= 0 {
		return value
	}
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

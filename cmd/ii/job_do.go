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
	jobDoAgent       string
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
	jobDoCmd.Flags().StringVar(&jobDoAgent, "agent", "", "Opencode agent")
}

func runJobDo(cmd *cobra.Command, args []string) error {
	if err := resolveDescriptionFlag(cmd, &jobDoDescription, os.Stdin); err != nil {
		return err
	}

	hasCreateFlags := hasTodoCreateFlags(cmd)
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
	eventStream := make(chan jobpkg.Event, 128)
	eventErrs := make(chan error, 1)
	go func() {
		formatter := jobpkg.NewEventFormatter()
		var streamErr error
		for event := range eventStream {
			if strings.HasPrefix(event.Name, "job.") {
				continue
			}
			if err := appendAndPrintEvent(formatter, event); err != nil {
				if streamErr == nil {
					streamErr = err
				}
				continue
			}
		}
		eventErrs <- streamErr
	}()

	result, err := jobRun(repoPath, todoID, jobpkg.RunOptions{
		OnStart:       onStart,
		OnStageChange: onStageChange,
		Logger:        logger,
		EventStream:   eventStream,
		OpencodeAgent: resolveOpencodeAgent(cmd, jobDoAgent),
	})
	streamErr := <-eventErrs
	if err != nil {
		return err
	}
	if streamErr != nil {
		return streamErr
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
		out.WriteString(jobpkg.IndentBlock(label+":", jobDocumentIndent))
		out.WriteString("\n")
		out.WriteString(formatCommitMessageBody(entry.Message, jobSubdocumentIndent))
		out.WriteString("\n")
	}
	return internalstrings.TrimTrailingNewlines(out.String())
}

func formatCommitMessageOutput(message string) string {
	formatted := formatCommitMessageBody(message, jobDocumentIndent)
	return fmt.Sprintf("Commit message:\n\n%s", formatted)
}

func formatCommitMessageBody(message string, indent int) string {
	message = internalstrings.TrimTrailingNewlines(message)
	if strings.TrimSpace(message) == "" {
		return jobpkg.IndentBlock("-", indent)
	}
	width := jobLineWidth - indent
	formatted := renderMarkdownOrDash(message, width)
	return jobpkg.IndentBlock(formatted, indent)
}

func printJobStart(info jobpkg.StartInfo) {
	fmt.Printf("Doing job %s\n", info.JobID)
	fmt.Printf("Workdir: %s\n", info.Workdir)
	fmt.Println("Todo:")
	indent := strings.Repeat(" ", jobDocumentIndent)
	fmt.Printf("%s\n", formatJobField("ID", info.Todo.ID))
	fmt.Printf("%s\n", formatJobField("Title", info.Todo.Title))
	fmt.Printf("%s\n", formatJobField("Type", string(info.Todo.Type)))
	fmt.Printf("%s\n", formatJobField("Priority", fmt.Sprintf("%d (%s)", info.Todo.Priority, todo.PriorityName(info.Todo.Priority))))
	fmt.Printf("%sDescription:\n", indent)
	description := reflowJobText(info.Todo.Description, jobLineWidth-jobSubdocumentIndent)
	fmt.Printf("%s\n\n", jobpkg.IndentBlock(description, jobSubdocumentIndent))
}

func createTodoForJob(cmd *cobra.Command, hasCreateFlags bool) (string, error) {
	return createTodoFromJobFlags(cmd, hasCreateFlags, func() (*todo.Store, error) {
		return openTodoStore(cmd, nil)
	})
}

func createTodoFromJobFlags(cmd *cobra.Command, hasCreateFlags bool, openStore func() (*todo.Store, error)) (string, error) {
	useEditor := shouldUseEditor(hasCreateFlags, jobDoEdit, jobDoNoEdit, editor.IsInteractive())
	if useEditor {
		data := editor.DefaultCreateData()
		data.Status = string(defaultTodoStatus())
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

		store, err := openStore()
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

	store, err := openStore()
	if err != nil {
		return "", err
	}
	defer store.Release()

	created, err := store.Create(jobDoTitle, todo.CreateOptions{
		Status:       defaultTodoStatus(),
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
	fmt.Println(jobpkg.StageMessage(stage))
	if reporter.logger != nil {
		reporter.logger.ResetSpacing()
	}
}

const (
	jobLineWidth         = 80
	jobDocumentIndent    = 4
	jobSubdocumentIndent = 8
)

func formatJobField(label, value string) string {
	prefix := fmt.Sprintf("%s: ", label)
	value = strings.TrimSpace(value)
	if value == "" {
		value = "-"
	}
	value = internalstrings.NormalizeWhitespace(value)
	if value == "" {
		value = "-"
	}

	wrapWidth := jobLineWidth - jobDocumentIndent - len(prefix)
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	wrapped := wordwrap.String(value, wrapWidth)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefix + line
			continue
		}
		lines[i] = strings.Repeat(" ", len(prefix)) + line
	}
	return jobpkg.IndentBlock(strings.Join(lines, "\n"), jobDocumentIndent)
}

func reflowJobText(value string, width int) string {
	return renderMarkdownOrDash(value, width)
}

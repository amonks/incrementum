package main

import (
	"fmt"
	"os"

	"github.com/amonks/incrementum/internal/editor"
	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
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
	jobDoRev         string
)

func init() {
	jobCmd.AddCommand(jobDoCmd)

	jobDoCmd.Flags().StringVar(&jobDoTitle, "title", "", "Todo title")
	jobDoCmd.Flags().StringVarP(&jobDoType, "type", "t", "task", "Todo type (task, bug, feature)")
	jobDoCmd.Flags().IntVarP(&jobDoPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	jobDoCmd.Flags().StringVarP(&jobDoDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	jobDoCmd.Flags().StringVar(&jobDoDescription, "desc", "", "Description (use '-' to read from stdin)")
	jobDoCmd.Flags().StringArrayVar(&jobDoDeps, "deps", nil, "Dependencies in format type:id (e.g., blocks:abc123)")
	jobDoCmd.Flags().BoolVarP(&jobDoEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	jobDoCmd.Flags().BoolVar(&jobDoNoEdit, "no-edit", false, "Do not open $EDITOR")
	jobDoCmd.Flags().StringVar(&jobDoRev, "rev", "@", "Revision to check out")
}

func runJobDo(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
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

	onStageChange := func(stage jobpkg.Stage) {
		fmt.Printf("Stage: %s\n", stage)
	}

	result, err := jobRun(repoPath, todoID, jobpkg.RunOptions{Rev: jobDoRev, OnStageChange: onStageChange})
	if err != nil {
		return err
	}

	if result.CommitMessage != "" {
		fmt.Printf("\nCommit message:\n%s\n", result.CommitMessage)
	}
	return nil
}

func jobDoHasCreateFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("title") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("priority") ||
		cmd.Flags().Changed("description") ||
		cmd.Flags().Changed("desc") ||
		cmd.Flags().Changed("deps")
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
		if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
			data.Description = jobDoDescription
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

	store, err := openTodoStore()
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

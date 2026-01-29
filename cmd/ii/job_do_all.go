package main

import (
	"fmt"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/internal/validation"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

var jobDoAllCmd = &cobra.Command{
	Use:   "do-all",
	Short: "Run jobs for all ready todos",
	Args:  cobra.NoArgs,
	RunE:  runJobDoAll,
}

var (
	jobDoAllPriority int
	jobDoAllType     string
)

type jobDoAllFilter struct {
	maxPriority *int
	todoType    *todo.TodoType
}

func init() {
	jobCmd.AddCommand(jobDoAllCmd)

	jobDoAllCmd.Flags().IntVar(&jobDoAllPriority, "priority", -1, "Filter by priority (0-4, includes higher priorities)")
	jobDoAllCmd.Flags().StringVar(&jobDoAllType, "type", "", "Filter by type (task, bug, feature); design todos are excluded")
}

func runJobDoAll(cmd *cobra.Command, args []string) error {
	filter, err := jobDoAllFilters(cmd)
	if err != nil {
		return err
	}

	for {
		store, handled, err := openTodoStoreReadOnlyOrEmpty(cmd, args, false, func() error {
			fmt.Println("nothing left to do")
			return nil
		})
		if err != nil {
			return err
		}
		if handled {
			return nil
		}

		todoID, err := nextJobDoAllTodoID(store, filter)
		store.Release()
		if err != nil {
			return err
		}
		if todoID == "" {
			fmt.Println("nothing left to do")
			return nil
		}

		if err := runJobDoTodo(cmd, todoID); err != nil {
			return err
		}
	}
}

func jobDoAllFilters(cmd *cobra.Command) (jobDoAllFilter, error) {
	filter := jobDoAllFilter{}
	if cmd.Flags().Changed("priority") {
		if err := todo.ValidatePriority(jobDoAllPriority); err != nil {
			return filter, err
		}
		filter.maxPriority = &jobDoAllPriority
	}

	if cmd.Flags().Changed("type") {
		normalized := todo.TodoType(internalstrings.NormalizeLowerTrimSpace(jobDoAllType))
		if !normalized.IsValid() {
			return filter, validation.FormatInvalidValueError(todo.ErrInvalidType, normalized, todo.ValidTodoTypes())
		}
		if normalized.IsInteractive() {
			return filter, fmt.Errorf("%s todos require interactive sessions and cannot be run with do-all", normalized)
		}
		filter.todoType = &normalized
	}

	return filter, nil
}

func nextJobDoAllTodoID(store *todo.Store, filter jobDoAllFilter) (string, error) {
	todos, err := store.Ready(0)
	if err != nil {
		return "", err
	}

	for _, item := range todos {
		// Skip interactive todos (e.g., design) since they require user collaboration
		if item.Type.IsInteractive() {
			continue
		}
		if filter.maxPriority != nil && item.Priority > *filter.maxPriority {
			continue
		}
		if filter.todoType != nil && item.Type != *filter.todoType {
			continue
		}
		return item.ID, nil
	}
	return "", nil
}

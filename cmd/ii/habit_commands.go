package main

import (
	"fmt"
	"os"

	"github.com/amonks/incrementum/habit"
	"github.com/amonks/incrementum/internal/editor"
	"github.com/spf13/cobra"
)

var habitCmd = &cobra.Command{
	Use:   "habit",
	Short: "Manage habits for the current repository",
}

// habit list
var habitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all habits",
	Args:  cobra.NoArgs,
	RunE:  runHabitList,
}

// habit show
var habitShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show habit instructions",
	Args:  cobra.ExactArgs(1),
	RunE:  runHabitShow,
}

// habit edit
var habitEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit habit instructions in $EDITOR",
	Aliases: []string{
		"update",
	},
	Args: cobra.ExactArgs(1),
	RunE: runHabitEdit,
}

// habit create
var habitCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new habit and open it in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE:  runHabitCreate,
}

func init() {
	rootCmd.AddCommand(habitCmd)
	habitCmd.AddCommand(habitListCmd, habitShowCmd, habitEditCmd, habitCreateCmd)
}

func runHabitList(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	names, err := habit.List(repoPath)
	if err != nil {
		return err
	}

	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}

func runHabitShow(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	name := args[0]
	path, err := habit.Path(repoPath, name)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("habit not found: %s", name)
		}
		return fmt.Errorf("read habit: %w", err)
	}

	fmt.Print(string(content))
	return nil
}

func runHabitEdit(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	name := args[0]
	exists, err := habit.Exists(repoPath, name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("habit not found: %s", name)
	}

	path, err := habit.Path(repoPath, name)
	if err != nil {
		return err
	}

	return editor.Edit(path)
}

func runHabitCreate(cmd *cobra.Command, args []string) error {
	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	name := args[0]
	path, err := habit.Create(repoPath, name)
	if err != nil {
		return err
	}

	fmt.Printf("Created habit: %s\n", path)
	return editor.Edit(path)
}

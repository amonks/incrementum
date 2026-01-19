package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage todos for the current repository",
}

// todo create
var todoCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new todo",
	Args:  cobra.ExactArgs(1),
	RunE:  runTodoCreate,
}

var (
	todoCreateType        string
	todoCreatePriority    int
	todoCreateDescription string
	todoCreateDeps        []string
)

// todo update
var todoUpdateCmd = &cobra.Command{
	Use:   "update <id>...",
	Short: "Update one or more todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoUpdate,
}

var (
	todoUpdateTitle              string
	todoUpdateDescription        string
	todoUpdateDesign             string
	todoUpdateAcceptanceCriteria string
	todoUpdateNotes              string
	todoUpdateStatus             string
	todoUpdatePriority           int
	todoUpdateType               string
	todoUpdatePrioritySet        bool
)

// todo close
var todoCloseCmd = &cobra.Command{
	Use:   "close <id>...",
	Short: "Close one or more todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoClose,
}

var todoCloseReason string

// todo reopen
var todoReopenCmd = &cobra.Command{
	Use:   "reopen <id>...",
	Short: "Reopen one or more closed todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoReopen,
}

var todoReopenReason string

// todo show
var todoShowCmd = &cobra.Command{
	Use:   "show <id>...",
	Short: "Show detailed information about todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoShow,
}

var todoShowJSON bool

// todo list
var todoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List todos",
	RunE:  runTodoList,
}

var (
	todoListStatus      string
	todoListPriority    int
	todoListType        string
	todoListIDs         string
	todoListTitle       string
	todoListDescription string
	todoListJSON        bool
	todoListPrioritySet bool
)

// todo ready
var todoReadyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List todos ready to work on (no unresolved blockers)",
	RunE:  runTodoReady,
}

var (
	todoReadyLimit int
	todoReadyJSON  bool
)

// todo dep
var todoDepCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage todo dependencies",
}

// todo dep add
var todoDepAddCmd = &cobra.Command{
	Use:   "add <todo-id> <depends-on-id>",
	Short: "Add a dependency between todos",
	Args:  cobra.ExactArgs(2),
	RunE:  runTodoDepAdd,
}

var todoDepAddType string

// todo dep tree
var todoDepTreeCmd = &cobra.Command{
	Use:   "tree <id>",
	Short: "Show dependency tree for a todo",
	Args:  cobra.ExactArgs(1),
	RunE:  runTodoDepTree,
}

func init() {
	rootCmd.AddCommand(todoCmd)
	todoCmd.AddCommand(todoCreateCmd, todoUpdateCmd, todoCloseCmd, todoReopenCmd,
		todoShowCmd, todoListCmd, todoReadyCmd, todoDepCmd)
	todoDepCmd.AddCommand(todoDepAddCmd, todoDepTreeCmd)

	// todo create flags
	todoCreateCmd.Flags().StringVarP(&todoCreateType, "type", "t", "task", "Todo type (task, bug, feature)")
	todoCreateCmd.Flags().IntVarP(&todoCreatePriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	todoCreateCmd.Flags().StringVarP(&todoCreateDescription, "description", "d", "", "Description")
	todoCreateCmd.Flags().StringArrayVar(&todoCreateDeps, "deps", nil, "Dependencies in format type:id (e.g., blocks:abc123)")

	// todo update flags
	todoUpdateCmd.Flags().StringVar(&todoUpdateTitle, "title", "", "New title")
	todoUpdateCmd.Flags().StringVar(&todoUpdateDescription, "description", "", "New description")
	todoUpdateCmd.Flags().StringVar(&todoUpdateDesign, "design", "", "Design notes")
	todoUpdateCmd.Flags().StringVar(&todoUpdateAcceptanceCriteria, "acceptance-criteria", "", "Acceptance criteria")
	todoUpdateCmd.Flags().StringVar(&todoUpdateNotes, "notes", "", "Notes")
	todoUpdateCmd.Flags().StringVar(&todoUpdateStatus, "status", "", "New status (open, in_progress, closed)")
	todoUpdateCmd.Flags().IntVar(&todoUpdatePriority, "priority", 0, "New priority (0-4)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateType, "type", "", "New type (task, bug, feature)")

	// todo close flags
	todoCloseCmd.Flags().StringVar(&todoCloseReason, "reason", "", "Reason for closing")

	// todo reopen flags
	todoReopenCmd.Flags().StringVar(&todoReopenReason, "reason", "", "Reason for reopening")

	// todo show flags
	todoShowCmd.Flags().BoolVar(&todoShowJSON, "json", false, "Output as JSON")

	// todo list flags
	todoListCmd.Flags().StringVar(&todoListStatus, "status", "", "Filter by status")
	todoListCmd.Flags().IntVar(&todoListPriority, "priority", -1, "Filter by priority (0-4)")
	todoListCmd.Flags().StringVar(&todoListType, "type", "", "Filter by type")
	todoListCmd.Flags().StringVar(&todoListIDs, "id", "", "Filter by IDs (comma-separated)")
	todoListCmd.Flags().StringVar(&todoListTitle, "title", "", "Filter by title substring")
	todoListCmd.Flags().StringVar(&todoListDescription, "description", "", "Filter by description substring")
	todoListCmd.Flags().BoolVar(&todoListJSON, "json", false, "Output as JSON")

	// todo ready flags
	todoReadyCmd.Flags().IntVar(&todoReadyLimit, "limit", 20, "Maximum number of todos to show")
	todoReadyCmd.Flags().BoolVar(&todoReadyJSON, "json", false, "Output as JSON")

	// todo dep add flags
	todoDepAddCmd.Flags().StringVar(&todoDepAddType, "type", "blocks", "Dependency type (blocks, discovered-from)")
}

// openTodoStore opens the todo store, prompting to create if it doesn't exist.
func openTodoStore() (*todo.Store, error) {
	repoPath, err := getRepoPath()
	if err != nil {
		return nil, err
	}

	return todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
	})
}

func runTodoCreate(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	created, err := store.Create(args[0], todo.CreateOptions{
		Type:         todo.TodoType(todoCreateType),
		Priority:     todoCreatePriority,
		Description:  todoCreateDescription,
		Dependencies: todoCreateDeps,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created todo %s: %s\n", created.ID, created.Title)
	return nil
}

func runTodoUpdate(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	opts := todo.UpdateOptions{}

	// Only set fields that were explicitly provided
	if cmd.Flags().Changed("title") {
		opts.Title = &todoUpdateTitle
	}
	if cmd.Flags().Changed("description") {
		opts.Description = &todoUpdateDescription
	}
	if cmd.Flags().Changed("design") {
		opts.Design = &todoUpdateDesign
	}
	if cmd.Flags().Changed("acceptance-criteria") {
		opts.AcceptanceCriteria = &todoUpdateAcceptanceCriteria
	}
	if cmd.Flags().Changed("notes") {
		opts.Notes = &todoUpdateNotes
	}
	if cmd.Flags().Changed("status") {
		status := todo.Status(todoUpdateStatus)
		opts.Status = &status
	}
	if cmd.Flags().Changed("priority") {
		opts.Priority = &todoUpdatePriority
	}
	if cmd.Flags().Changed("type") {
		typ := todo.TodoType(todoUpdateType)
		opts.Type = &typ
	}

	updated, err := store.Update(args, opts)
	if err != nil {
		return err
	}

	for _, t := range updated {
		fmt.Printf("Updated %s: %s\n", t.ID, t.Title)
	}
	return nil
}

func runTodoClose(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	closed, err := store.Close(args, todoCloseReason)
	if err != nil {
		return err
	}

	for _, t := range closed {
		fmt.Printf("Closed %s: %s\n", t.ID, t.Title)
	}
	return nil
}

func runTodoReopen(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	reopened, err := store.Reopen(args, todoReopenReason)
	if err != nil {
		return err
	}

	for _, t := range reopened {
		fmt.Printf("Reopened %s: %s\n", t.ID, t.Title)
	}
	return nil
}

func runTodoShow(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	todos, err := store.Show(args)
	if err != nil {
		return err
	}

	if todoShowJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(todos)
	}

	for i, t := range todos {
		if i > 0 {
			fmt.Println("---")
		}
		printTodoDetail(t)
	}
	return nil
}

func runTodoList(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	filter := todo.ListFilter{}

	if todoListStatus != "" {
		status := todo.Status(todoListStatus)
		filter.Status = &status
	}
	if cmd.Flags().Changed("priority") && todoListPriority >= 0 {
		filter.Priority = &todoListPriority
	}
	if todoListType != "" {
		typ := todo.TodoType(todoListType)
		filter.Type = &typ
	}
	if todoListIDs != "" {
		filter.IDs = strings.Split(todoListIDs, ",")
	}
	filter.TitleSubstring = todoListTitle
	filter.DescriptionSubstring = todoListDescription

	todos, err := store.List(filter)
	if err != nil {
		return err
	}

	if todoListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(todos)
	}

	printTodoTable(todos)
	return nil
}

func runTodoReady(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	todos, err := store.Ready(todoReadyLimit)
	if err != nil {
		return err
	}

	if todoReadyJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(todos)
	}

	if len(todos) == 0 {
		fmt.Println("No ready todos found.")
		return nil
	}

	printTodoTable(todos)
	return nil
}

func runTodoDepAdd(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	dep, err := store.DepAdd(args[0], args[1], todo.DependencyType(todoDepAddType))
	if err != nil {
		return err
	}

	fmt.Printf("Added dependency: %s %s %s\n", dep.TodoID, dep.Type, dep.DependsOnID)
	return nil
}

func runTodoDepTree(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	tree, err := store.DepTree(args[0])
	if err != nil {
		return err
	}

	printDepTree(tree, "", true)
	return nil
}

// printTodoTable prints todos in a table format.
func printTodoTable(todos []todo.Todo) {
	if len(todos) == 0 {
		fmt.Println("No todos found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPRI\tTYPE\tSTATUS\tTITLE")
	for _, t := range todos {
		title := t.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID,
			priorityShort(t.Priority),
			t.Type,
			t.Status,
			title,
		)
	}
	w.Flush()
}

// printTodoDetail prints detailed information about a todo.
func printTodoDetail(t todo.Todo) {
	fmt.Printf("ID:       %s\n", t.ID)
	fmt.Printf("Title:    %s\n", t.Title)
	fmt.Printf("Type:     %s\n", t.Type)
	fmt.Printf("Status:   %s\n", t.Status)
	fmt.Printf("Priority: %s (%d)\n", todo.PriorityName(t.Priority), t.Priority)
	fmt.Printf("Created:  %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:  %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))

	if t.ClosedAt != nil {
		fmt.Printf("Closed:   %s\n", t.ClosedAt.Format("2006-01-02 15:04:05"))
	}

	if t.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", t.Description)
	}
	if t.Design != "" {
		fmt.Printf("\nDesign:\n%s\n", t.Design)
	}
	if t.AcceptanceCriteria != "" {
		fmt.Printf("\nAcceptance Criteria:\n%s\n", t.AcceptanceCriteria)
	}
	if t.Notes != "" {
		fmt.Printf("\nNotes:\n%s\n", t.Notes)
	}
}

// printDepTree prints a dependency tree with ASCII art.
func printDepTree(node *todo.DepTreeNode, prefix string, isLast bool) {
	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	statusIcon := statusIcon(node.Todo.Status)
	typeStr := ""
	if node.Type != "" {
		typeStr = fmt.Sprintf(" [%s]", node.Type)
	}

	fmt.Printf("%s%s%s %s%s (%s)\n",
		prefix, connector, statusIcon, node.Todo.Title, typeStr, node.Todo.ID)

	// Print children
	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		printDepTree(child, childPrefix, isLastChild)
	}
}

// priorityShort returns a short representation of priority.
func priorityShort(p int) string {
	switch p {
	case 0:
		return "P0"
	case 1:
		return "P1"
	case 2:
		return "P2"
	case 3:
		return "P3"
	case 4:
		return "P4"
	default:
		return "P" + strconv.Itoa(p)
	}
}

// statusIcon returns an icon for the status.
func statusIcon(s todo.Status) string {
	switch s {
	case todo.StatusOpen:
		return "[ ]"
	case todo.StatusInProgress:
		return "[~]"
	case todo.StatusClosed:
		return "[x]"
	case todo.StatusTombstone:
		return "[-]"
	default:
		return "[?]"
	}
}

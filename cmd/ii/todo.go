package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/ui"

	"github.com/amonks/incrementum/internal/editor"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

var todoCmd = &cobra.Command{
	Use:   "todo",
	Short: "Manage todos for the current repository",
}

// todo create
var todoCreateCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new todo",
	Long: `Create a new todo.

By default, opens $EDITOR to edit a TOML representation of the todo
when running interactively. Use --no-edit to skip the editor, or
--edit to force opening the editor even when not interactive.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTodoCreate,
}

var (
	todoCreateType        string
	todoCreatePriority    int
	todoCreateDescription string
	todoCreateDeps        []string
	todoCreateEdit        bool
	todoCreateNoEdit      bool
)

// todo update
var todoUpdateCmd = &cobra.Command{
	Use:   "update <id>...",
	Short: "Update one or more todos",
	Long: `Update one or more todos.

By default, opens $EDITOR to edit a TOML representation of the todo
when running interactively and no update flags are provided (one editor session per ID).
Use --no-edit to skip the editor, or --edit to force opening the editor even when not interactive.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTodoUpdate,
}

var (
	todoUpdateTitle       string
	todoUpdateDescription string
	todoUpdateStatus      string
	todoUpdatePriority    int
	todoUpdateType        string
	todoUpdatePrioritySet bool
	todoUpdateEdit        bool
	todoUpdateNoEdit      bool
)

// todo close
var todoCloseCmd = &cobra.Command{
	Use:   "close <id>...",
	Short: "Close one or more todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoClose,
}

// todo start
var todoStartCmd = &cobra.Command{
	Use:   "start <id>...",
	Short: "Mark one or more todos as in progress",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoStart,
}

// todo finish
var todoFinishCmd = &cobra.Command{
	Use:   "finish <id>...",
	Short: "Mark one or more todos as done",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoFinish,
}

// todo reopen
var todoReopenCmd = &cobra.Command{
	Use:   "reopen <id>...",
	Short: "Reopen one or more closed todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoReopen,
}

// todo delete
var todoDeleteCmd = &cobra.Command{
	Use:   "delete <id>...",
	Short: "Delete one or more todos",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runTodoDelete,
}

var todoDeleteReason string

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
	todoListDesc        string
	todoListJSON        bool
	todoListAll         bool
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
	todoCmd.AddCommand(todoCreateCmd, todoUpdateCmd, todoStartCmd, todoCloseCmd, todoFinishCmd, todoReopenCmd,
		todoDeleteCmd, todoShowCmd, todoListCmd, todoReadyCmd, todoDepCmd)
	todoDepCmd.AddCommand(todoDepAddCmd, todoDepTreeCmd)

	// todo create flags
	todoCreateCmd.Flags().StringVarP(&todoCreateType, "type", "t", "task", "Todo type (task, bug, feature)")
	todoCreateCmd.Flags().IntVarP(&todoCreatePriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	todoCreateCmd.Flags().StringVarP(&todoCreateDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	todoCreateCmd.Flags().StringVar(&todoCreateDescription, "desc", "", "Description (use '-' to read from stdin)")
	todoCreateCmd.Flags().StringArrayVar(&todoCreateDeps, "deps", nil, "Dependencies in format type:id (e.g., blocks:abc123)")
	todoCreateCmd.Flags().BoolVarP(&todoCreateEdit, "edit", "e", false, "Open $EDITOR (default if interactive)")
	todoCreateCmd.Flags().BoolVar(&todoCreateNoEdit, "no-edit", false, "Do not open $EDITOR")

	// todo update flags
	todoUpdateCmd.Flags().StringVar(&todoUpdateTitle, "title", "", "New title")
	todoUpdateCmd.Flags().StringVar(&todoUpdateDescription, "description", "", "New description (use '-' to read from stdin)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateDescription, "desc", "", "New description (use '-' to read from stdin)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateStatus, "status", "", "New status (open, in_progress, closed, done)")
	todoUpdateCmd.Flags().IntVar(&todoUpdatePriority, "priority", 0, "New priority (0-4)")
	todoUpdateCmd.Flags().StringVar(&todoUpdateType, "type", "", "New type (task, bug, feature)")
	todoUpdateCmd.Flags().BoolVarP(&todoUpdateEdit, "edit", "e", false, "Open $EDITOR (default if interactive)")
	todoUpdateCmd.Flags().BoolVar(&todoUpdateNoEdit, "no-edit", false, "Do not open $EDITOR")

	// todo close flags

	// todo finish flags

	// todo reopen flags

	// todo delete flags
	todoDeleteCmd.Flags().StringVar(&todoDeleteReason, "reason", "", "Reason for deletion")

	// todo show flags
	todoShowCmd.Flags().BoolVar(&todoShowJSON, "json", false, "Output as JSON")

	// todo list flags
	todoListCmd.Flags().StringVar(&todoListStatus, "status", "", "Filter by status")
	todoListCmd.Flags().IntVar(&todoListPriority, "priority", -1, "Filter by priority (0-4)")
	todoListCmd.Flags().StringVar(&todoListType, "type", "", "Filter by type")
	todoListCmd.Flags().StringVar(&todoListIDs, "id", "", "Filter by IDs (comma-separated)")
	todoListCmd.Flags().StringVar(&todoListTitle, "title", "", "Filter by title substring")
	todoListCmd.Flags().StringVar(&todoListDesc, "description", "", "Filter by description substring")
	todoListCmd.Flags().StringVar(&todoListDesc, "desc", "", "Filter by description substring")
	todoListCmd.Flags().BoolVar(&todoListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(todoListCmd, &todoListAll)

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

func resolveDescriptionFromStdin(description string, reader io.Reader) (string, error) {
	if description != "-" {
		return description, nil
	}

	input, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read description from stdin: %w", err)
	}

	value := strings.TrimSuffix(string(input), "\n")
	value = strings.TrimSuffix(value, "\r")
	return value, nil
}

func runTodoCreate(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
		desc, err := resolveDescriptionFromStdin(todoCreateDescription, os.Stdin)
		if err != nil {
			return err
		}
		todoCreateDescription = desc
	}

	// Determine whether to open editor:
	// - --edit forces editor
	// - --no-edit skips editor
	// - otherwise, open editor if interactive
	useEditor := todoCreateEdit || (!todoCreateNoEdit && editor.IsInteractive())

	if useEditor {
		// Pre-populate from flags/args if provided
		data := editor.DefaultCreateData()
		if len(args) > 0 {
			data.Title = args[0]
		}
		if cmd.Flags().Changed("type") {
			data.Type = todoCreateType
		}
		if cmd.Flags().Changed("priority") {
			data.Priority = todoCreatePriority
		}
		if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
			data.Description = todoCreateDescription
		}

		parsed, err := editor.EditTodoWithData(data)
		if err != nil {
			return err
		}

		store, err := openTodoStore()
		if err != nil {
			return err
		}
		defer store.Release()

		opts := parsed.ToCreateOptions()
		opts.Dependencies = todoCreateDeps

		created, err := store.Create(parsed.Title, opts)
		if err != nil {
			return err
		}

		prefixLengths, err := todoIDPrefixLengthsForStore(store)
		if err != nil {
			return err
		}
		highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
		fmt.Printf("Created todo %s: %s\n", highlight(created.ID), created.Title)
		return nil
	}

	// Non-editor path: title is required
	if len(args) == 0 {
		return fmt.Errorf("title is required (use --edit to open editor)")
	}

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

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	fmt.Printf("Created todo %s: %s\n", highlight(created.ID), created.Title)
	return nil
}

func runTodoUpdate(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
		desc, err := resolveDescriptionFromStdin(todoUpdateDescription, os.Stdin)
		if err != nil {
			return err
		}
		todoUpdateDescription = desc
	}

	hasFlags := cmd.Flags().Changed("title") ||
		cmd.Flags().Changed("description") ||
		cmd.Flags().Changed("desc") ||
		cmd.Flags().Changed("status") ||
		cmd.Flags().Changed("priority") ||
		cmd.Flags().Changed("type")

	// Determine whether to open editor:
	// - --edit forces editor
	// - --no-edit skips editor
	// - otherwise, open editor only when no update flags and interactive
	useEditor := shouldUseTodoUpdateEditor(hasFlags, todoUpdateEdit, todoUpdateNoEdit, editor.IsInteractive())
	if useEditor {
		updatedItems := make([]todo.Todo, 0, len(args))
		for _, id := range args {
			// Fetch the existing todo
			existing, err := store.Show([]string{id})
			if err != nil {
				return err
			}

			// Pre-populate from existing todo, then override with any flags
			data := editor.DataFromTodo(&existing[0])
			if cmd.Flags().Changed("title") {
				data.Title = todoUpdateTitle
			}
			if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
				data.Description = todoUpdateDescription
			}

			if cmd.Flags().Changed("status") {
				data.Status = todoUpdateStatus
			}
			if cmd.Flags().Changed("priority") {
				data.Priority = todoUpdatePriority
			}
			if cmd.Flags().Changed("type") {
				data.Type = todoUpdateType
			}

			parsed, err := editor.EditTodoWithData(data)
			if err != nil {
				return err
			}

			opts := parsed.ToUpdateOptions()
			updated, err := store.Update([]string{id}, opts)
			if err != nil {
				return err
			}
			updatedItems = append(updatedItems, updated[0])
		}

		prefixLengths, err := todoIDPrefixLengthsForStore(store)
		if err != nil {
			return err
		}
		highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
		for _, item := range updatedItems {
			fmt.Printf("Updated %s: %s\n", highlight(item.ID), item.Title)
		}
		return nil
	}

	// Non-editor path: at least one flag is required
	if !hasFlags {
		return fmt.Errorf("at least one update flag is required (use --edit to open editor)")
	}

	opts := todo.UpdateOptions{}

	if cmd.Flags().Changed("title") {
		opts.Title = &todoUpdateTitle
	}
	if cmd.Flags().Changed("description") || cmd.Flags().Changed("desc") {
		opts.Description = &todoUpdateDescription
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

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for _, item := range updated {
		fmt.Printf("Updated %s: %s\n", highlight(item.ID), item.Title)
	}
	return nil
}

func shouldUseTodoUpdateEditor(hasUpdateFlags bool, editFlag bool, noEditFlag bool, interactive bool) bool {
	if editFlag {
		return true
	}
	if noEditFlag {
		return false
	}
	if hasUpdateFlags {
		return false
	}
	return interactive
}

func runTodoClose(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	closed, err := store.Close(args)
	if err != nil {
		return err
	}

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for _, t := range closed {
		fmt.Printf("Closed %s: %s\n", highlight(t.ID), t.Title)
	}
	return nil
}

func runTodoStart(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	started, err := store.Start(args)
	if err != nil {
		return err
	}

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for _, t := range started {
		fmt.Printf("Started %s: %s\n", highlight(t.ID), t.Title)
	}
	return nil
}

func runTodoFinish(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	finished, err := store.Finish(args)
	if err != nil {
		return err
	}

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for _, t := range finished {
		fmt.Printf("Finished %s: %s\n", highlight(t.ID), t.Title)
	}
	return nil
}

func runTodoReopen(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	reopened, err := store.Reopen(args)
	if err != nil {
		return err
	}

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for _, t := range reopened {
		fmt.Printf("Reopened %s: %s\n", highlight(t.ID), t.Title)
	}
	return nil
}

func runTodoDelete(cmd *cobra.Command, args []string) error {
	store, err := openTodoStore()
	if err != nil {
		return err
	}
	defer store.Release()

	deleted, err := store.Delete(args, todoDeleteReason)
	if err != nil {
		return err
	}

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for _, t := range deleted {
		fmt.Printf("Deleted %s: %s\n", highlight(t.ID), t.Title)
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

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	for i, t := range todos {
		if i > 0 {
			fmt.Println("---")
		}
		printTodoDetail(t, highlight)
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
		filter.IDs = parseIDList(todoListIDs)
	}
	filter.TitleSubstring = todoListTitle
	filter.DescriptionSubstring = todoListDesc

	todos, err := store.List(filter)
	if err != nil {
		return err
	}
	if todoListStatus == "" && !todoListAll {
		filtered := todos[:0]
		for _, item := range todos {
			if item.Status != todo.StatusDone {
				filtered = append(filtered, item)
			}
		}
		todos = filtered
	}

	if todoListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(todos)
	}

	index, err := store.IDIndex()
	if err != nil {
		return err
	}

	printTodoTable(todos, index.PrefixLengths(), time.Now())
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

	index, err := store.IDIndex()
	if err != nil {
		return err
	}

	printTodoTable(todos, index.PrefixLengths(), time.Now())
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

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	fmt.Printf("Added dependency: %s %s %s\n", highlight(dep.TodoID), dep.Type, highlight(dep.DependsOnID))
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

	prefixLengths, err := todoIDPrefixLengthsForStore(store)
	if err != nil {
		return err
	}
	highlight := todoLogHighlighter(prefixLengths, ui.HighlightID)
	printDepTree(tree, "", true, highlight)
	return nil
}

func parseIDList(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})

	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		ids = append(ids, part)
	}
	return ids
}

// printTodoTable prints todos in a table format.
func printTodoTable(todos []todo.Todo, prefixLengths map[string]int, now time.Time) {
	if len(todos) == 0 {
		fmt.Println("No todos found.")
		return
	}

	fmt.Print(formatTodoTable(todos, prefixLengths, ui.HighlightID, now))
}

func formatTodoTable(todos []todo.Todo, prefixLengths map[string]int, highlight func(string, int) string, now time.Time) string {
	rows := make([][]string, 0, len(todos))

	if prefixLengths == nil {
		prefixLengths = todoIDPrefixLengths(todos)
	}

	for _, t := range todos {
		title := truncateTableCell(t.Title)
		prefixLen := prefixLengths[strings.ToLower(t.ID)]
		highlighted := highlight(t.ID, prefixLen)
		createdAge := ui.FormatTimeAgo(t.CreatedAt, now)
		updatedAge := ui.FormatTimeAgo(t.UpdatedAt, now)
		row := []string{
			highlighted,
			priorityShort(t.Priority),
			string(t.Type),
			string(t.Status),
			createdAge,
			updatedAge,
			title,
		}
		rows = append(rows, row)
	}

	return formatTable([]string{"ID", "PRI", "TYPE", "STATUS", "CREATED", "UPDATED", "TITLE"}, rows)
}

func todoIDPrefixLengths(todos []todo.Todo) map[string]int {
	index := todo.NewIDIndex(todos)
	return index.PrefixLengths()
}

func todoIDPrefixLengthsForStore(store *todo.Store) (map[string]int, error) {
	index, err := store.IDIndex()
	if err != nil {
		return nil, err
	}
	return index.PrefixLengths(), nil
}

func todoLogHighlighter(prefixLengths map[string]int, highlight func(string, int) string) func(string) string {
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

// printTodoDetail prints detailed information about a todo.
func printTodoDetail(t todo.Todo, highlight func(string) string) {
	fmt.Printf("ID:       %s\n", highlight(t.ID))
	fmt.Printf("Title:    %s\n", t.Title)
	fmt.Printf("Type:     %s\n", t.Type)
	fmt.Printf("Status:   %s\n", t.Status)
	fmt.Printf("Priority: %s (%d)\n", todo.PriorityName(t.Priority), t.Priority)
	fmt.Printf("Created:  %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:  %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))

	if t.ClosedAt != nil {
		fmt.Printf("Closed:   %s\n", t.ClosedAt.Format("2006-01-02 15:04:05"))
	}

	if t.DeletedAt != nil {
		fmt.Printf("Deleted:  %s\n", t.DeletedAt.Format("2006-01-02 15:04:05"))
	}

	if t.DeleteReason != "" {
		fmt.Printf("Delete Reason: %s\n", t.DeleteReason)
	}

	if t.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", t.Description)
	}
}

// printDepTree prints a dependency tree with ASCII art.
func printDepTree(node *todo.DepTreeNode, prefix string, isLast bool, highlight func(string) string) {
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
		prefix, connector, statusIcon, node.Todo.Title, typeStr, highlight(node.Todo.ID))

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
		printDepTree(child, childPrefix, isLastChild, highlight)
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
	case todo.StatusDone:
		return "[d]"
	case todo.StatusTombstone:
		return "[-]"
	default:
		return "[?]"
	}
}

package todo

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CreateOptions configures a new todo.
type CreateOptions struct {
	// Type is the todo type (task, bug, feature). Defaults to TypeTask.
	Type TodoType

	// Priority is the importance level (0-4). Defaults to PriorityMedium (2).
	Priority int

	// Description provides additional context.
	Description string

	// Dependencies is a list of dependency specifications in the format "type:id".
	// For example: "blocks:abc123" or "discovered-from:def456".
	Dependencies []string
}

// Create creates a new todo with the given title.
func (s *Store) Create(title string, opts CreateOptions) (*Todo, error) {
	// Validate title
	if err := ValidateTitle(title); err != nil {
		return nil, err
	}

	// Apply defaults
	if opts.Type == "" {
		opts.Type = TypeTask
	}
	if !opts.Type.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidType, opts.Type)
	}

	// Note: Priority 0 is valid (critical), so CLI must pass explicit default
	if err := ValidatePriority(opts.Priority); err != nil {
		return nil, err
	}

	// Parse and validate dependencies
	var deps []struct {
		Type DependencyType
		ID   string
	}
	for _, depSpec := range opts.Dependencies {
		parts := strings.SplitN(depSpec, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid dependency format %q: expected 'type:id'", depSpec)
		}
		depType := DependencyType(parts[0])
		if !depType.IsValid() {
			return nil, fmt.Errorf("%w: %q", ErrInvalidDependencyType, parts[0])
		}
		deps = append(deps, struct {
			Type DependencyType
			ID   string
		}{Type: depType, ID: parts[1]})
	}

	now := time.Now()
	todo := Todo{
		ID:          GenerateID(title, now),
		Title:       title,
		Description: opts.Description,
		Status:      StatusOpen,
		Priority:    opts.Priority,
		Type:        opts.Type,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Read existing todos
	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}

	if len(deps) > 0 {
		depIDs := make([]string, 0, len(deps))
		for _, dep := range deps {
			depIDs = append(depIDs, dep.ID)
		}
		resolvedIDs, err := resolveTodoIDsWithTodos(depIDs, todos)
		if err != nil {
			return nil, err
		}
		for i := range deps {
			deps[i].ID = resolvedIDs[i]
		}
	}

	// Add the new todo
	todos = append(todos, todo)

	// Write todos
	if err := s.writeTodos(todos); err != nil {
		return nil, fmt.Errorf("write todos: %w", err)
	}

	// Add dependencies
	if len(deps) > 0 {
		existingDeps, err := s.readDependencies()
		if err != nil {
			return nil, fmt.Errorf("read dependencies: %w", err)
		}

		for _, dep := range deps {
			existingDeps = append(existingDeps, Dependency{
				TodoID:      todo.ID,
				DependsOnID: dep.ID,
				Type:        dep.Type,
				CreatedAt:   now,
			})
		}

		if err := s.writeDependencies(existingDeps); err != nil {
			return nil, fmt.Errorf("write dependencies: %w", err)
		}
	}

	return &todo, nil
}

// UpdateOptions configures fields to update on todos.
// Nil pointers mean "don't update this field".
type UpdateOptions struct {
	Title        *string
	Description  *string
	Status       *Status
	Priority     *int
	Type         *TodoType
	DeletedAt    *time.Time
	DeleteReason *string
}

// Update updates one or more todos with the given options.
// Returns the updated todos.
func (s *Store) Update(ids []string, opts UpdateOptions) ([]Todo, error) {
	resolvedIDs, err := s.resolveTodoIDs(ids)
	if err != nil {
		return nil, err
	}

	// Validate options
	if opts.Title != nil {
		if err := ValidateTitle(*opts.Title); err != nil {
			return nil, err
		}
	}
	if opts.Status != nil && !opts.Status.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, *opts.Status)
	}
	if opts.Priority != nil {
		if err := ValidatePriority(*opts.Priority); err != nil {
			return nil, err
		}
	}
	if opts.Type != nil && !opts.Type.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidType, *opts.Type)
	}

	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}

	// Build a set of IDs to update
	idSet := make(map[string]bool)
	for _, id := range resolvedIDs {
		idSet[id] = true
	}

	now := time.Now()
	var updated []Todo

	for i := range todos {
		if !idSet[todos[i].ID] {
			continue
		}
		delete(idSet, todos[i].ID)

		// Apply updates
		if opts.Title != nil {
			todos[i].Title = *opts.Title
		}
		if opts.Description != nil {
			todos[i].Description = *opts.Description
		}
		if opts.Status != nil {
			todos[i].Status = *opts.Status
			switch *opts.Status {
			case StatusClosed, StatusDone:
				todos[i].ClosedAt = &now
				todos[i].DeletedAt = nil
				todos[i].DeleteReason = ""
			case StatusTombstone:
				todos[i].ClosedAt = nil
				if opts.DeletedAt == nil && todos[i].DeletedAt == nil {
					todos[i].DeletedAt = &now
				}
			case StatusOpen, StatusInProgress:
				todos[i].ClosedAt = nil
				todos[i].DeletedAt = nil
				todos[i].DeleteReason = ""
			}
		}
		if opts.Priority != nil {
			todos[i].Priority = *opts.Priority
		}
		if opts.Type != nil {
			todos[i].Type = *opts.Type
		}
		if opts.DeletedAt != nil {
			todos[i].DeletedAt = opts.DeletedAt
		}
		if opts.DeleteReason != nil {
			todos[i].DeleteReason = *opts.DeleteReason
		}
		if opts.DeletedAt != nil && opts.DeleteReason == nil {
			todos[i].DeleteReason = ""
		}
		todos[i].UpdatedAt = now

		if err := ValidateTodo(&todos[i]); err != nil {
			return nil, fmt.Errorf("validate todo %s: %w", todos[i].ID, err)
		}

		updated = append(updated, todos[i])
	}

	// Check for unfound IDs
	if len(idSet) > 0 {
		var missing []string
		for id := range idSet {
			missing = append(missing, id)
		}
		return nil, fmt.Errorf("todos not found: %s", strings.Join(missing, ", "))
	}

	if err := s.writeTodos(todos); err != nil {
		return nil, fmt.Errorf("write todos: %w", err)
	}

	return updated, nil
}

// Close closes one or more todos.
func (s *Store) Close(ids []string) ([]Todo, error) {
	status := StatusClosed
	opts := UpdateOptions{
		Status: &status,
	}
	return s.Update(ids, opts)
}

// Finish marks one or more todos as done.
func (s *Store) Finish(ids []string) ([]Todo, error) {
	status := StatusDone
	opts := UpdateOptions{
		Status: &status,
	}
	return s.Update(ids, opts)
}

// Reopen reopens one or more closed todos.
func (s *Store) Reopen(ids []string) ([]Todo, error) {
	status := StatusOpen
	opts := UpdateOptions{
		Status: &status,
	}
	return s.Update(ids, opts)
}

// Start marks one or more todos as in progress.
func (s *Store) Start(ids []string) ([]Todo, error) {
	status := StatusInProgress
	opts := UpdateOptions{
		Status: &status,
	}
	return s.Update(ids, opts)
}

// Delete tombstones one or more todos with an optional reason.
func (s *Store) Delete(ids []string, reason string) ([]Todo, error) {
	status := StatusTombstone
	now := time.Now()
	opts := UpdateOptions{
		Status:    &status,
		DeletedAt: &now,
	}
	if reason != "" {
		opts.DeleteReason = &reason
	}
	return s.Update(ids, opts)
}

// Show returns the full details of one or more todos.
func (s *Store) Show(ids []string) ([]Todo, error) {
	resolvedIDs, err := s.resolveTodoIDs(ids)
	if err != nil {
		return nil, err
	}

	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}

	idSet := make(map[string]bool)
	for _, id := range resolvedIDs {
		idSet[id] = true
	}

	var result []Todo
	for _, todo := range todos {
		if idSet[todo.ID] {
			result = append(result, todo)
			delete(idSet, todo.ID)
		}
	}

	if len(idSet) > 0 {
		var missing []string
		for id := range idSet {
			missing = append(missing, id)
		}
		return nil, fmt.Errorf("todos not found: %s", strings.Join(missing, ", "))
	}

	return result, nil
}

// ListFilter configures which todos to return.
type ListFilter struct {
	// Status filters by exact status match.
	Status *Status

	// Priority filters by exact priority match.
	Priority *int

	// Type filters by exact type match.
	Type *TodoType

	// IDs filters to specific IDs.
	IDs []string

	// TitleSubstring filters to todos with this substring in the title.
	TitleSubstring string

	// DescriptionSubstring filters to todos with this substring in the description.
	DescriptionSubstring string

	// IncludeTombstones includes soft-deleted todos. Default is false.
	IncludeTombstones bool
}

// List returns todos matching the filter.
func (s *Store) List(filter ListFilter) ([]Todo, error) {
	if filter.Status != nil && !filter.Status.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, *filter.Status)
	}
	if filter.Type != nil && !filter.Type.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidType, *filter.Type)
	}
	if filter.Priority != nil {
		if err := ValidatePriority(*filter.Priority); err != nil {
			return nil, err
		}
	}

	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}

	// Build ID set if filtering by IDs
	var idSet map[string]bool
	if len(filter.IDs) > 0 {
		resolvedIDs, err := resolveTodoIDsWithTodos(filter.IDs, todos)
		if err != nil {
			return nil, err
		}
		idSet = make(map[string]bool)
		for _, id := range resolvedIDs {
			idSet[id] = true
		}
	}

	var result []Todo
	for _, todo := range todos {
		// Filter tombstones unless explicitly included
		if todo.Status == StatusTombstone && !filter.IncludeTombstones {
			continue
		}

		// Apply filters
		if filter.Status != nil && todo.Status != *filter.Status {
			continue
		}
		if filter.Priority != nil && todo.Priority != *filter.Priority {
			continue
		}
		if filter.Type != nil && todo.Type != *filter.Type {
			continue
		}
		if idSet != nil && !idSet[todo.ID] {
			continue
		}
		if filter.TitleSubstring != "" && !strings.Contains(strings.ToLower(todo.Title), strings.ToLower(filter.TitleSubstring)) {
			continue
		}
		if filter.DescriptionSubstring != "" && !strings.Contains(strings.ToLower(todo.Description), strings.ToLower(filter.DescriptionSubstring)) {
			continue
		}

		result = append(result, todo)
	}

	return result, nil
}

// Ready returns open todos with no unresolved blockers, sorted by priority.
func (s *Store) Ready(limit int) ([]Todo, error) {
	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}

	deps, err := s.readDependencies()
	if err != nil {
		return nil, fmt.Errorf("read dependencies: %w", err)
	}

	// Build map of todo ID -> todo for quick lookup
	todoMap := make(map[string]*Todo)
	for i := range todos {
		todoMap[todos[i].ID] = &todos[i]
	}

	// Build map of todo ID -> blocking todo IDs
	blockers := make(map[string][]string)
	for _, dep := range deps {
		if dep.Type == DepBlocks {
			blockers[dep.TodoID] = append(blockers[dep.TodoID], dep.DependsOnID)
		}
	}

	// Filter to open todos with no open blockers
	var ready []Todo
	for _, todo := range todos {
		if todo.Status != StatusOpen {
			continue
		}

		hasOpenBlocker := false
		for _, blockerID := range blockers[todo.ID] {
			blocker, ok := todoMap[blockerID]
			if ok && blocker.Status != StatusClosed && blocker.Status != StatusDone && blocker.Status != StatusTombstone {
				hasOpenBlocker = true
				break
			}
		}

		if !hasOpenBlocker {
			ready = append(ready, todo)
		}
	}

	// Sort by priority (0 = highest priority)
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].Priority != ready[j].Priority {
			return ready[i].Priority < ready[j].Priority
		}
		if TodoTypeRank(ready[i].Type) != TodoTypeRank(ready[j].Type) {
			return TodoTypeRank(ready[i].Type) < TodoTypeRank(ready[j].Type)
		}
		// Secondary sort by creation time (oldest first)
		return ready[i].CreatedAt.Before(ready[j].CreatedAt)
	})

	// Apply limit
	if limit > 0 && len(ready) > limit {
		ready = ready[:limit]
	}

	return ready, nil
}

// DepAdd adds a dependency between two todos.
func (s *Store) DepAdd(todoID, dependsOnID string, depType DependencyType) (*Dependency, error) {
	// Validate dependency type
	if !depType.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidDependencyType, depType)
	}

	resolvedIDs, err := s.resolveTodoIDs([]string{todoID, dependsOnID})
	if err != nil {
		return nil, err
	}
	todoID = resolvedIDs[0]
	dependsOnID = resolvedIDs[1]

	// Check for self-dependency
	if todoID == dependsOnID {
		return nil, ErrSelfDependency
	}

	// Read existing dependencies
	deps, err := s.readDependencies()
	if err != nil {
		return nil, fmt.Errorf("read dependencies: %w", err)
	}

	// Check for duplicate
	for _, d := range deps {
		if d.TodoID == todoID && d.DependsOnID == dependsOnID {
			return nil, ErrDuplicateDependency
		}
	}

	// Add new dependency
	dep := Dependency{
		TodoID:      todoID,
		DependsOnID: dependsOnID,
		Type:        depType,
		CreatedAt:   time.Now(),
	}
	deps = append(deps, dep)

	if err := s.writeDependencies(deps); err != nil {
		return nil, fmt.Errorf("write dependencies: %w", err)
	}

	return &dep, nil
}

// DepTree returns the dependency tree for a todo.
func (s *Store) DepTree(id string) (*DepTreeNode, error) {
	resolvedIDs, err := s.resolveTodoIDs([]string{id})
	if err != nil {
		return nil, err
	}
	if len(resolvedIDs) == 0 {
		return nil, ErrTodoNotFound
	}
	id = resolvedIDs[0]

	todos, err := s.readTodos()
	if err != nil {
		return nil, fmt.Errorf("read todos: %w", err)
	}

	deps, err := s.readDependencies()
	if err != nil {
		return nil, fmt.Errorf("read dependencies: %w", err)
	}

	// Build lookup maps
	todoMap := make(map[string]*Todo)
	for i := range todos {
		todoMap[todos[i].ID] = &todos[i]
	}

	// Group dependencies by todo ID
	depsByTodo := make(map[string][]Dependency)
	for _, d := range deps {
		depsByTodo[d.TodoID] = append(depsByTodo[d.TodoID], d)
	}

	// Find the root todo
	rootTodo, ok := todoMap[id]
	if !ok {
		return nil, ErrTodoNotFound
	}

	// Build tree recursively
	visited := make(map[string]bool)
	return buildDepTree(rootTodo, "", depsByTodo, todoMap, visited), nil
}

// buildDepTree recursively builds a dependency tree node.
func buildDepTree(todo *Todo, depType DependencyType, depsByTodo map[string][]Dependency, todoMap map[string]*Todo, visited map[string]bool) *DepTreeNode {
	if visited[todo.ID] {
		// Avoid cycles
		return &DepTreeNode{Todo: todo, Type: depType}
	}
	visited[todo.ID] = true

	node := &DepTreeNode{
		Todo: todo,
		Type: depType,
	}

	for _, dep := range depsByTodo[todo.ID] {
		childTodo, ok := todoMap[dep.DependsOnID]
		if !ok {
			continue
		}
		childNode := buildDepTree(childTodo, dep.Type, depsByTodo, todoMap, visited)
		node.Children = append(node.Children, childNode)
	}

	return node
}

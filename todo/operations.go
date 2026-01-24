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

	// Priority is the importance level (0-4). Defaults to PriorityMedium (2) when nil.
	Priority *int

	// Description provides additional context.
	Description string

	// Dependencies is a list of dependency IDs.
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
	normalizedType, err := normalizeTodoTypeInput(opts.Type)
	if err != nil {
		return nil, err
	}
	opts.Type = normalizedType

	priority := opts.Priority
	if priority == nil {
		defaultPriority := PriorityMedium
		priority = &defaultPriority
	}
	// Note: Priority 0 is valid (critical), so nil indicates default.
	if err := ValidatePriority(*priority); err != nil {
		return nil, err
	}

	// Parse and validate dependencies
	var deps []struct {
		ID string
	}
	for _, depID := range opts.Dependencies {
		if strings.Contains(depID, ":") {
			return nil, fmt.Errorf("invalid dependency format %q: expected '<id>'", depID)
		}
		deps = append(deps, struct {
			ID string
		}{ID: depID})
	}

	now := time.Now()
	todo := Todo{
		ID:          GenerateID(title, now),
		Title:       title,
		Description: opts.Description,
		Status:      StatusOpen,
		Priority:    *priority,
		Type:        opts.Type,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Read existing todos
	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
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
		seen := make(map[string]struct{})
		for _, dep := range deps {
			if dep.ID == todo.ID {
				return nil, ErrSelfDependency
			}
			if _, ok := seen[dep.ID]; ok {
				return nil, ErrDuplicateDependency
			}
			seen[dep.ID] = struct{}{}
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
		existingDeps, err := s.readDependenciesWithContext()
		if err != nil {
			return nil, err
		}

		for _, dep := range deps {
			existingDeps = append(existingDeps, Dependency{
				TodoID:      todo.ID,
				DependsOnID: dep.ID,
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
	if opts.Status != nil {
		normalized, err := normalizeStatusInput(*opts.Status)
		if err != nil {
			return nil, err
		}
		opts.Status = &normalized
	}
	if opts.Priority != nil {
		if err := ValidatePriority(*opts.Priority); err != nil {
			return nil, err
		}
	}
	if opts.Type != nil {
		normalized, err := normalizeTodoTypeInput(*opts.Type)
		if err != nil {
			return nil, err
		}
		opts.Type = &normalized
	}

	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
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
			newStatus := *opts.Status
			if newStatus != todos[i].Status {
				previousStatus := todos[i].Status
				todos[i].Status = newStatus
				if newStatus != StatusDone {
					todos[i].StartedAt = nil
					todos[i].CompletedAt = nil
				}
				switch newStatus {
				case StatusClosed, StatusDone:
					todos[i].ClosedAt = &now
					todos[i].DeletedAt = nil
					todos[i].DeleteReason = ""
					if newStatus == StatusDone {
						if previousStatus == StatusInProgress {
							todos[i].CompletedAt = &now
						} else {
							todos[i].CompletedAt = nil
						}
					}
				case StatusTombstone:
					todos[i].ClosedAt = nil
					if opts.DeletedAt == nil && todos[i].DeletedAt == nil {
						todos[i].DeletedAt = &now
					}
				case StatusOpen, StatusInProgress:
					todos[i].ClosedAt = nil
					todos[i].DeletedAt = nil
					todos[i].DeleteReason = ""
					if newStatus == StatusInProgress && previousStatus != StatusInProgress {
						todos[i].StartedAt = &now
						todos[i].CompletedAt = nil
					}
				}
			} else {
				todos[i].Status = newStatus
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
		return nil, missingTodoIDsError(missing)
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

	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
	}

	todoByID := todoMapByID(todos)

	var result []Todo
	seen := make(map[string]bool)
	var missing []string
	for _, id := range resolvedIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		todo, ok := todoByID[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		result = append(result, *todo)
	}

	if err := missingTodoIDsError(missing); err != nil {
		return nil, err
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
	if filter.Status != nil {
		normalized, err := normalizeStatusInput(*filter.Status)
		if err != nil {
			return nil, err
		}
		filter.Status = &normalized
	}
	if filter.Type != nil {
		normalized, err := normalizeTodoTypeInput(*filter.Type)
		if err != nil {
			return nil, err
		}
		filter.Type = &normalized
	}
	if filter.Priority != nil {
		if err := ValidatePriority(*filter.Priority); err != nil {
			return nil, err
		}
	}

	titleQuery := strings.ToLower(filter.TitleSubstring)
	descriptionQuery := strings.ToLower(filter.DescriptionSubstring)

	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
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

	includeTombstones := filter.IncludeTombstones
	if filter.Status != nil && *filter.Status == StatusTombstone {
		includeTombstones = true
	}

	var result []Todo
	for _, todo := range todos {
		// Filter tombstones unless explicitly included
		if todo.Status == StatusTombstone && !includeTombstones {
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
		if titleQuery != "" && !strings.Contains(strings.ToLower(todo.Title), titleQuery) {
			continue
		}
		if descriptionQuery != "" && !strings.Contains(strings.ToLower(todo.Description), descriptionQuery) {
			continue
		}

		result = append(result, todo)
	}

	return result, nil
}

func todoMapByID(todos []Todo) map[string]*Todo {
	todoMap := make(map[string]*Todo, len(todos))
	for i := range todos {
		todoMap[todos[i].ID] = &todos[i]
	}
	return todoMap
}

func missingTodoIDsError(missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("todos not found: %s", strings.Join(missing, ", "))
}

// Ready returns open todos with no unresolved blockers, sorted by priority.
func (s *Store) Ready(limit int) ([]Todo, error) {
	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
	}

	deps, err := s.readDependenciesWithContext()
	if err != nil {
		return nil, err
	}

	// Build map of todo ID -> todo for quick lookup
	todoMap := todoMapByID(todos)

	// Build map of todo ID -> blocking todo IDs
	blockers := make(map[string][]string)
	for _, dep := range deps {
		blockers[dep.TodoID] = append(blockers[dep.TodoID], dep.DependsOnID)
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
			if ok && !blocker.Status.IsResolved() {
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
func (s *Store) DepAdd(todoID, dependsOnID string) (*Dependency, error) {
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
	deps, err := s.readDependenciesWithContext()
	if err != nil {
		return nil, err
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

	todos, err := s.readTodosWithContext()
	if err != nil {
		return nil, err
	}

	deps, err := s.readDependenciesWithContext()
	if err != nil {
		return nil, err
	}

	// Build lookup maps
	todoMap := todoMapByID(todos)

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
	path := make(map[string]bool)
	return buildDepTree(rootTodo, depsByTodo, todoMap, path), nil
}

// buildDepTree recursively builds a dependency tree node.

func buildDepTree(todo *Todo, depsByTodo map[string][]Dependency, todoMap map[string]*Todo, path map[string]bool) *DepTreeNode {
	if path[todo.ID] {
		// Avoid cycles
		return &DepTreeNode{Todo: todo}
	}
	path[todo.ID] = true
	defer delete(path, todo.ID)

	node := &DepTreeNode{
		Todo: todo,
	}

	for _, dep := range depsByTodo[todo.ID] {
		childTodo, ok := todoMap[dep.DependsOnID]
		if !ok {
			continue
		}
		childNode := buildDepTree(childTodo, depsByTodo, todoMap, path)
		node.Children = append(node.Children, childNode)
	}

	return node
}

package todo

import (
	"errors"
	"testing"
	"time"
)

func TestStore_Create(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create a basic todo (CLI will pass defaults)
	todo, err := store.Create("Fix login bug", CreateOptions{
		Priority: PriorityMedium, // CLI provides default
	})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	if todo.Title != "Fix login bug" {
		t.Errorf("expected title 'Fix login bug', got %q", todo.Title)
	}
	if todo.Status != StatusOpen {
		t.Errorf("expected status 'open', got %q", todo.Status)
	}
	if todo.Type != TypeTask {
		t.Errorf("expected type 'task', got %q", todo.Type)
	}
	if todo.Priority != PriorityMedium {
		t.Errorf("expected priority 2, got %d", todo.Priority)
	}
	if len(todo.ID) != 8 {
		t.Errorf("expected 8-char ID, got %q", todo.ID)
	}
}

func TestStore_Create_WithOptions(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	todo, err := store.Create("Add dark mode", CreateOptions{
		Type:        TypeFeature,
		Priority:    PriorityHigh,
		Description: "Users want dark mode",
	})

	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	if todo.Type != TypeFeature {
		t.Errorf("expected type 'feature', got %q", todo.Type)
	}
	if todo.Priority != PriorityHigh {
		t.Errorf("expected priority 1, got %d", todo.Priority)
	}
	if todo.Description != "Users want dark mode" {
		t.Errorf("expected description 'Users want dark mode', got %q", todo.Description)
	}
}

func TestStore_Create_WithDependency(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create parent todo first
	parent, err := store.Create("Parent task", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child with dependency
	child, err := store.Create("Child task", CreateOptions{
		Dependencies: []string{"discovered-from:" + parent.ID},
	})
	if err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Verify dependency was created
	deps, err := store.readDependencies()
	if err != nil {
		t.Fatalf("failed to read dependencies: %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].TodoID != child.ID {
		t.Errorf("expected TodoID %q, got %q", child.ID, deps[0].TodoID)
	}
	if deps[0].DependsOnID != parent.ID {
		t.Errorf("expected DependsOnID %q, got %q", parent.ID, deps[0].DependsOnID)
	}
	if deps[0].Type != DepDiscoveredFrom {
		t.Errorf("expected type 'discovered-from', got %q", deps[0].Type)
	}
}

func TestStore_Create_Validation(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Empty title
	_, err = store.Create("", CreateOptions{})
	if !errors.Is(err, ErrEmptyTitle) {
		t.Errorf("expected ErrEmptyTitle, got %v", err)
	}

	// Invalid priority
	_, err = store.Create("Test", CreateOptions{Priority: 10})
	if !errors.Is(err, ErrInvalidPriority) {
		t.Errorf("expected ErrInvalidPriority, got %v", err)
	}

	// Invalid type
	_, err = store.Create("Test", CreateOptions{Type: TodoType("invalid")})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestStore_Update(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create a todo
	todo, err := store.Create("Original title", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	// Update the title
	newTitle := "Updated title"
	updated, err := store.Update([]string{todo.ID}, UpdateOptions{
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	if len(updated) != 1 {
		t.Fatalf("expected 1 updated todo, got %d", len(updated))
	}
	if updated[0].Title != "Updated title" {
		t.Errorf("expected title 'Updated title', got %q", updated[0].Title)
	}

	// Verify by reading again
	todos, err := store.Show([]string{todo.ID})
	if err != nil {
		t.Fatalf("failed to show: %v", err)
	}
	if todos[0].Title != "Updated title" {
		t.Errorf("title was not persisted")
	}
}

func TestStore_Update_TombstoneSetsDeletedAt(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	created, err := store.Create("Old todo", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	status := StatusTombstone
	updated, err := store.Update([]string{created.ID}, UpdateOptions{Status: &status})
	if err != nil {
		t.Fatalf("failed to tombstone todo: %v", err)
	}

	if updated[0].Status != StatusTombstone {
		t.Errorf("expected status 'tombstone', got %q", updated[0].Status)
	}
	if updated[0].DeletedAt == nil {
		t.Error("expected DeletedAt to be set")
	}
	if updated[0].ClosedAt != nil {
		t.Error("expected ClosedAt to be nil")
	}
}

func TestStore_Update_DeleteReasonKeepsDeletedAt(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	created, err := store.Create("Old todo", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	_, err = store.Delete([]string{created.ID}, "")
	if err != nil {
		t.Fatalf("failed to delete todo: %v", err)
	}

	reason := "Superseded"
	updated, err := store.Update([]string{created.ID}, UpdateOptions{DeleteReason: &reason})
	if err != nil {
		t.Fatalf("failed to update delete reason: %v", err)
	}

	if updated[0].DeleteReason != reason {
		t.Errorf("expected delete reason %q, got %q", reason, updated[0].DeleteReason)
	}
	if updated[0].DeletedAt == nil {
		t.Error("expected DeletedAt to remain set")
	}
}

func TestStore_Update_DeletedAtRequiresTombstone(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	created, err := store.Create("Open todo", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	deletedAt := time.Now()
	_, err = store.Update([]string{created.ID}, UpdateOptions{DeletedAt: &deletedAt})
	if !errors.Is(err, ErrDeletedAtRequiresTombstoneStatus) {
		t.Errorf("expected ErrDeletedAtRequiresTombstoneStatus, got %v", err)
	}
}

func TestStore_Update_Multiple(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create multiple todos
	todo1, _ := store.Create("Todo 1", CreateOptions{Priority: PriorityLow})
	todo2, _ := store.Create("Todo 2", CreateOptions{Priority: PriorityLow})

	// Update both
	newPriority := PriorityCritical
	updated, err := store.Update([]string{todo1.ID, todo2.ID}, UpdateOptions{
		Priority: &newPriority,
	})
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	if len(updated) != 2 {
		t.Fatalf("expected 2 updated todos, got %d", len(updated))
	}
	for _, u := range updated {
		if u.Priority != PriorityCritical {
			t.Errorf("expected priority 0, got %d for %q", u.Priority, u.ID)
		}
	}
}

func TestStore_Update_NotFound(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	newTitle := "test"
	_, err = store.Update([]string{"nonexistent"}, UpdateOptions{
		Title: &newTitle,
	})
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestStore_Close(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create a todo
	todo, _ := store.Create("Test todo", CreateOptions{})

	// Close it
	closed, err := store.Close([]string{todo.ID}, "")
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if len(closed) != 1 {
		t.Fatalf("expected 1 closed todo, got %d", len(closed))
	}
	if closed[0].Status != StatusClosed {
		t.Errorf("expected status 'closed', got %q", closed[0].Status)
	}
	if closed[0].ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}

	// Mark as done
	doneStatus := StatusDone
	done, err := store.Update([]string{todo.ID}, UpdateOptions{Status: &doneStatus})
	if err != nil {
		t.Fatalf("failed to mark done: %v", err)
	}
	if done[0].Status != StatusDone {
		t.Errorf("expected status 'done', got %q", done[0].Status)
	}
	if done[0].ClosedAt == nil {
		t.Error("expected ClosedAt to be set for done")
	}
}

func TestStore_Finish(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	created, _ := store.Create("Finish the docs", CreateOptions{})

	finished, err := store.Finish([]string{created.ID}, "")
	if err != nil {
		t.Fatalf("failed to finish: %v", err)
	}

	if len(finished) != 1 {
		t.Fatalf("expected 1 finished todo, got %d", len(finished))
	}
	if finished[0].Status != StatusDone {
		t.Errorf("expected status 'done', got %q", finished[0].Status)
	}
	if finished[0].ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
}

func TestStore_Reopen(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create and close a todo
	todo, _ := store.Create("Test todo", CreateOptions{})
	store.Close([]string{todo.ID}, "")

	// Reopen it
	reopened, err := store.Reopen([]string{todo.ID}, "")
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}

	if len(reopened) != 1 {
		t.Fatalf("expected 1 reopened todo, got %d", len(reopened))
	}
	if reopened[0].Status != StatusOpen {
		t.Errorf("expected status 'open', got %q", reopened[0].Status)
	}
	if reopened[0].ClosedAt != nil {
		t.Error("expected ClosedAt to be nil")
	}
}

func TestStore_Start(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	created, _ := store.Create("Start the work", CreateOptions{})
	_, err = store.Close([]string{created.ID}, "")
	if err != nil {
		t.Fatalf("failed to close todo: %v", err)
	}

	started, err := store.Start([]string{created.ID})
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if len(started) != 1 {
		t.Fatalf("expected 1 started todo, got %d", len(started))
	}
	if started[0].Status != StatusInProgress {
		t.Errorf("expected status 'in_progress', got %q", started[0].Status)
	}
	if started[0].ClosedAt != nil {
		t.Error("expected ClosedAt to be nil")
	}
}

func TestStore_Delete(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	todo, err := store.Create("Remove old endpoint", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	deleted, err := store.Delete([]string{todo.ID}, "No longer needed")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if len(deleted) != 1 {
		t.Fatalf("expected 1 deleted todo, got %d", len(deleted))
	}
	if deleted[0].Status != StatusTombstone {
		t.Errorf("expected status 'tombstone', got %q", deleted[0].Status)
	}
	if deleted[0].DeletedAt == nil {
		t.Error("expected DeletedAt to be set")
	}
	if deleted[0].DeleteReason != "No longer needed" {
		t.Errorf("expected delete reason to be set, got %q", deleted[0].DeleteReason)
	}
	if deleted[0].ClosedAt != nil {
		t.Error("expected ClosedAt to be nil")
	}

	shown, err := store.Show([]string{todo.ID})
	if err != nil {
		t.Fatalf("failed to show: %v", err)
	}
	if shown[0].Status != StatusTombstone {
		t.Errorf("expected tombstone status in show, got %q", shown[0].Status)
	}
}

func TestStore_Delete_ClearsClosedAt(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	created, err := store.Create("Old task", CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	_, err = store.Close([]string{created.ID}, "")
	if err != nil {
		t.Fatalf("failed to close todo: %v", err)
	}

	deleted, err := store.Delete([]string{created.ID}, "Superseded")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
	if deleted[0].ClosedAt != nil {
		t.Error("expected ClosedAt to be cleared for tombstone")
	}
}

func TestStore_Delete_NotFound(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	_, err = store.Delete([]string{"nonexistent"}, "No longer needed")
	if err == nil {
		t.Fatal("expected error for nonexistent todo")
	}
	if !errors.Is(err, ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestStore_Delete_ListExcludesTombstones(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	first, _ := store.Create("First", CreateOptions{})
	store.Create("Second", CreateOptions{})

	_, err = store.Delete([]string{first.ID}, "No longer needed")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	listed, err := store.List(ListFilter{})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 todo after delete, got %d", len(listed))
	}
	if listed[0].Title != "Second" {
		t.Errorf("expected remaining todo 'Second', got %q", listed[0].Title)
	}

	listed, err = store.List(ListFilter{IncludeTombstones: true})
	if err != nil {
		t.Fatalf("failed to list with tombstones: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 todos including tombstone, got %d", len(listed))
	}
}

func TestStore_Show(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create todos
	todo1, _ := store.Create("Todo 1", CreateOptions{})
	todo2, _ := store.Create("Todo 2", CreateOptions{})

	// Show both
	shown, err := store.Show([]string{todo1.ID, todo2.ID})
	if err != nil {
		t.Fatalf("failed to show: %v", err)
	}

	if len(shown) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(shown))
	}
}

func TestStore_List(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create various todos
	store.Create("Bug 1", CreateOptions{Type: TypeBug, Priority: PriorityHigh})
	store.Create("Feature 1", CreateOptions{Type: TypeFeature, Priority: PriorityLow})
	store.Create("Task 1", CreateOptions{Type: TypeTask, Priority: PriorityMedium})

	// List all
	all, err := store.List(ListFilter{})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 todos, got %d", len(all))
	}

	// Filter by type
	bugType := TypeBug
	bugs, err := store.List(ListFilter{Type: &bugType})
	if err != nil {
		t.Fatalf("failed to list bugs: %v", err)
	}
	if len(bugs) != 1 {
		t.Errorf("expected 1 bug, got %d", len(bugs))
	}

	// Filter by priority
	highPriority := PriorityHigh
	high, err := store.List(ListFilter{Priority: &highPriority})
	if err != nil {
		t.Fatalf("failed to list high priority: %v", err)
	}
	if len(high) != 1 {
		t.Errorf("expected 1 high priority, got %d", len(high))
	}
}

func TestStore_List_InvalidFilters(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	if _, err := store.Create("Task 1", CreateOptions{}); err != nil {
		t.Fatalf("failed to create todo: %v", err)
	}

	invalidStatus := Status("maybe")
	if _, err := store.List(ListFilter{Status: &invalidStatus}); err == nil || !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected invalid status error, got %v", err)
	}

	invalidType := TodoType("oops")
	if _, err := store.List(ListFilter{Type: &invalidType}); err == nil || !errors.Is(err, ErrInvalidType) {
		t.Fatalf("expected invalid type error, got %v", err)
	}

	invalidPriority := 99
	if _, err := store.List(ListFilter{Priority: &invalidPriority}); err == nil || !errors.Is(err, ErrInvalidPriority) {
		t.Fatalf("expected invalid priority error, got %v", err)
	}
}

func TestStore_List_IDPrefix(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	first, _ := store.Create("Task one", CreateOptions{})
	store.Create("Task two", CreateOptions{})

	prefix := first.ID[:6]
	listed, err := store.List(ListFilter{IDs: []string{prefix}})
	if err != nil {
		t.Fatalf("failed to list by ID prefix: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(listed))
	}
	if listed[0].ID != first.ID {
		t.Fatalf("expected todo %s, got %s", first.ID, listed[0].ID)
	}
}

func TestStore_List_TitleSubstring(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	store.Create("Fix authentication bug", CreateOptions{})
	store.Create("Add login feature", CreateOptions{})
	store.Create("Update auth flow", CreateOptions{})

	// Search for "auth" (case insensitive)
	found, err := store.List(ListFilter{TitleSubstring: "auth"})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 todos matching 'auth', got %d", len(found))
	}
}

func TestStore_Ready(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create todos with different priorities
	todo1, _ := store.Create("Low priority", CreateOptions{Priority: PriorityLow})
	todo2, _ := store.Create("High priority", CreateOptions{Priority: PriorityHigh})
	todo3, _ := store.Create("Critical", CreateOptions{Priority: PriorityCritical})

	// All should be ready (no blockers)
	ready, err := store.Ready(10)
	if err != nil {
		t.Fatalf("failed to get ready: %v", err)
	}
	if len(ready) != 3 {
		t.Fatalf("expected 3 ready todos, got %d", len(ready))
	}

	// Should be sorted by priority (critical first)
	if ready[0].ID != todo3.ID {
		t.Errorf("expected critical todo first, got %q", ready[0].Title)
	}
	if ready[1].ID != todo2.ID {
		t.Errorf("expected high priority second, got %q", ready[1].Title)
	}
	if ready[2].ID != todo1.ID {
		t.Errorf("expected low priority third, got %q", ready[2].Title)
	}
}

func TestStore_Ready_TypePriority(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	feature, _ := store.Create("Feature", CreateOptions{Type: TypeFeature, Priority: PriorityMedium})
	task, _ := store.Create("Task", CreateOptions{Type: TypeTask, Priority: PriorityMedium})
	bug, _ := store.Create("Bug", CreateOptions{Type: TypeBug, Priority: PriorityMedium})

	ready, err := store.Ready(10)
	if err != nil {
		t.Fatalf("failed to get ready: %v", err)
	}
	if len(ready) != 3 {
		t.Fatalf("expected 3 ready todos, got %d", len(ready))
	}

	if ready[0].ID != bug.ID {
		t.Errorf("expected bug todo first, got %q", ready[0].Title)
	}
	if ready[1].ID != task.ID {
		t.Errorf("expected task todo second, got %q", ready[1].Title)
	}
	if ready[2].ID != feature.ID {
		t.Errorf("expected feature todo third, got %q", ready[2].Title)
	}
}

func TestStore_Ready_WithBlockers(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create todos
	blocker, _ := store.Create("Blocker", CreateOptions{})
	blocked, _ := store.Create("Blocked", CreateOptions{})
	_, _ = store.Create("Unblocked", CreateOptions{})

	// Add blocking dependency
	_, err = store.DepAdd(blocked.ID, blocker.ID, DepBlocks)
	if err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Only unblocked and blocker should be ready
	ready, err := store.Ready(10)
	if err != nil {
		t.Fatalf("failed to get ready: %v", err)
	}
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready todos, got %d", len(ready))
	}

	// Close the blocker
	store.Close([]string{blocker.ID}, "")

	// Now all three should be ready (but blocker is closed, so only 2)
	ready, err = store.Ready(10)
	if err != nil {
		t.Fatalf("failed to get ready: %v", err)
	}
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready todos after closing blocker, got %d", len(ready))
	}

	// Verify blocked is now ready
	foundBlocked := false
	for _, r := range ready {
		if r.ID == blocked.ID {
			foundBlocked = true
			break
		}
	}
	if !foundBlocked {
		t.Error("expected blocked todo to be ready after blocker was closed")
	}
}

func TestStore_Ready_Limit(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create 5 todos
	for i := 0; i < 5; i++ {
		store.Create("Todo", CreateOptions{})
	}

	// Limit to 3
	ready, err := store.Ready(3)
	if err != nil {
		t.Fatalf("failed to get ready: %v", err)
	}
	if len(ready) != 3 {
		t.Errorf("expected 3 todos with limit, got %d", len(ready))
	}
}

func TestStore_DepAdd(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	todo1, _ := store.Create("Todo 1", CreateOptions{})
	todo2, _ := store.Create("Todo 2", CreateOptions{})

	// Add dependency
	dep, err := store.DepAdd(todo1.ID, todo2.ID, DepBlocks)
	if err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	if dep.TodoID != todo1.ID {
		t.Errorf("expected TodoID %q, got %q", todo1.ID, dep.TodoID)
	}
	if dep.DependsOnID != todo2.ID {
		t.Errorf("expected DependsOnID %q, got %q", todo2.ID, dep.DependsOnID)
	}
	if dep.Type != DepBlocks {
		t.Errorf("expected type 'blocks', got %q", dep.Type)
	}
}

func TestStore_DepAdd_Validation(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	todo, _ := store.Create("Todo", CreateOptions{})

	// Self dependency
	_, err = store.DepAdd(todo.ID, todo.ID, DepBlocks)
	if !errors.Is(err, ErrSelfDependency) {
		t.Errorf("expected ErrSelfDependency, got %v", err)
	}

	// Invalid type
	_, err = store.DepAdd(todo.ID, "other", DependencyType("invalid"))
	if !errors.Is(err, ErrInvalidDependencyType) {
		t.Errorf("expected ErrInvalidDependencyType, got %v", err)
	}

	// Nonexistent todo
	_, err = store.DepAdd("nonexistent", todo.ID, DepBlocks)
	if !errors.Is(err, ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

func TestStore_DepAdd_Duplicate(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	todo1, _ := store.Create("Todo 1", CreateOptions{})
	todo2, _ := store.Create("Todo 2", CreateOptions{})

	// Add dependency
	store.DepAdd(todo1.ID, todo2.ID, DepBlocks)

	// Try to add again
	_, err = store.DepAdd(todo1.ID, todo2.ID, DepBlocks)
	if !errors.Is(err, ErrDuplicateDependency) {
		t.Errorf("expected ErrDuplicateDependency, got %v", err)
	}
}

func TestStore_DepTree(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	// Create tree:
	//   root
	//   ├── child1 (blocks)
	//   │   └── grandchild (blocks)
	//   └── child2 (discovered-from)
	root, _ := store.Create("Root", CreateOptions{})
	child1, _ := store.Create("Child 1", CreateOptions{})
	child2, _ := store.Create("Child 2", CreateOptions{})
	grandchild, _ := store.Create("Grandchild", CreateOptions{})

	store.DepAdd(root.ID, child1.ID, DepBlocks)
	store.DepAdd(root.ID, child2.ID, DepDiscoveredFrom)
	store.DepAdd(child1.ID, grandchild.ID, DepBlocks)

	// Get tree from root
	tree, err := store.DepTree(root.ID)
	if err != nil {
		t.Fatalf("failed to get dep tree: %v", err)
	}

	if tree.Todo.ID != root.ID {
		t.Errorf("expected root ID, got %q", tree.Todo.ID)
	}
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(tree.Children))
	}

	// Find child1 and verify it has grandchild
	var child1Node *DepTreeNode
	for _, child := range tree.Children {
		if child.Todo.ID == child1.ID {
			child1Node = child
			break
		}
	}
	if child1Node == nil {
		t.Fatal("child1 not found in tree")
	}
	if len(child1Node.Children) != 1 {
		t.Fatalf("expected 1 grandchild, got %d", len(child1Node.Children))
	}
	if child1Node.Children[0].Todo.ID != grandchild.ID {
		t.Errorf("expected grandchild ID, got %q", child1Node.Children[0].Todo.ID)
	}
}

func TestStore_DepTree_NotFound(t *testing.T) {
	store, err := openTestStore(t)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Release()

	_, err = store.DepTree("nonexistent")
	if !errors.Is(err, ErrTodoNotFound) {
		t.Errorf("expected ErrTodoNotFound, got %v", err)
	}
}

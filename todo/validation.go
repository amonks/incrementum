package todo

import (
	"errors"
	"fmt"
)

var (
	// ErrEmptyTitle is returned when a todo title is empty.
	ErrEmptyTitle = errors.New("title cannot be empty")

	// ErrTitleTooLong is returned when a todo title exceeds MaxTitleLength.
	ErrTitleTooLong = errors.New("title exceeds maximum length")

	// ErrInvalidStatus is returned when an invalid status is provided.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidPriority is returned when priority is outside valid range.
	ErrInvalidPriority = errors.New("priority must be between 0 and 4")

	// ErrInvalidType is returned when an invalid todo type is provided.
	ErrInvalidType = errors.New("invalid todo type")

	// ErrInvalidDependencyType is returned when an invalid dependency type is provided.
	ErrInvalidDependencyType = errors.New("invalid dependency type")

	// ErrTodoNotFound is returned when a todo with the given ID doesn't exist.
	ErrTodoNotFound = errors.New("todo not found")

	// ErrAmbiguousTodoIDPrefix is returned when an ID prefix matches multiple todos.
	ErrAmbiguousTodoIDPrefix = errors.New("ambiguous todo ID prefix")

	// ErrSelfDependency is returned when trying to create a dependency on itself.
	ErrSelfDependency = errors.New("todo cannot depend on itself")

	// ErrDuplicateDependency is returned when the dependency already exists.
	ErrDuplicateDependency = errors.New("dependency already exists")

	// ErrNoTaskStore is returned when the task store bookmark doesn't exist.
	ErrNoTaskStore = errors.New("no task store found (bookmark incr/tasks does not exist)")

	// ErrClosedTodoMissingClosedAt is returned when a closed or done todo has no closed_at timestamp.
	ErrClosedTodoMissingClosedAt = errors.New("closed or done todo must have closed_at timestamp")

	// ErrNotClosedTodoHasClosedAt is returned when a non-closed todo has a closed_at timestamp.
	ErrNotClosedTodoHasClosedAt = errors.New("non-closed todo cannot have closed_at timestamp")

	// ErrDeleteReasonRequiresDeletedAt is returned when a delete reason is provided without deleted_at.
	ErrDeleteReasonRequiresDeletedAt = errors.New("delete reason requires deleted_at timestamp")

	// ErrTombstoneMissingDeletedAt is returned when a tombstone todo has no deleted_at timestamp.
	ErrTombstoneMissingDeletedAt = errors.New("tombstone todo must have deleted_at timestamp")

	// ErrDeletedAtRequiresTombstoneStatus is returned when deleted_at is set without tombstone status.
	ErrDeletedAtRequiresTombstoneStatus = errors.New("deleted_at requires tombstone status")

	// ErrDeleteReasonRequiresTombstoneStatus is returned when delete reason is set without tombstone status.
	ErrDeleteReasonRequiresTombstoneStatus = errors.New("delete reason requires tombstone status")

	// ErrTombstoneHasClosedAt is returned when a tombstone todo has a closed_at timestamp.
	ErrTombstoneHasClosedAt = errors.New("tombstone todo cannot have closed_at timestamp")
)

// ValidateTitle checks if the title is valid.
func ValidateTitle(title string) error {
	if title == "" {
		return ErrEmptyTitle
	}
	if len(title) > MaxTitleLength {
		return fmt.Errorf("%w: %d > %d", ErrTitleTooLong, len(title), MaxTitleLength)
	}
	return nil
}

// ValidatePriority checks if the priority is valid.
func ValidatePriority(priority int) error {
	if priority < PriorityMin || priority > PriorityMax {
		return fmt.Errorf("%w: got %d", ErrInvalidPriority, priority)
	}
	return nil
}

// ValidateTodo checks if a todo struct is valid.
func ValidateTodo(t *Todo) error {
	if err := ValidateTitle(t.Title); err != nil {
		return err
	}

	if !t.Status.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidStatus, t.Status)
	}

	if err := ValidatePriority(t.Priority); err != nil {
		return err
	}

	if !t.Type.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidType, t.Type)
	}

	// Check closed_at consistency
	if t.Status == StatusClosed || t.Status == StatusDone || t.Status == StatusTombstone {
		if (t.Status == StatusClosed || t.Status == StatusDone) && t.ClosedAt == nil {
			return ErrClosedTodoMissingClosedAt
		}
		if t.Status == StatusTombstone && t.ClosedAt != nil {
			return ErrTombstoneHasClosedAt
		}
	} else {
		if t.ClosedAt != nil {
			return ErrNotClosedTodoHasClosedAt
		}
	}

	// Check deleted_at consistency
	if t.Status == StatusTombstone {
		if t.DeletedAt == nil {
			return ErrTombstoneMissingDeletedAt
		}
	} else {
		if t.DeletedAt != nil {
			return ErrDeletedAtRequiresTombstoneStatus
		}
		if t.DeleteReason != "" {
			return ErrDeleteReasonRequiresTombstoneStatus
		}
	}

	if t.DeletedAt == nil && t.DeleteReason != "" {
		return ErrDeleteReasonRequiresDeletedAt
	}

	return nil
}

// ValidateDependency checks if a dependency is valid.
func ValidateDependency(d *Dependency) error {
	if d.TodoID == "" {
		return fmt.Errorf("todo_id cannot be empty")
	}
	if d.DependsOnID == "" {
		return fmt.Errorf("depends_on_id cannot be empty")
	}
	if d.TodoID == d.DependsOnID {
		return ErrSelfDependency
	}
	if !d.Type.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidDependencyType, d.Type)
	}
	return nil
}

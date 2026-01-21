// Package todo implements a Jira-like todo tracker for jujutsu repositories.
//
// Todos are stored in a special orphan change (parented at root()) that is
// bookmarked as "incr/tasks". This allows todos to sync across workspaces
// without polluting the code history.
//
// The public API mirrors the CLI commands:
//   - Create, Update, Start, Close, Finish, Reopen for todo lifecycle
//   - Show, List, Ready for querying
//   - DepAdd, DepTree for dependency management
package todo

// Status represents the state of a todo.
type Status string

const (
	// StatusOpen indicates the todo is ready to be worked on.
	StatusOpen Status = "open"

	// StatusInProgress indicates the todo is currently being worked on.
	StatusInProgress Status = "in_progress"

	// StatusClosed indicates the todo has been completed.
	StatusClosed Status = "closed"

	// StatusDone indicates the todo is finished without closing it.
	StatusDone Status = "done"

	// StatusTombstone indicates the todo has been soft-deleted.
	StatusTombstone Status = "tombstone"
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{StatusOpen, StatusInProgress, StatusClosed, StatusDone, StatusTombstone}
}

// IsValid returns true if the status is a known valid value.
func (s Status) IsValid() bool {
	for _, valid := range ValidStatuses() {
		if s == valid {
			return true
		}
	}
	return false
}

// TodoType represents the category of a todo.
type TodoType string

const (
	// TypeTask is a general task (default).
	TypeTask TodoType = "task"

	// TypeBug is a bug fix.
	TypeBug TodoType = "bug"

	// TypeFeature is a new feature.
	TypeFeature TodoType = "feature"
)

// ValidTodoTypes returns all valid todo type values.
func ValidTodoTypes() []TodoType {
	return []TodoType{TypeTask, TypeBug, TypeFeature}
}

// IsValid returns true if the type is a known valid value.
func (t TodoType) IsValid() bool {
	for _, valid := range ValidTodoTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// DependencyType represents the relationship between two todos.
type DependencyType string

const (
	// DepBlocks indicates that DependsOnID blocks TodoID.
	// The TodoID cannot be completed until DependsOnID is done.
	DepBlocks DependencyType = "blocks"

	// DepDiscoveredFrom indicates that TodoID was discovered while working on DependsOnID.
	DepDiscoveredFrom DependencyType = "discovered-from"
)

// ValidDependencyTypes returns all valid dependency type values.
func ValidDependencyTypes() []DependencyType {
	return []DependencyType{DepBlocks, DepDiscoveredFrom}
}

// IsValid returns true if the dependency type is a known valid value.
func (d DependencyType) IsValid() bool {
	for _, valid := range ValidDependencyTypes() {
		if d == valid {
			return true
		}
	}
	return false
}

// Priority constants for todos.
const (
	PriorityCritical = 0
	PriorityHigh     = 1
	PriorityMedium   = 2 // default
	PriorityLow      = 3
	PriorityBacklog  = 4

	PriorityMin = 0
	PriorityMax = 4
)

// PriorityName returns a human-readable name for the priority level.
func PriorityName(p int) string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	case PriorityBacklog:
		return "backlog"
	default:
		return "unknown"
	}
}

// MaxTitleLength is the maximum allowed length for a todo title.
const MaxTitleLength = 500

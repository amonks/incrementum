package todo

import "testing"

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		status Status
		valid  bool
	}{
		{StatusOpen, true},
		{StatusInProgress, true},
		{StatusClosed, true},
		{StatusDone, true},
		{StatusTombstone, true},
		{Status("invalid"), false},
		{Status(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("Status(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestTodoType_IsValid(t *testing.T) {
	tests := []struct {
		typ   TodoType
		valid bool
	}{
		{TypeTask, true},
		{TypeBug, true},
		{TypeFeature, true},
		{TodoType("invalid"), false},
		{TodoType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			if got := tt.typ.IsValid(); got != tt.valid {
				t.Errorf("TodoType(%q).IsValid() = %v, want %v", tt.typ, got, tt.valid)
			}
		})
	}
}

func TestDependencyType_IsValid(t *testing.T) {
	tests := []struct {
		typ   DependencyType
		valid bool
	}{
		{DepBlocks, true},
		{DepDiscoveredFrom, true},
		{DependencyType("invalid"), false},
		{DependencyType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.typ), func(t *testing.T) {
			if got := tt.typ.IsValid(); got != tt.valid {
				t.Errorf("DependencyType(%q).IsValid() = %v, want %v", tt.typ, got, tt.valid)
			}
		})
	}
}

func TestPriorityName(t *testing.T) {
	tests := []struct {
		priority int
		name     string
	}{
		{PriorityCritical, "critical"},
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
		{PriorityBacklog, "backlog"},
		{-1, "unknown"},
		{5, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PriorityName(tt.priority); got != tt.name {
				t.Errorf("PriorityName(%d) = %q, want %q", tt.priority, got, tt.name)
			}
		})
	}
}

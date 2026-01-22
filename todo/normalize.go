package todo

import "strings"

func normalizeStatus(status Status) Status {
	return Status(strings.ToLower(string(status)))
}

func normalizeTodoType(todoType TodoType) TodoType {
	return TodoType(strings.ToLower(string(todoType)))
}

func normalizeDependencyType(depType DependencyType) DependencyType {
	return DependencyType(strings.ToLower(string(depType)))
}

package main

import (
	"os"
	"strings"

	"github.com/amonks/incrementum/todo"
)

const todoProposerEnvVar = "INCREMENTUM_TODO_PROPOSER"

func defaultTodoStatus() todo.Status {
	if strings.EqualFold(os.Getenv(todoProposerEnvVar), "true") {
		return todo.StatusProposed
	}
	return todo.StatusOpen
}

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/todo"
)

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
		createdAge := ui.FormatTimeAgeShort(t.CreatedAt, now)
		updatedAge := ui.FormatTimeAgeShort(t.UpdatedAt, now)
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

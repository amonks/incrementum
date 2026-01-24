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
	builder := ui.NewTableBuilder([]string{"ID", "PRI", "TYPE", "STATUS", "AGE", "DURATION", "TITLE"}, len(todos))

	if prefixLengths == nil {
		prefixLengths = todoIDPrefixLengths(todos)
	}

	for _, t := range todos {
		title := ui.TruncateTableCell(t.Title)
		prefixLen := prefixLengths[strings.ToLower(t.ID)]
		highlighted := highlight(t.ID, prefixLen)
		age := formatTodoAge(t, now)
		duration := formatTodoDuration(t, now)
		row := []string{
			highlighted,
			priorityShort(t.Priority),
			string(t.Type),
			string(t.Status),
			age,
			duration,
			title,
		}
		builder.AddRow(row)
	}

	return builder.String()
}

func todoIDPrefixLengths(todos []todo.Todo) map[string]int {
	index := todo.NewIDIndex(todos)
	return index.PrefixLengths()
}

func formatTodoAge(item todo.Todo, now time.Time) string {
	ageValue, ok := todo.AgeData(item, now)
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(ageValue)
}

func formatTodoDuration(item todo.Todo, now time.Time) string {
	duration, ok := todo.DurationData(item, now)
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(duration)
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

package job

import (
	"fmt"
	"strings"

	"github.com/amonks/incrementum/todo"
)

func formatCommitMessage(item todo.Todo, message string) string {
	summary, body := splitCommitMessage(message)
	formatted := wrapLines(summary, lineWidth)

	bodyText := reflowParagraphs(body, lineWidth-documentIndent)
	if strings.TrimSpace(bodyText) == "" {
		bodyText = "-"
	}
	formatted += "\n\nHere is a generated commit message:\n\n"
	formatted += indentBlock(bodyText, documentIndent)

	formatted += "\n\nThis commit is a step towards implementing this todo:\n\n"
	formatted += formatCommitTodo(item)
	return formatted
}

func splitCommitMessage(message string) (string, string) {
	message = strings.TrimRight(message, "\r\n")
	lines := strings.Split(message, "\n")
	if len(lines) == 0 {
		return "", ""
	}
	summary := strings.TrimSpace(lines[0])
	body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	return summary, body
}

func formatCommitTodo(item todo.Todo) string {
	fields := []string{
		fmt.Sprintf("ID: %s", item.ID),
		fmt.Sprintf("Title: %s", item.Title),
		fmt.Sprintf("Type: %s", item.Type),
		fmt.Sprintf("Priority: %d (%s)", item.Priority, todo.PriorityName(item.Priority)),
		"Description:",
	}
	fieldBlock := wrapLines(strings.Join(fields, "\n"), lineWidth-documentIndent)
	fieldBlock = indentBlock(fieldBlock, documentIndent)

	description := strings.TrimSpace(item.Description)
	if description == "" {
		description = "-"
	}
	description = reflowParagraphs(description, lineWidth-subdocumentIndent)
	if description == "" {
		description = "-"
	}
	description = indentBlock(description, subdocumentIndent)
	return fieldBlock + "\n" + description
}

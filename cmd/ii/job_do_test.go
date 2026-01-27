package main

import (
	"regexp"
	"strings"
	"testing"

	jobpkg "github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
	"github.com/spf13/cobra"
)

func TestReflowJobTextPreservesMarkdown(t *testing.T) {
	input := "Intro line.\n\n- First item\n- Second item\n\n```text\nline one\nline two\n```"
	output := reflowJobText(input, 80)

	if output == "-" {
		t.Fatalf("expected non-empty output, got %q", output)
	}
	checks := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^\s+Intro line\.$`),
		regexp.MustCompile(`(?m)^\s+.*First item$`),
		regexp.MustCompile(`(?m)^\s+.*Second item$`),
		regexp.MustCompile(`(?m)^\s+line one$`),
		regexp.MustCompile(`(?m)^\s+line two$`),
	}
	for _, check := range checks {
		if !check.MatchString(output) {
			t.Fatalf("expected markdown output to match %q, got %q", check.String(), output)
		}
	}
}

func TestFormatJobFieldWrapsValue(t *testing.T) {
	value := strings.Repeat("word ", 40)
	output := formatJobField("Title", value)

	firstIndent := strings.Repeat(" ", jobDocumentIndent)
	if !strings.HasPrefix(output, firstIndent+"Title: ") {
		t.Fatalf("expected title prefix, got %q", output)
	}
	continuationIndent := strings.Repeat(" ", jobDocumentIndent+len("Title: "))
	if !strings.Contains(output, "\n"+continuationIndent) {
		t.Fatalf("expected wrapped continuation indentation, got %q", output)
	}
}

func TestFormatCommitMessagesOutputPreservesIndentation(t *testing.T) {
	entries := []jobpkg.CommitLogEntry{{
		ID:      "commit-123",
		Message: "Summary line\n\nHere is a generated commit message:\n\n    Body line\n\nThis commit is a step towards implementing this todo:\n\n    ID: todo-1",
	}}

	output := formatCommitMessagesOutput(entries)
	if !strings.Contains(output, "Commit messages:") {
		t.Fatalf("expected header, got %q", output)
	}
	if !strings.Contains(output, "    Commit commit-123:") {
		t.Fatalf("expected commit id label, got %q", output)
	}
	if !strings.Contains(output, "\n        Summary line") {
		t.Fatalf("expected summary line indentation, got %q", output)
	}
	if !strings.Contains(output, "\n            Body line") {
		t.Fatalf("expected body line indentation, got %q", output)
	}
	if !strings.Contains(output, "\n            ID: todo-1") {
		t.Fatalf("expected commit message indentation preserved, got %q", output)
	}
}

func TestFormatCommitMessageOutputIndentsMessage(t *testing.T) {
	message := "Summary line\n\nHere is a generated commit message:\n\n    Body line\n\nThis commit is a step towards implementing this todo:\n\n    ID: todo-1"
	output := formatCommitMessageOutput(message)
	if !strings.Contains(output, "Commit message:") {
		t.Fatalf("expected header, got %q", output)
	}
	if !strings.Contains(output, "\n    Summary line") {
		t.Fatalf("expected summary indentation, got %q", output)
	}
	if !strings.Contains(output, "\n        Body line") {
		t.Fatalf("expected body indentation, got %q", output)
	}
}

func TestStageMessageUsesReviewLabel(t *testing.T) {
	message := jobpkg.StageMessage(jobpkg.StageReviewing)
	if message != "Starting review:" {
		t.Fatalf("expected review stage message, got %q", message)
	}
}

func TestRunJobDoMultipleTodos(t *testing.T) {
	originalJobRun := jobRun
	defer func() {
		jobRun = originalJobRun
	}()

	var got []string
	jobRun = func(repoPath, todoID string, opts jobpkg.RunOptions) (*jobpkg.RunResult, error) {
		got = append(got, todoID)
		if opts.EventStream != nil {
			close(opts.EventStream)
		}
		return &jobpkg.RunResult{}, nil
	}

	resetJobDoGlobals()
	cmd := newTestJobDoCommand()
	if err := runJobDo(cmd, []string{"todo-1", "todo-2", "todo-3"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{"todo-1", "todo-2", "todo-3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected job runs %v, got %v", want, got)
	}
}

func resetJobDoGlobals() {
	jobDoTitle = ""
	jobDoType = "task"
	jobDoPriority = todo.PriorityMedium
	jobDoDescription = ""
	jobDoDeps = nil
	jobDoEdit = false
	jobDoNoEdit = false
	jobDoAgent = ""
}

func newTestJobDoCommand() *cobra.Command {
	cmd := &cobra.Command{RunE: runJobDo}
	addDescriptionFlagAliases(cmd)
	cmd.Flags().StringVar(&jobDoTitle, "title", "", "Todo title")
	cmd.Flags().StringVarP(&jobDoType, "type", "t", "task", "Todo type (task, bug, feature)")
	cmd.Flags().IntVarP(&jobDoPriority, "priority", "p", todo.PriorityMedium, "Priority (0=critical, 1=high, 2=medium, 3=low, 4=backlog)")
	cmd.Flags().StringVarP(&jobDoDescription, "description", "d", "", "Description (use '-' to read from stdin)")
	cmd.Flags().StringArrayVar(&jobDoDeps, "deps", nil, "Dependencies in format <id> (e.g., abc123)")
	cmd.Flags().BoolVarP(&jobDoEdit, "edit", "e", false, "Open $EDITOR (default if interactive and no create flags)")
	cmd.Flags().BoolVar(&jobDoNoEdit, "no-edit", false, "Do not open $EDITOR")
	cmd.Flags().StringVar(&jobDoAgent, "agent", "", "Opencode agent")
	return cmd
}

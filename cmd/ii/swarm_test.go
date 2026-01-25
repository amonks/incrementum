package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/amonks/incrementum/todo"
)

func TestSwarmDoArgsAllowsTodoID(t *testing.T) {
	if err := swarmDoCmd.Args(swarmDoCmd, []string{"todo-123"}); err != nil {
		t.Fatalf("expected todo id to be accepted, got %v", err)
	}
}

func TestSwarmDoArgsRejectsTooManyArgs(t *testing.T) {
	if err := swarmDoCmd.Args(swarmDoCmd, []string{"todo-123", "extra"}); err == nil {
		t.Fatalf("expected too many args error")
	}
}

func TestSwarmDoRejectsTodoIDWithCreateFlags(t *testing.T) {
	cases := []struct {
		name  string
		flag  string
		value string
	}{
		{name: "title", flag: "title", value: "New title"},
		{name: "type", flag: "type", value: "bug"},
		{name: "priority", flag: "priority", value: "1"},
		{name: "description", flag: "description", value: "Some details"},
		{name: "deps", flag: "deps", value: "todo-456"},
		{name: "edit", flag: "edit", value: "true"},
		{name: "no-edit", flag: "no-edit", value: "true"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetSwarmDoState(t)
			if err := swarmDoCmd.Flags().Set(tc.flag, tc.value); err != nil {
				t.Fatalf("set %s flag: %v", tc.flag, err)
			}
			err := runSwarmDo(swarmDoCmd, []string{"todo-123"})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "todo id cannot be combined") {
				t.Fatalf("expected todo id guard error, got %v", err)
			}
		})
	}
}

func TestSwarmDoSkipsTodoCreationWithTodoID(t *testing.T) {
	resetSwarmDoState(t)
	repoPath := setupTestRepo(t)
	swarmPath = repoPath
	swarmAddr = "invalid"

	err := runSwarmDo(swarmDoCmd, []string{"todo-123"})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "title is required") {
		t.Fatalf("expected todo creation to be skipped, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("expected addr error, got %v", err)
	}
}

func TestSwarmDoUsesCwdWhenPathMissing(t *testing.T) {
	resetSwarmDoState(t)
	repoPath := setupTestRepo(t)
	swarmAddr = "invalid"

	withCwd(t, repoPath, func() {
		err := runSwarmDo(swarmDoCmd, []string{"todo-123"})
		if err == nil {
			t.Fatal("expected error")
		}
		if strings.Contains(err.Error(), "path is required") {
			t.Fatalf("expected to default path to cwd, got %v", err)
		}
		if !strings.Contains(err.Error(), "invalid port") {
			t.Fatalf("expected addr error, got %v", err)
		}
	})
}

func TestLogSwarmServeAddr(t *testing.T) {
	cases := []struct {
		name string
		addr string
		want string
	}{
		{name: "with host", addr: "127.0.0.1:9090", want: "127.0.0.1:9090"},
		{name: "all interfaces", addr: ":8088", want: "0.0.0.0:8088"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				if err := logSwarmServeAddr(tc.addr); err != nil {
					t.Fatalf("log swarm addr: %v", err)
				}
			})
			want := fmt.Sprintf("Swarm server listening on %s", tc.want)
			if strings.TrimSpace(output) != want {
				t.Fatalf("expected %q, got %q", want, output)
			}
		})
	}
}

func resetSwarmDoState(t *testing.T) {
	t.Helper()
	jobDoTitle = ""
	jobDoType = "task"
	jobDoPriority = todo.PriorityMedium
	jobDoDescription = ""
	jobDoDeps = nil
	jobDoEdit = false
	jobDoNoEdit = false
	swarmAddr = ""
	swarmPath = ""

	flags := swarmDoCmd.Flags()
	for _, name := range []string{"title", "type", "priority", "description", "deps", "edit", "no-edit", "addr", "path"} {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Fatalf("missing flag %q", name)
		}
		flag.Changed = false
	}
}

package swarm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/todo"
)

func setupSwarmRepo(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	homeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "state", "incrementum"), 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".local", "share", "incrementum", "workspaces"), 0o755); err != nil {
		t.Fatalf("create workspaces dir: %v", err)
	}
	t.Setenv("HOME", homeDir)

	client := jj.New()
	if err := client.Init(tmpDir); err != nil {
		t.Fatalf("failed to init jj repo: %v", err)
	}

	return tmpDir
}

func TestTodosListReturnsEmptyWhenMissing(t *testing.T) {
	repoDir := setupSwarmRepo(t)

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: t.TempDir(),
		Pool:     noopPool{},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, err := json.Marshal(todosListRequest{Filter: todo.ListFilter{}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/todos/list", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload todosListResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Todos) != 0 {
		t.Fatalf("expected empty todos, got %d", len(payload.Todos))
	}
}

func TestTodosCreateUpdateAndList(t *testing.T) {
	repoDir := setupSwarmRepo(t)

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: t.TempDir(),
		Pool:     noopPool{},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	createBody, err := json.Marshal(todosCreateRequest{
		Title:   "First todo",
		Options: todo.CreateOptions{Description: "Initial"},
	})
	if err != nil {
		t.Fatalf("marshal create request: %v", err)
	}
	createRequest := httptest.NewRequest(http.MethodPost, "/todos/create", bytes.NewReader(createBody))
	createResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", createResponse.Code)
	}

	var createPayload todosCreateResponse
	if err := json.NewDecoder(createResponse.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createPayload.Todo.ID == "" {
		t.Fatal("expected todo id")
	}

	newTitle := "Updated todo"
	updateBody, err := json.Marshal(todosUpdateRequest{
		IDs:     []string{createPayload.Todo.ID},
		Options: todo.UpdateOptions{Title: &newTitle},
	})
	if err != nil {
		t.Fatalf("marshal update request: %v", err)
	}
	updateRequest := httptest.NewRequest(http.MethodPost, "/todos/update", bytes.NewReader(updateBody))
	updateResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", updateResponse.Code)
	}

	var updatePayload todosUpdateResponse
	if err := json.NewDecoder(updateResponse.Body).Decode(&updatePayload); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if len(updatePayload.Todos) != 1 || updatePayload.Todos[0].Title != newTitle {
		t.Fatalf("expected updated todo title %q", newTitle)
	}

	listBody, err := json.Marshal(todosListRequest{Filter: todo.ListFilter{}})
	if err != nil {
		t.Fatalf("marshal list request: %v", err)
	}
	listRequest := httptest.NewRequest(http.MethodPost, "/todos/list", bytes.NewReader(listBody))
	listResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", listResponse.Code)
	}

	var listPayload todosListResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listPayload.Todos) != 1 || listPayload.Todos[0].Title != newTitle {
		t.Fatalf("expected listed todo title %q", newTitle)
	}
}

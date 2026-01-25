package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
)

func TestTodosViewDefaultsToFirstTodo(t *testing.T) {
	now := time.Now()
	todos := []todo.Todo{
		{
			ID:        "todo-1",
			Title:     "First todo",
			Status:    todo.StatusOpen,
			Priority:  todo.PriorityMedium,
			Type:      todo.TypeTask,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "todo-2",
			Title:     "Second todo",
			Status:    todo.StatusOpen,
			Priority:  todo.PriorityLow,
			Type:      todo.TypeFeature,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/todos/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(todosListResponse{Todos: todos})
	})
	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(listResponse{Jobs: []job.Job{}})
	})

	webHandler := NewHandler(Options{})
	mux.Handle("/web/", webHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/web/todos")
	if err != nil {
		t.Fatalf("get todos: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	output := string(body)
	if !strings.Contains(output, "value=\"First todo\"") {
		t.Fatalf("expected form to include first todo title, got %s", output)
	}
}

func TestTodoCreateRedirectsToNewTodo(t *testing.T) {
	createdID := "todo-9"
	now := time.Now()

	mux := http.NewServeMux()
	mux.HandleFunc("/todos/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request todosCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Title != "New todo" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Options.Status != todo.StatusOpen {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Options.Priority == nil || *request.Options.Priority != todo.PriorityMedium {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		response := todosCreateResponse{Todo: todo.Todo{
			ID:        createdID,
			Title:     request.Title,
			Status:    request.Options.Status,
			Priority:  *request.Options.Priority,
			Type:      request.Options.Type,
			CreatedAt: now,
			UpdatedAt: now,
		}}
		_ = json.NewEncoder(w).Encode(response)
	})

	webHandler := NewHandler(Options{})
	mux.Handle("/web/", webHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	form := url.Values{}
	form.Set("title", "New todo")
	form.Set("status", string(todo.StatusOpen))
	form.Set("priority", strconv.Itoa(todo.PriorityMedium))
	form.Set("type", string(todo.TypeTask))
	form.Set("description", "Notes")

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(server.URL+"/web/todos/create", form)
	if err != nil {
		t.Fatalf("post create: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if location != "/web/todos?id="+createdID {
		t.Fatalf("expected redirect to todo, got %q", location)
	}
}

func TestJobsViewDefaultsToFirstJob(t *testing.T) {
	now := time.Now()
	jobs := []job.Job{
		{
			ID:        "job-1",
			TodoID:    "todo-1",
			Stage:     job.StageImplementing,
			Status:    job.StatusActive,
			CreatedAt: now,
			StartedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "job-2",
			TodoID:    "todo-2",
			Stage:     job.StageTesting,
			Status:    job.StatusFailed,
			CreatedAt: now,
			StartedAt: now,
			UpdatedAt: now,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(listResponse{Jobs: jobs})
	})
	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(logsResponse{Events: []job.Event{}})
	})

	webHandler := NewHandler(Options{})
	mux.Handle("/web/", webHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/web/jobs")
	if err != nil {
		t.Fatalf("get jobs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	output := string(body)
	if !strings.Contains(output, "Job job-1") {
		t.Fatalf("expected page to include first job heading, got %s", output)
	}
}

func TestJobsStartRedirectsToNewJob(t *testing.T) {
	jobID := "job-99"

	mux := http.NewServeMux()
	mux.HandleFunc("/do", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request doRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.TodoID != "todo-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(doResponse{JobID: jobID})
	})

	webHandler := NewHandler(Options{})
	mux.Handle("/web/", webHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	form := url.Values{}
	form.Set("confirm", "yes")

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.PostForm(server.URL+"/web/jobs/start?id=todo-1", form)
	if err != nil {
		t.Fatalf("post start: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if location != "/web/jobs?id="+jobID {
		t.Fatalf("expected redirect to job, got %q", location)
	}
}

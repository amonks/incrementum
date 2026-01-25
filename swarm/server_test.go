package swarm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/workspace"
)

type noopPool struct{}

func (noopPool) Acquire(string, workspace.AcquireOptions) (string, error) {
	return "", nil
}

func (noopPool) Release(string) error {
	return nil
}

type poolCall struct {
	repoPath string
	options  workspace.AcquireOptions
}

type recordingPool struct {
	path  string
	calls chan poolCall
}

func (pool *recordingPool) Acquire(repoPath string, opts workspace.AcquireOptions) (string, error) {
	if pool.calls != nil {
		pool.calls <- poolCall{repoPath: repoPath, options: opts}
	}
	return pool.path, nil
}

func (pool *recordingPool) Release(string) error {
	return nil
}

type runCall struct {
	workspacePath string
	eventsDir     string
	interrupts    <-chan os.Signal
}

type panicPool struct{}

func (panicPool) Acquire(string, workspace.AcquireOptions) (string, error) {
	panic("boom")
}

func (panicPool) Release(string) error {
	return nil
}

func TestLogsReturnsEmptyEventsJSON(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	eventsDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath:        repoDir,
		StateDir:        stateDir,
		Pool:            noopPool{},
		EventLogOptions: job.EventLogOptions{EventsDir: eventsDir},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	manager, err := job.Open(repoDir, job.OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open job manager: %v", err)
	}
	created, err := manager.Create("todo-1", time.Now())
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	body, err := json.Marshal(logsRequest{JobID: created.ID})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if string(payload["events"]) != "[]" {
		t.Fatalf("expected empty events array, got %s", payload["events"])
	}
}

func TestListFiltersActiveJobsByDefault(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: stateDir,
		Pool:     noopPool{},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	manager, err := job.Open(repoDir, job.OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open job manager: %v", err)
	}

	activeJob, err := manager.Create("todo-active", time.Now())
	if err != nil {
		t.Fatalf("create active job: %v", err)
	}
	completedJob, err := manager.Create("todo-complete", time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("create completed job: %v", err)
	}
	status := job.StatusCompleted
	if _, err := manager.Update(completedJob.ID, job.UpdateOptions{Status: &status}, time.Now()); err != nil {
		t.Fatalf("update job status: %v", err)
	}

	body, err := json.Marshal(listRequest{Filter: job.ListFilter{}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/list", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload listResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].ID != activeJob.ID {
		t.Fatalf("expected active job list, got %v", payload.Jobs)
	}

	body, err = json.Marshal(listRequest{Filter: job.ListFilter{IncludeAll: true}})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request = httptest.NewRequest(http.MethodPost, "/list", bytes.NewReader(body))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Jobs) != 2 {
		t.Fatalf("expected two jobs, got %d", len(payload.Jobs))
	}
}

func TestDoStartsJobWithWorkspace(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	eventsDir := t.TempDir()
	workspacePath := filepath.Join(t.TempDir(), "ws")

	poolCalls := make(chan poolCall, 1)
	pool := &recordingPool{path: workspacePath, calls: poolCalls}
	jobCalls := make(chan runCall, 1)

	server, err := NewServer(ServerOptions{
		RepoPath:        repoDir,
		StateDir:        stateDir,
		Pool:            pool,
		EventLogOptions: job.EventLogOptions{EventsDir: eventsDir},
		RunJob: func(_ string, _ string, opts job.RunOptions) (*job.RunResult, error) {
			if opts.OnStart != nil {
				opts.OnStart(job.StartInfo{JobID: "job-123"})
			}
			jobCalls <- runCall{
				workspacePath: opts.WorkspacePath,
				eventsDir:     opts.EventLogOptions.EventsDir,
				interrupts:    opts.Interrupts,
			}
			return &job.RunResult{Job: job.Job{ID: "job-123"}}, nil
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, err := json.Marshal(doRequest{TodoID: "todo-1"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/do", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload doResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.JobID != "job-123" {
		t.Fatalf("expected job id, got %q", payload.JobID)
	}

	select {
	case call := <-poolCalls:
		if call.options.Rev != "main" {
			t.Fatalf("expected workspace rev main, got %q", call.options.Rev)
		}
		if call.options.Purpose == "" {
			t.Fatal("expected workspace purpose")
		}
		if call.options.NewChangeMessage != "staging for todo todo-1" {
			t.Fatalf("expected new change message, got %q", call.options.NewChangeMessage)
		}
	default:
		t.Fatal("expected workspace acquire call")
	}

	select {
	case call := <-jobCalls:
		if call.workspacePath != workspacePath {
			t.Fatalf("expected workspace path %q, got %q", workspacePath, call.workspacePath)
		}
		if call.eventsDir != eventsDir {
			t.Fatalf("expected events dir %q, got %q", eventsDir, call.eventsDir)
		}
		if call.interrupts == nil {
			t.Fatal("expected interrupts channel")
		}
	default:
		t.Fatal("expected job run call")
	}
}

func TestDoRejectsEmptyTodoID(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: stateDir,
		Pool:     noopPool{},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, err := json.Marshal(doRequest{TodoID: "  "})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/do", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}

	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(payload["error"], "todo id is required") {
		t.Fatalf("expected todo id error, got %q", payload["error"])
	}
}

func TestJobRequestsRejectEmptyJobID(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: stateDir,
		Pool:     noopPool{},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	cases := []struct {
		name    string
		path    string
		payload any
	}{
		{name: "kill", path: "/kill", payload: killRequest{JobID: ""}},
		{name: "logs", path: "/logs", payload: logsRequest{JobID: "  "}},
		{name: "tail", path: "/tail", payload: tailRequest{JobID: "\n"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			request := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(body))
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", response.Code)
			}

			var payload map[string]string
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !strings.Contains(payload["error"], "job id is required") {
				t.Fatalf("expected job id error, got %q", payload["error"])
			}
		})
	}
}

func TestDoRecoversFromJobPanic(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	var logs bytes.Buffer

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: stateDir,
		Pool:     noopPool{},
		Logger:   log.New(&logs, "", 0),
		RunJob: func(_ string, _ string, opts job.RunOptions) (*job.RunResult, error) {
			if opts.OnStart != nil {
				opts.OnStart(job.StartInfo{JobID: "job-panic"})
			}
			panic("boom")
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, err := json.Marshal(doRequest{TodoID: "todo-panic"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/do", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload doResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.JobID != "job-panic" {
		t.Fatalf("expected job id, got %q", payload.JobID)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		server.mu.Lock()
		_, ok := server.jobs[payload.JobID]
		server.mu.Unlock()
		if !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected job %s to be cleared", payload.JobID)
		}
		time.Sleep(10 * time.Millisecond)
	}

	output := logs.String()
	if !strings.Contains(output, "job panic for todo todo-panic") {
		t.Fatalf("expected panic log, got %q", output)
	}
	if !strings.Contains(output, "goroutine") {
		t.Fatalf("expected panic stack trace, got %q", output)
	}
}

func TestRequestPanicReturnsInternalError(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: stateDir,
		Pool:     panicPool{},
		Logger:   log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, err := json.Marshal(doRequest{TodoID: "todo-1"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/do", bytes.NewReader(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", response.Code)
	}

	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "internal server error" {
		t.Fatalf("expected internal error, got %q", payload["error"])
	}
}

func TestKillInterruptsJob(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	workspacePath := filepath.Join(t.TempDir(), "ws")

	jobCalls := make(chan runCall, 1)
	interruptReceived := make(chan struct{})

	server, err := NewServer(ServerOptions{
		RepoPath: repoDir,
		StateDir: stateDir,
		Pool:     &recordingPool{path: workspacePath, calls: make(chan poolCall, 1)},
		RunJob: func(_ string, _ string, opts job.RunOptions) (*job.RunResult, error) {
			if opts.OnStart != nil {
				opts.OnStart(job.StartInfo{JobID: "job-456"})
			}
			jobCalls <- runCall{interrupts: opts.Interrupts}
			<-opts.Interrupts
			close(interruptReceived)
			return &job.RunResult{Job: job.Job{ID: "job-456"}}, nil
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body, err := json.Marshal(doRequest{TodoID: "todo-2"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	startRequest := httptest.NewRequest(http.MethodPost, "/do", bytes.NewReader(body))
	startResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(startResponse, startRequest)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", startResponse.Code)
	}

	var payload doResponse
	if err := json.NewDecoder(startResponse.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.JobID != "job-456" {
		t.Fatalf("expected job id, got %q", payload.JobID)
	}

	var jobCall runCall
	select {
	case jobCall = <-jobCalls:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job call")
	}

	killBody, err := json.Marshal(killRequest{JobID: payload.JobID})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	killRequest := httptest.NewRequest(http.MethodPost, "/kill", bytes.NewReader(killBody))
	killResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(killResponse, killRequest)
	if killResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", killResponse.Code)
	}

	select {
	case <-interruptReceived:
	case <-time.After(2 * time.Second):
		if jobCall.interrupts != nil {
			select {
			case <-jobCall.interrupts:
			default:
			}
		}
		t.Fatal("timed out waiting for interrupt")
	}
}

func TestTailStreamsExistingAndNewEvents(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	eventsDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath:        repoDir,
		StateDir:        stateDir,
		Pool:            noopPool{},
		EventLogOptions: job.EventLogOptions{EventsDir: eventsDir},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	manager, err := job.Open(repoDir, job.OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open job manager: %v", err)
	}
	created, err := manager.Create("todo-3", time.Now())
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	jobID := created.ID

	log, err := job.OpenEventLog(jobID, job.EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	firstEvent := job.Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}
	if err := log.Append(firstEvent); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	serverInstance := httptest.NewServer(server.Handler())
	defer serverInstance.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payload, err := json.Marshal(tailRequest{JobID: jobID})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, serverInstance.URL+"/tail", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}

	decoder := json.NewDecoder(response.Body)
	var gotFirst job.Event
	if err := decoder.Decode(&gotFirst); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	if gotFirst.Name != firstEvent.Name {
		t.Fatalf("expected first event %q, got %q", firstEvent.Name, gotFirst.Name)
	}

	path, err := job.EventLogPath(jobID, job.EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("event log path: %v", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open event log for append: %v", err)
	}
	secondEvent := job.Event{Name: "job.prompt", Data: "{\"prompt\":\"hello\"}"}
	if err := json.NewEncoder(file).Encode(secondEvent); err != nil {
		_ = file.Close()
		t.Fatalf("append second event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	var gotSecond job.Event
	if err := decoder.Decode(&gotSecond); err != nil {
		t.Fatalf("decode second event: %v", err)
	}
	if gotSecond.Name != secondEvent.Name {
		t.Fatalf("expected second event %q, got %q", secondEvent.Name, gotSecond.Name)
	}

	cancel()
}

func TestTailResolvesTodoID(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	eventsDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath:        repoDir,
		StateDir:        stateDir,
		Pool:            noopPool{},
		EventLogOptions: job.EventLogOptions{EventsDir: eventsDir},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	manager, err := job.Open(repoDir, job.OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open job manager: %v", err)
	}
	todoID := "todo-5"
	created, err := manager.Create(todoID, time.Now())
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	log, err := job.OpenEventLog(created.ID, job.EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	firstEvent := job.Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}
	if err := log.Append(firstEvent); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	serverInstance := httptest.NewServer(server.Handler())
	defer serverInstance.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payload, err := json.Marshal(tailRequest{JobID: todoID})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, serverInstance.URL+"/tail", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}

	decoder := json.NewDecoder(response.Body)
	var gotFirst job.Event
	if err := decoder.Decode(&gotFirst); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	if gotFirst.Name != firstEvent.Name {
		t.Fatalf("expected first event %q, got %q", firstEvent.Name, gotFirst.Name)
	}
}

func TestTailWaitsForEventLog(t *testing.T) {
	repoDir := t.TempDir()
	stateDir := t.TempDir()
	eventsDir := t.TempDir()

	server, err := NewServer(ServerOptions{
		RepoPath:        repoDir,
		StateDir:        stateDir,
		Pool:            noopPool{},
		EventLogOptions: job.EventLogOptions{EventsDir: eventsDir},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	manager, err := job.Open(repoDir, job.OpenOptions{StateDir: stateDir})
	if err != nil {
		t.Fatalf("open job manager: %v", err)
	}
	created, err := manager.Create("todo-4", time.Now())
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	jobID := created.ID

	serverInstance := httptest.NewServer(server.Handler())
	defer serverInstance.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	firstEvent := job.Event{Name: "job.stage", Data: "{\"stage\":\"planning\"}"}
	errCh := make(chan error, 1)
	go func() {
		time.Sleep(200 * time.Millisecond)
		log, err := job.OpenEventLog(jobID, job.EventLogOptions{EventsDir: eventsDir})
		if err != nil {
			errCh <- err
			return
		}
		if err := log.Append(firstEvent); err != nil {
			_ = log.Close()
			errCh <- err
			return
		}
		if err := log.Close(); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	payload, err := json.Marshal(tailRequest{JobID: jobID})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, serverInstance.URL+"/tail", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}

	decoder := json.NewDecoder(response.Body)
	var gotFirst job.Event
	if err := decoder.Decode(&gotFirst); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	if gotFirst.Name != firstEvent.Name {
		t.Fatalf("expected first event %q, got %q", firstEvent.Name, gotFirst.Name)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("append event: %v", err)
	}
}

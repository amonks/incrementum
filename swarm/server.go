package swarm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/web"
	"github.com/amonks/incrementum/workspace"
)

// JobRunner runs a job for the given todo.
type JobRunner func(repoPath, todoID string, opts job.RunOptions) (*job.RunResult, error)

// WorkspacePool supplies workspaces for jobs.
type WorkspacePool interface {
	Acquire(repoPath string, opts workspace.AcquireOptions) (string, error)
	Release(path string) error
}

// ServerOptions configures a swarm server.
type ServerOptions struct {
	RepoPath        string
	StateDir        string
	WorkspacesDir   string
	Pool            WorkspacePool
	RunJob          JobRunner
	JobRunOptions   job.RunOptions
	EventLogOptions job.EventLogOptions
	Logger          *log.Logger
}

// Server handles swarm RPCs.
type Server struct {
	repoPath        string
	pool            WorkspacePool
	manager         *job.Manager
	runJob          JobRunner
	jobRunOptions   job.RunOptions
	eventLogOptions job.EventLogOptions
	logger          *log.Logger

	mu   sync.Mutex
	jobs map[string]*runningJob
}

type runningJob struct {
	interrupts chan os.Signal
	complete   chan struct{}
}

// NewServer creates a swarm server.
func NewServer(opts ServerOptions) (*Server, error) {
	if strings.TrimSpace(opts.RepoPath) == "" {
		return nil, fmt.Errorf("repo path is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "swarm: ", log.LstdFlags)
	}
	pool := opts.Pool
	if pool == nil {
		created, err := workspace.OpenWithOptions(workspace.Options{
			StateDir:      opts.StateDir,
			WorkspacesDir: opts.WorkspacesDir,
		})
		if err != nil {
			return nil, fmt.Errorf("open workspace pool: %w", err)
		}
		pool = created
	}
	manager, err := job.Open(opts.RepoPath, job.OpenOptions{StateDir: opts.StateDir})
	if err != nil {
		return nil, fmt.Errorf("open job manager: %w", err)
	}
	runJob := opts.RunJob
	if runJob == nil {
		runJob = job.Run
	}
	eventLogOptions := opts.EventLogOptions
	if eventLogOptions.EventsDir == "" {
		eventLogOptions = opts.JobRunOptions.EventLogOptions
	}

	return &Server{
		repoPath:        opts.RepoPath,
		pool:            pool,
		manager:         manager,
		runJob:          runJob,
		jobRunOptions:   opts.JobRunOptions,
		eventLogOptions: eventLogOptions,
		logger:          logger,
		jobs:            make(map[string]*runningJob),
	}, nil
}

// Handler returns the HTTP handler for swarm RPCs.
func (s *Server) Handler() http.Handler {
	return s.handler("")
}

func (s *Server) handler(baseURL string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/do", s.handleDo)
	mux.HandleFunc("/kill", s.handleKill)
	mux.HandleFunc("/tail", s.handleTail)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/list", s.handleList)
	mux.HandleFunc("/todos/list", s.handleTodosList)
	mux.HandleFunc("/todos/create", s.handleTodosCreate)
	mux.HandleFunc("/todos/update", s.handleTodosUpdate)
	webHandler := web.NewHandler(web.Options{BaseURL: baseURL})
	mux.Handle("/web/", webHandler)
	mux.Handle("/web", http.RedirectHandler("/web/todos", http.StatusFound))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/web/todos", http.StatusFound)
	})
	return s.recoverHandler(mux)
}

// Serve runs the server on the given address.
func (s *Server) Serve(addr string) error {
	server := &http.Server{
		Addr:     addr,
		Handler:  s.handler(resolveWebBaseURL(addr)),
		ErrorLog: s.logger,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logf("server stopped: %v", err)
		return err
	}
	return nil
}

func resolveWebBaseURL(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return strings.TrimRight(trimmed, "/")
	}
	host := trimmed
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if strings.HasPrefix(host, "0.0.0.0:") {
		host = "127.0.0.1:" + strings.TrimPrefix(host, "0.0.0.0:")
	}
	return "http://" + host
}

func (s *Server) handleDo(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload doRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	todoID := strings.TrimSpace(payload.TodoID)
	if todoID == "" {
		s.writeError(w, r, http.StatusBadRequest, fmt.Errorf("todo id is required"))
		return
	}
	jobID, err := s.startJob(r.Context(), todoID)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, doResponse{JobID: jobID})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload killRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	jobID := strings.TrimSpace(payload.JobID)
	if jobID == "" {
		s.writeError(w, r, http.StatusBadRequest, fmt.Errorf("job id is required"))
		return
	}
	if err := s.killJob(jobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrJobNotFound) {
			status = http.StatusNotFound
		}
		s.writeError(w, r, status, err)
		return
	}
	writeJSON(w, http.StatusOK, emptyResponse{})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload logsRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	jobID := strings.TrimSpace(payload.JobID)
	if jobID == "" {
		s.writeError(w, r, http.StatusBadRequest, fmt.Errorf("job id is required"))
		return
	}
	if _, err := s.manager.Find(jobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrJobNotFound) {
			status = http.StatusNotFound
		}
		s.writeError(w, r, status, err)
		return
	}
	events, err := job.EventSnapshot(jobID, s.eventLogOptions)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logsResponse{Events: events})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload listRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	jobs, err := s.manager.List(payload.Filter)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Jobs: jobs})
}

func (s *Server) handleTodosList(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload todosListRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	store, err := todo.Open(s.repoPath, todo.OpenOptions{
		CreateIfMissing: false,
		PromptToCreate:  false,
		ReadOnly:        true,
		Purpose:         "swarm todos list",
	})
	if err != nil {
		if errors.Is(err, todo.ErrNoTodoStore) {
			writeJSON(w, http.StatusOK, todosListResponse{Todos: []todo.Todo{}})
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	defer store.Release()

	todos, err := store.List(payload.Filter)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, todosListResponse{Todos: todos})
}

func (s *Server) handleTodosCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload todosCreateRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	store, err := todo.Open(s.repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "swarm todos create",
	})
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	defer store.Release()

	created, err := store.Create(payload.Title, payload.Options)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, todosCreateResponse{Todo: *created})
}

func (s *Server) handleTodosUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload todosUpdateRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	store, err := todo.Open(s.repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  false,
		Purpose:         "swarm todos update",
	})
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	defer store.Release()

	updated, err := store.Update(payload.IDs, payload.Options)
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, todosUpdateResponse{Todos: updated})
}

func (s *Server) handleTail(w http.ResponseWriter, r *http.Request) {
	if !s.requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload tailRequest
	if err := decodeJSON(r, &payload); err != nil {
		s.writeError(w, r, http.StatusBadRequest, err)
		return
	}
	jobID := strings.TrimSpace(payload.JobID)
	if jobID == "" {
		s.writeError(w, r, http.StatusBadRequest, fmt.Errorf("job id is required"))
		return
	}
	if _, err := s.manager.Find(jobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrJobNotFound) {
			status = http.StatusNotFound
		}
		s.writeError(w, r, status, err)
		return
	}
	if err := streamEvents(r.Context(), w, jobID, s.eventLogOptions); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, err)
	}
}

func (s *Server) recoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &responseTracker{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logf("panic handling request %s %s: %v\n%s", r.Method, r.URL.Path, recovered, debug.Stack())
				if writer.wroteHeader {
					return
				}
				writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(writer, r)
	})
}

func (s *Server) startJob(ctx context.Context, todoID string) (string, error) {
	cleanID := strings.TrimSpace(todoID)
	if cleanID == "" {
		return "", fmt.Errorf("todo id is required")
	}
	stagingMessage := fmt.Sprintf("staging for todo %s", cleanID)
	wsPath, err := s.pool.Acquire(s.repoPath, workspace.AcquireOptions{
		Purpose:          fmt.Sprintf("swarm job %s", cleanID),
		Rev:              "main",
		NewChangeMessage: stagingMessage,
	})
	if err != nil {
		return "", fmt.Errorf("acquire workspace: %w", err)
	}

	interrupts := make(chan os.Signal, 1)
	started := make(chan job.StartInfo, 1)
	completed := make(chan struct{})
	var runResult *job.RunResult
	var runErr error
	var jobID string

	runOpts := s.jobRunOptions
	runOpts.WorkspacePath = wsPath
	runOpts.Interrupts = interrupts
	runOpts.EventLogOptions = s.eventLogOptions
	baseOnStart := runOpts.OnStart
	runOpts.OnStart = func(info job.StartInfo) {
		jobID = info.JobID
		if baseOnStart != nil {
			baseOnStart(info)
		}
		select {
		case started <- info:
		default:
		}
	}

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				label := jobID
				if label == "" {
					label = "unknown"
				}
				s.logf("job panic for todo %s (job %s): %v\n%s", cleanID, label, recovered, debug.Stack())
				runErr = fmt.Errorf("job panic: %v", recovered)
			}
			close(completed)
			if releaseErr := s.pool.Release(wsPath); releaseErr != nil {
				s.logf("release workspace for todo %s: %v", cleanID, releaseErr)
			}
			finalJobID := jobID
			if runResult != nil && runResult.Job.ID != "" {
				finalJobID = runResult.Job.ID
			}
			if finalJobID != "" {
				s.mu.Lock()
				delete(s.jobs, finalJobID)
				s.mu.Unlock()
			}
			s.logJobCompletion(cleanID, finalJobID, runResult, runErr)
		}()
		runResult, runErr = s.runJob(s.repoPath, todoID, runOpts)
	}()

	select {
	case info := <-started:
		s.mu.Lock()
		s.jobs[info.JobID] = &runningJob{interrupts: interrupts, complete: completed}
		s.mu.Unlock()
		s.logf("job %s started for todo %s", info.JobID, cleanID)
		return info.JobID, nil
	case <-completed:
		if runErr != nil {
			return "", runErr
		}
		if runResult != nil && runResult.Job.ID != "" {
			return runResult.Job.ID, nil
		}
		return "", fmt.Errorf("job start failed")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *Server) killJob(jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return job.ErrJobNotFound
	}
	s.mu.Lock()
	handle, ok := s.jobs[jobID]
	s.mu.Unlock()
	if !ok {
		return job.ErrJobNotFound
	}
	select {
	case handle.interrupts <- os.Interrupt:
	default:
	}
	return nil
}

type doRequest struct {
	TodoID string `json:"todo_id"`
}

type doResponse struct {
	JobID string `json:"job_id"`
}

type killRequest struct {
	JobID string `json:"job_id"`
}

type logsRequest struct {
	JobID string `json:"job_id"`
}

type logsResponse struct {
	Events []job.Event `json:"events"`
}

type tailRequest struct {
	JobID string `json:"job_id"`
}

type listResponse struct {
	Jobs []job.Job `json:"jobs"`
}

type listRequest struct {
	Filter job.ListFilter `json:"filter"`
}

type todosListRequest struct {
	Filter todo.ListFilter `json:"filter"`
}

type todosListResponse struct {
	Todos []todo.Todo `json:"todos"`
}

type todosCreateRequest struct {
	Title   string             `json:"title"`
	Options todo.CreateOptions `json:"options"`
}

type todosCreateResponse struct {
	Todo todo.Todo `json:"todo"`
}

type todosUpdateRequest struct {
	IDs     []string           `json:"ids"`
	Options todo.UpdateOptions `json:"options"`
}

type todosUpdateResponse struct {
	Todos []todo.Todo `json:"todos"`
}

type emptyResponse struct{}

func (s *Server) requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	s.writeError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
	return false
}

func decodeJSON(r *http.Request, dest any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected extra JSON data")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	s.logRequestError(r, status, err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) logRequestError(r *http.Request, status int, err error) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Printf("request %s %s failed (%d): %v", r.Method, r.URL.Path, status, err)
}

func (s *Server) logJobCompletion(todoID, jobID string, result *job.RunResult, err error) {
	if s == nil || s.logger == nil {
		return
	}
	label := jobID
	if label == "" {
		label = "unknown"
	}
	status := ""
	if result != nil && result.Job.Status != "" {
		status = string(result.Job.Status)
	}
	if err != nil {
		s.logger.Printf("job %s for todo %s failed: %v", label, todoID, err)
		return
	}
	if status != "" {
		s.logger.Printf("job %s for todo %s finished with status %s", label, todoID, status)
		return
	}
	s.logger.Printf("job %s for todo %s finished", label, todoID)
}

func (s *Server) logf(format string, args ...any) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Printf(format, args...)
}

type responseTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *responseTracker) WriteHeader(status int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseTracker) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseTracker) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func streamEvents(ctx context.Context, w http.ResponseWriter, jobID string, opts job.EventLogOptions) error {
	path, err := job.EventLogPath(jobID, opts)
	if err != nil {
		return err
	}
	if err := waitForFile(ctx, path); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response does not support streaming")
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(w)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			var event job.Event
			if unmarshalErr := json.Unmarshal([]byte(line), &event); unmarshalErr != nil {
				return fmt.Errorf("decode job event: %w", unmarshalErr)
			}
			if err := encoder.Encode(event); err != nil {
				return err
			}
			flusher.Flush()
		}
		if errors.Is(err, io.EOF) {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}
	}
}

func waitForFile(ctx context.Context, path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

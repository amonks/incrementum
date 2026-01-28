package swarm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/amonks/incrementum/internal/ids"
	internalstrings "github.com/amonks/incrementum/internal/strings"
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

var errAmbiguousTodoIDPrefix = errors.New("ambiguous todo id prefix")

type runningJob struct {
	interrupts chan os.Signal
	complete   chan struct{}
}

const shutdownJobTimeout = 5 * time.Second

// NewServer creates a swarm server.
func NewServer(opts ServerOptions) (*Server, error) {
	if internalstrings.IsBlank(opts.RepoPath) {
		return nil, fmt.Errorf("repo path is required")
	}
	repoPath := filepath.Clean(opts.RepoPath)
	if abs, err := filepath.Abs(repoPath); err == nil {
		repoPath = abs
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
	manager, err := job.Open(repoPath, job.OpenOptions{StateDir: opts.StateDir})
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
		repoPath:        repoPath,
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

func (s *Server) openTodoStore(opts todo.OpenOptions) (*todo.Store, error) {
	opts.PromptToCreate = false
	return todo.Open(s.repoPath, opts)
}

// Serve runs the server on the given address.
func (s *Server) Serve(addr string) error {
	server := &http.Server{
		Addr:     addr,
		Handler:  s.handler(resolveWebBaseURL(addr)),
		ErrorLog: s.logger,
	}

	listenErrs := make(chan error, 1)
	go func() {
		listenErrs <- server.ListenAndServe()
	}()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	select {
	case err := <-listenErrs:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logf("server stopped: %v", err)
			return err
		}
		return nil
	case <-interrupts:
		s.logf("interrupt received, shutting down")
		jobErr := s.shutdownJobs(shutdownJobTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownJobTimeout)
		shutdownErr := server.Shutdown(shutdownCtx)
		cancel()
		listenErr := <-listenErrs
		if errors.Is(listenErr, http.ErrServerClosed) {
			listenErr = nil
		}
		if errors.Is(shutdownErr, http.ErrServerClosed) {
			shutdownErr = nil
		}
		if err := errors.Join(jobErr, shutdownErr, listenErr); err != nil {
			return err
		}
		return nil
	}
}

func (s *Server) shutdownJobs(timeout time.Duration) error {
	s.mu.Lock()
	jobs := make(map[string]*runningJob, len(s.jobs))
	for jobID, handle := range s.jobs {
		jobs[jobID] = handle
	}
	s.mu.Unlock()

	for _, handle := range jobs {
		select {
		case handle.interrupts <- os.Interrupt:
		default:
		}
	}

	deadline := time.Now().Add(timeout)
	var jobErr error
	for jobID, handle := range jobs {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			jobErr = errors.Join(jobErr, s.failActiveJob(jobID))
			continue
		}
		waitTimer := time.NewTimer(remaining)
		select {
		case <-handle.complete:
			if !waitTimer.Stop() {
				<-waitTimer.C
			}
		case <-waitTimer.C:
			jobErr = errors.Join(jobErr, s.failActiveJob(jobID))
		}
	}
	return jobErr
}

func (s *Server) failActiveJob(jobID string) error {
	jobRecord, err := s.manager.Find(jobID)
	if err != nil {
		return err
	}
	if jobRecord.Status != job.StatusActive {
		return nil
	}
	status := job.StatusFailed
	updated, err := s.manager.Update(jobRecord.ID, job.UpdateOptions{Status: &status}, time.Now())
	if err != nil {
		return err
	}
	if updated.TodoID == "" {
		return nil
	}
	store, err := s.openTodoStore(todo.OpenOptions{CreateIfMissing: false})
	if err != nil {
		return err
	}
	_, reopenErr := store.Reopen([]string{updated.TodoID})
	releaseErr := store.Release()
	return errors.Join(reopenErr, releaseErr)
}

func resolveWebBaseURL(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return internalstrings.TrimTrailingSlash(trimmed)
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
	store, err := s.openTodoStore(todo.OpenOptions{
		CreateIfMissing: false,
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
	store, err := s.openTodoStore(todo.OpenOptions{
		CreateIfMissing: true,
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
	store, err := s.openTodoStore(todo.OpenOptions{
		CreateIfMissing: true,
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
	resolvedJobID, err := resolveTailJobID(s.manager, jobID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, job.ErrJobNotFound):
			status = http.StatusNotFound
		case errors.Is(err, job.ErrAmbiguousJobIDPrefix), errors.Is(err, errAmbiguousTodoIDPrefix):
			status = http.StatusBadRequest
		}
		s.writeError(w, r, status, err)
		return
	}
	if err := streamEvents(r.Context(), w, resolvedJobID, s.eventLogOptions, s.manager, s.isJobRunning); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, err)
	}
}

func (s *Server) isJobRunning(jobID string) bool {
	if internalstrings.IsBlank(jobID) {
		return false
	}
	s.mu.Lock()
	_, ok := s.jobs[jobID]
	s.mu.Unlock()
	return ok
}

func resolveTailJobID(manager *job.Manager, input string) (string, error) {
	jobRecord, err := manager.Find(input)
	if err == nil {
		return jobRecord.ID, nil
	}
	if !errors.Is(err, job.ErrJobNotFound) {
		return "", err
	}
	jobs, err := manager.List(job.ListFilter{IncludeAll: true})
	if err != nil {
		return "", err
	}
	if len(jobs) == 0 {
		return "", job.ErrJobNotFound
	}
	idsByTodo := make([]string, 0, len(jobs))
	for _, item := range jobs {
		if item.TodoID != "" {
			idsByTodo = append(idsByTodo, item.TodoID)
		}
	}
	matchID, matched, ambiguous := ids.MatchPrefix(idsByTodo, input)
	if ambiguous {
		return "", fmt.Errorf("%w: %s", errAmbiguousTodoIDPrefix, input)
	}
	if !matched {
		return "", job.ErrJobNotFound
	}
	var candidate job.Job
	hasCandidate := false
	for _, item := range jobs {
		if item.TodoID != matchID {
			continue
		}
		if !hasCandidate {
			candidate = item
			hasCandidate = true
			continue
		}
		if item.Status == job.StatusActive {
			if candidate.Status != job.StatusActive || item.StartedAt.After(candidate.StartedAt) {
				candidate = item
			}
			continue
		}
		if candidate.Status == job.StatusActive {
			continue
		}
		if item.StartedAt.After(candidate.StartedAt) {
			candidate = item
		}
	}
	if !hasCandidate {
		return "", job.ErrJobNotFound
	}
	return candidate.ID, nil
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
		Rev:              "@",
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
		if runErr == nil && runResult != nil && runResult.Job.Status == job.StatusCompleted {
			if err := syncWorkspaceOutputs(wsPath, s.repoPath); err != nil {
				s.logf("sync workspace outputs for todo %s: %v", cleanID, err)
			}
		}
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
	if internalstrings.IsBlank(jobID) {
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

func streamEvents(ctx context.Context, w http.ResponseWriter, jobID string, opts job.EventLogOptions, manager *job.Manager, isRunning func(string) bool) error {
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
	var pending []byte
	for {
		chunk, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
			return err
		}
		if len(chunk) > 0 {
			pending = append(pending, chunk...)
			if chunk[len(chunk)-1] == '\n' {
				line := strings.TrimSpace(string(pending))
				pending = pending[:0]
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
			}
		}
		if errors.Is(err, io.EOF) {
			if manager != nil && len(pending) == 0 {
				terminal, checkErr := isJobTerminal(manager, jobID)
				if checkErr != nil {
					return checkErr
				}
				if terminal {
					if isRunning != nil && isRunning(jobID) {
						select {
						case <-ctx.Done():
							return nil
						case <-time.After(200 * time.Millisecond):
							continue
						}
					}
					return nil
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
	}
}

func isJobTerminal(manager *job.Manager, jobID string) (bool, error) {
	if manager == nil {
		return false, nil
	}
	record, err := manager.Find(jobID)
	if err != nil {
		if errors.Is(err, job.ErrJobNotFound) {
			return false, nil
		}
		return false, err
	}
	return record.Status != job.StatusActive, nil
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

func syncWorkspaceOutputs(workspacePath, repoPath string) error {
	workspacePath = filepath.Clean(workspacePath)
	repoPath = filepath.Clean(repoPath)
	return filepath.WalkDir(workspacePath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workspacePath, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkipWorkspaceEntry(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		return copyWorkspaceFile(path, filepath.Join(repoPath, rel))
	})
}

func shouldSkipWorkspaceEntry(rel string, entry fs.DirEntry) bool {
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) == 0 {
		return false
	}
	top := parts[0]
	if top == ".jj" || top == ".git" || top == ".incrementum" {
		return true
	}
	if strings.HasPrefix(top, ".incrementum-") {
		return true
	}
	if entry.IsDir() {
		return false
	}
	return top == ".incrementum-commit-message" || top == ".incrementum-feedback" || top == ".incrementum-work-complete"
}

func copyWorkspaceFile(srcPath, destPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dest, src)
	closeErr := dest.Close()
	return errors.Join(copyErr, closeErr)
}

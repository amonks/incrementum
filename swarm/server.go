package swarm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amonks/incrementum/job"
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
}

// Server handles swarm RPCs.
type Server struct {
	repoPath        string
	pool            WorkspacePool
	manager         *job.Manager
	runJob          JobRunner
	jobRunOptions   job.RunOptions
	eventLogOptions job.EventLogOptions

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
		jobs:            make(map[string]*runningJob),
	}, nil
}

// Handler returns the HTTP handler for swarm RPCs.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/do", s.handleDo)
	mux.HandleFunc("/kill", s.handleKill)
	mux.HandleFunc("/tail", s.handleTail)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/list", s.handleList)
	return mux
}

// Serve runs the server on the given address.
func (s *Server) Serve(addr string) error {
	server := &http.Server{Addr: addr, Handler: s.Handler()}
	return server.ListenAndServe()
}

func (s *Server) handleDo(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload doRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	jobID, err := s.startJob(r.Context(), payload.TodoID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, doResponse{JobID: jobID})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload killRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.killJob(payload.JobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrJobNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, emptyResponse{})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload logsRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.manager.Find(payload.JobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrJobNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	events, err := job.EventSnapshot(payload.JobID, s.eventLogOptions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logsResponse{Events: events})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	jobs, err := s.manager.List(job.ListFilter{IncludeAll: true})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Jobs: jobs})
}

func (s *Server) handleTail(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var payload tailRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := s.manager.Find(payload.JobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, job.ErrJobNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	if err := streamEvents(r.Context(), w, payload.JobID, s.eventLogOptions); err != nil {
		writeError(w, http.StatusInternalServerError, err)
	}
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

	runOpts := s.jobRunOptions
	runOpts.WorkspacePath = wsPath
	runOpts.Interrupts = interrupts
	runOpts.EventLogOptions = s.eventLogOptions
	baseOnStart := runOpts.OnStart
	runOpts.OnStart = func(info job.StartInfo) {
		if baseOnStart != nil {
			baseOnStart(info)
		}
		select {
		case started <- info:
		default:
		}
	}

	go func() {
		runResult, runErr = s.runJob(s.repoPath, todoID, runOpts)
		close(completed)
		_ = s.pool.Release(wsPath)
		if runResult != nil && runResult.Job.ID != "" {
			s.mu.Lock()
			delete(s.jobs, runResult.Job.ID)
			s.mu.Unlock()
		}
	}()

	select {
	case info := <-started:
		s.mu.Lock()
		s.jobs[info.JobID] = &runningJob{interrupts: interrupts, complete: completed}
		s.mu.Unlock()
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

type emptyResponse struct{}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed", r.Method))
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

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
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

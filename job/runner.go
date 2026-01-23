package job

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/jj"
	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/workspace"
)

const (
	feedbackFilename      = ".incrementum-feedback"
	commitMessageFilename = ".incrementum-commit-message"
)

// RunOptions configures job execution.
type RunOptions struct {
	OnStart       func(StartInfo)
	OnStageChange func(Stage)
	Now           func() time.Time
	LoadConfig    func(string) (*config.Config, error)
	RunTests      func(string, []string) ([]TestCommandResult, error)
	RunOpencode   func(opencodeRunOptions) (OpencodeRunResult, error)
	UpdateStale   func(string) error
}

// RunResult captures the output of running a job.
type RunResult struct {
	Job           Job
	CommitMessage string
}

// OpencodeRunResult captures output from running opencode.
type OpencodeRunResult struct {
	SessionID string
	ExitCode  int
}

type opencodeRunOptions struct {
	RepoPath      string
	WorkspacePath string
	Prompt        string
	StartedAt     time.Time
}

// Run creates and executes a job for the given todo.
func Run(repoPath, todoID string, opts RunOptions) (*RunResult, error) {
	if strings.TrimSpace(todoID) == "" {
		return nil, fmt.Errorf("todo id is required")
	}

	opts = normalizeRunOptions(opts)
	result := &RunResult{}
	repoPath = filepath.Clean(repoPath)
	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
	}

	store, err := todo.Open(repoPath, todo.OpenOptions{
		CreateIfMissing: true,
		PromptToCreate:  true,
		Purpose:         fmt.Sprintf("todo store (job run %s)", todoID),
	})
	if err != nil {
		return result, err
	}

	items, err := store.Show([]string{todoID})
	if err != nil {
		releaseErr := store.Release()
		return result, errors.Join(err, releaseErr)
	}
	if len(items) == 0 {
		releaseErr := store.Release()
		return result, errors.Join(fmt.Errorf("todo not found: %s", todoID), releaseErr)
	}
	item := items[0]
	_, err = store.Start([]string{item.ID})
	releaseErr := store.Release()
	if err != nil {
		return result, errors.Join(err, releaseErr)
	}
	if releaseErr != nil {
		return result, releaseErr
	}
	startedAt := opts.Now()
	workspacePath := repoPath
	workspaceAbs := repoPath
	if opts.OnStart != nil {
		opts.OnStart(StartInfo{
			Workdir: workspaceAbs,
			Todo:    item,
		})
	}

	manager, err := Open(repoPath, OpenOptions{})
	if err != nil {
		reopenErr := reopenTodo(repoPath, item.ID)
		return result, errors.Join(err, reopenErr)
	}

	created, err := manager.Create(item.ID, startedAt)
	if err != nil {
		reopenErr := reopenTodo(repoPath, item.ID)
		return result, errors.Join(err, reopenErr)
	}
	result.Job = created
	if opts.OnStageChange != nil {
		opts.OnStageChange(created.Stage)
	}

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	runCtx := runContext{
		repoPath:      repoPath,
		workspacePath: workspacePath,
		item:          item,
		opts:          opts,
		manager:       manager,
		result:        result,
	}
	finalJob, err := runJobStages(runCtx, created, interrupts)
	result.Job = finalJob
	statusErr := finalizeTodo(repoPath, item.ID, finalJob.Status)
	if err != nil {
		return result, errors.Join(err, statusErr)
	}
	if statusErr != nil {
		return result, statusErr
	}
	return result, nil
}

type runContext struct {
	repoPath      string
	workspacePath string
	item          todo.Todo
	opts          RunOptions
	manager       *Manager
	result        *RunResult
}

func runJobStages(ctx runContext, current Job, interrupts <-chan os.Signal) (Job, error) {
	for current.Status == StatusActive {
		stageFn, stageErr := ctx.stageRunner(current)
		next := Job{}
		if stageFn != nil {
			next, stageErr = ctx.runStageWithInterrupt(current, stageFn, interrupts)
		}
		if stageErr != nil && errors.Is(stageErr, ErrJobInterrupted) {
			return next, stageErr
		}
		current, stageErr = ctx.handleStageOutcome(current, next, stageErr)
		if stageErr != nil {
			return current, stageErr
		}
		if current.Status != StatusActive {
			break
		}
	}

	return current, nil
}

func (ctx runContext) stageRunner(current Job) (func() (Job, error), error) {
	switch current.Stage {
	case StageImplementing:
		return func() (Job, error) {
			return runImplementingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts)
		}, nil
	case StageTesting:
		return func() (Job, error) {
			return runTestingStage(ctx.manager, current, ctx.repoPath, ctx.workspacePath, ctx.opts)
		}, nil
	case StageReviewing:
		return func() (Job, error) {
			return runReviewingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts)
		}, nil
	case StageCommitting:
		return func() (Job, error) {
			return runCommittingStage(CommittingStageOptions{
				Manager:       ctx.manager,
				Current:       current,
				Item:          ctx.item,
				RepoPath:      ctx.repoPath,
				WorkspacePath: ctx.workspacePath,
				RunOptions:    ctx.opts,
				Result:        ctx.result,
			})
		}, nil
	default:
		return nil, fmt.Errorf("invalid job stage: %s", current.Stage)
	}
}

func (ctx runContext) runStageWithInterrupt(current Job, stageFn func() (Job, error), interrupts <-chan os.Signal) (Job, error) {
	stageResult := make(chan struct {
		job Job
		err error
	}, 1)
	go func() {
		job, err := stageFn()
		stageResult <- struct {
			job Job
			err error
		}{job: job, err: err}
	}()

	select {
	case <-interrupts:
		return ctx.handleInterrupt(current)
	case res := <-stageResult:
		return res.job, res.err
	}
}

func (ctx runContext) handleInterrupt(current Job) (Job, error) {
	status := StatusFailed
	updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
	return updated, errors.Join(ErrJobInterrupted, updateErr)
}

func (ctx runContext) handleStageOutcome(current, next Job, stageErr error) (Job, error) {
	if stageErr != nil {
		if next.Status == StatusAbandoned {
			ctx.result.Job = next
			return next, stageErr
		}
		updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: statusPtr(StatusFailed)}, ctx.opts.Now())
		ctx.result.Job = updated
		return updated, errors.Join(stageErr, updateErr)
	}
	if next.ID != "" {
		if next.Stage != current.Stage && ctx.opts.OnStageChange != nil {
			ctx.opts.OnStageChange(next.Stage)
		}
		current = next
		ctx.result.Job = next
	}
	return current, nil
}

func normalizeRunOptions(opts RunOptions) RunOptions {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.LoadConfig == nil {
		opts.LoadConfig = config.Load
	}
	if opts.RunTests == nil {
		opts.RunTests = RunTestCommands
	}
	if opts.RunOpencode == nil {
		opts.RunOpencode = func(runOpts opencodeRunOptions) (OpencodeRunResult, error) {
			pool, err := workspace.Open()
			if err != nil {
				return OpencodeRunResult{}, err
			}
			return runOpencodeSession(pool, runOpts)
		}
	}
	if opts.UpdateStale == nil {
		client := jj.New()
		opts.UpdateStale = client.WorkspaceUpdateStale
	}
	return opts
}

func runImplementingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions) (Job, error) {
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return Job{}, err
	}

	prompt, err := renderPromptTemplate(item, current.Feedback, "", "implement.tmpl", workspacePath)
	if err != nil {
		return Job{}, err
	}

	opencodeResult, err := opts.RunOpencode(opencodeRunOptions{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		Prompt:        prompt,
		StartedAt:     opts.Now(),
	})
	if err != nil {
		return Job{}, err
	}

	append := OpencodeSession{Purpose: "implement", ID: opencodeResult.SessionID}
	updated, err := manager.Update(current.ID, UpdateOptions{AppendOpencodeSession: &append}, opts.Now())
	if err != nil {
		return Job{}, err
	}

	if opencodeResult.ExitCode != 0 {
		return Job{}, fmt.Errorf("opencode implement failed with exit code %d", opencodeResult.ExitCode)
	}

	nextStage := StageTesting
	updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage}, opts.Now())
	if err != nil {
		return Job{}, err
	}
	return updated, nil
}

func runTestingStage(manager *Manager, current Job, repoPath, workspacePath string, opts RunOptions) (Job, error) {
	cfg, err := opts.LoadConfig(repoPath)
	if err != nil {
		return Job{}, fmt.Errorf("load config: %w", err)
	}

	results, err := opts.RunTests(workspacePath, cfg.Job.TestCommands)
	if err != nil {
		return Job{}, err
	}

	nextStage, feedback := testingStageOutcome(results)
	update := UpdateOptions{Stage: &nextStage}
	if feedback != "" {
		update.Feedback = &feedback
	} else {
		empty := ""
		update.Feedback = &empty
	}
	updated, err := manager.Update(current.ID, update, opts.Now())
	if err != nil {
		return Job{}, err
	}
	return updated, nil
}

func runReviewingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions) (Job, error) {
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return Job{}, err
	}

	prompt, err := renderPromptTemplate(item, "", "", "review.tmpl", workspacePath)
	if err != nil {
		return Job{}, err
	}

	opencodeResult, err := opts.RunOpencode(opencodeRunOptions{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		Prompt:        prompt,
		StartedAt:     opts.Now(),
	})
	if err != nil {
		return Job{}, err
	}

	append := OpencodeSession{Purpose: "review", ID: opencodeResult.SessionID}
	updated, err := manager.Update(current.ID, UpdateOptions{AppendOpencodeSession: &append}, opts.Now())
	if err != nil {
		return Job{}, err
	}

	if opencodeResult.ExitCode != 0 {
		return Job{}, fmt.Errorf("opencode review failed with exit code %d", opencodeResult.ExitCode)
	}

	fallbackFeedbackPath := filepath.Join(repoPath, feedbackFilename)
	feedback, err := readReviewFeedbackWithFallback(feedbackPath, fallbackFeedbackPath)
	if err != nil {
		return Job{}, err
	}

	switch feedback.Outcome {
	case ReviewOutcomeAccept:
		nextStage := StageCommitting
		empty := ""
		updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &empty}, opts.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	case ReviewOutcomeAbandon:
		status := StatusAbandoned
		updated, err = manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, fmt.Errorf("job abandoned")
	case ReviewOutcomeRequestChanges:
		nextStage := StageImplementing
		updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage, Feedback: &feedback.Details}, opts.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	default:
		return Job{}, ErrInvalidFeedbackFormat
	}
}

type CommittingStageOptions struct {
	Manager       *Manager
	Current       Job
	Item          todo.Todo
	RepoPath      string
	WorkspacePath string
	RunOptions    RunOptions
	Result        *RunResult
}

func runCommittingStage(opts CommittingStageOptions) (Job, error) {
	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	messagePath := filepath.Join(opts.WorkspacePath, commitMessageFilename)
	if err := removeFileIfExists(messagePath); err != nil {
		return Job{}, err
	}

	prompt, err := renderPromptTemplate(opts.Item, "", "", "commit-message.tmpl", opts.WorkspacePath)
	if err != nil {
		return Job{}, err
	}

	opencodeResult, err := opts.RunOptions.RunOpencode(opencodeRunOptions{
		RepoPath:      opts.RepoPath,
		WorkspacePath: opts.WorkspacePath,
		Prompt:        prompt,
		StartedAt:     opts.RunOptions.Now(),
	})
	if err != nil {
		return Job{}, err
	}

	append := OpencodeSession{Purpose: "commit-message", ID: opencodeResult.SessionID}
	updated, err := opts.Manager.Update(opts.Current.ID, UpdateOptions{AppendOpencodeSession: &append}, opts.RunOptions.Now())
	if err != nil {
		return Job{}, err
	}
	if opencodeResult.ExitCode != 0 {
		return Job{}, fmt.Errorf("opencode commit message failed with exit code %d", opencodeResult.ExitCode)
	}

	fallbackMessagePath := filepath.Join(opts.RepoPath, commitMessageFilename)
	message, err := readCommitMessageWithFallback(messagePath, fallbackMessagePath)
	if err != nil {
		return Job{}, err
	}

	finalMessage, err := renderPromptTemplate(opts.Item, "", message, "commit.tmpl", opts.WorkspacePath)
	if err != nil {
		return Job{}, err
	}
	opts.Result.CommitMessage = finalMessage

	client := jj.New()
	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	if err := client.Describe(opts.WorkspacePath, finalMessage); err != nil {
		return Job{}, err
	}

	status := StatusCompleted
	updated, err = opts.Manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.RunOptions.Now())
	if err != nil {
		return Job{}, err
	}
	return updated, nil
}

func testingStageOutcome(results []TestCommandResult) (Stage, string) {
	var failed []TestCommandResult
	for _, result := range results {
		if result.ExitCode != 0 {
			failed = append(failed, result)
		}
	}
	if len(failed) == 0 {
		return StageReviewing, ""
	}
	return StageImplementing, FormatTestFeedback(failed)
}

func renderPromptTemplate(item todo.Todo, feedback, message, name, workspacePath string) (string, error) {
	prompt, err := LoadPrompt(workspacePath, name)
	if err != nil {
		return "", err
	}
	return RenderPrompt(prompt, PromptData{Todo: item, Feedback: feedback, Message: message, WorkspacePath: workspacePath})
}

func readCommitMessage(path string) (string, error) {
	return readCommitMessageWithFallback(path, "")
}

func readCommitMessageWithFallback(path, fallbackPath string) (string, error) {
	data, usedPath, err := readFileWithFallback(path, fallbackPath)
	if err != nil {
		return "", fmt.Errorf("read commit message: %w", err)
	}
	removeErr := removeFileIfExists(usedPath)
	if removeErr != nil {
		removeErr = fmt.Errorf("remove commit message: %w", removeErr)
	}
	message := strings.TrimRight(string(data), "\r\n")
	if strings.TrimSpace(message) == "" {
		return "", errors.Join(fmt.Errorf("commit message is empty"), removeErr)
	}
	if removeErr != nil {
		return "", removeErr
	}
	return message, nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func updateStaleWorkspace(update func(string) error, workspacePath string) {
	if update == nil {
		return
	}
	_ = update(workspacePath)
}

func runOpencodeSession(pool *workspace.Pool, opts opencodeRunOptions) (OpencodeRunResult, error) {
	daemon, err := pool.FindOpencodeDaemon(opts.RepoPath)
	if err != nil {
		return OpencodeRunResult{}, err
	}
	if daemon.Status != workspace.OpencodeDaemonRunning {
		return OpencodeRunResult{}, fmt.Errorf("opencode daemon is not running")
	}

	sessionID, logPath, err := opencodeSessionLogPath(pool, opts.RepoPath, opts.Prompt, opts.StartedAt)
	if err != nil {
		return OpencodeRunResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return OpencodeRunResult{}, fmt.Errorf("create opencode log dir: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return OpencodeRunResult{}, fmt.Errorf("open opencode log: %w", err)
	}
	defer logFile.Close()

	created, err := pool.CreateOpencodeSession(opts.RepoPath, opts.Prompt, logPath, opts.StartedAt)
	if err != nil {
		return OpencodeRunResult{}, err
	}
	if created.ID != sessionID {
		return OpencodeRunResult{}, fmt.Errorf("opencode session id mismatch")
	}

	attachURL := workspace.DaemonAttachURL(daemon)
	runCmd := exec.Command("opencode", "run", "--attach", attachURL, opts.Prompt)
	runCmd.Dir = opts.WorkspacePath
	runCmd.Env = replaceEnvVar(os.Environ(), "PWD", opts.WorkspacePath)
	runCmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	runCmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	runCmd.Stdin = os.Stdin

	exitCode, runErr := runExitCode(runCmd)
	completedAt := time.Now()
	duration := int(completedAt.Sub(opts.StartedAt).Seconds())
	status := workspace.OpencodeSessionCompleted
	if exitCode != 0 {
		status = workspace.OpencodeSessionFailed
	}
	if _, err := pool.CompleteOpencodeSession(opts.RepoPath, sessionID, status, completedAt, &exitCode, duration); err != nil {
		return OpencodeRunResult{}, err
	}
	if runErr != nil {
		return OpencodeRunResult{}, runErr
	}
	return OpencodeRunResult{SessionID: sessionID, ExitCode: exitCode}, nil
}

func replaceEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	updated := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		updated = append(updated, entry)
	}
	updated = append(updated, prefix+value)
	return updated
}

func runExitCode(cmd *exec.Cmd) (int, error) {
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

func opencodeSessionLogPath(pool *workspace.Pool, repoPath, prompt string, startedAt time.Time) (string, string, error) {
	logDir, err := opencodeLogDir(pool, repoPath)
	if err != nil {
		return "", "", err
	}

	sessionID := workspace.GenerateOpencodeSessionID(prompt, startedAt)
	return sessionID, filepath.Join(logDir, sessionID+".log"), nil
}

func opencodeLogDir(pool *workspace.Pool, repoPath string) (string, error) {
	repoSlug, err := pool.RepoSlug(repoPath)
	if err != nil {
		return "", err
	}

	baseDir, err := opencodeBaseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, repoSlug), nil
}

func opencodeBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".local", "share", "incrementum", "opencode"), nil
}

func statusPtr(status Status) *Status {
	return &status
}

func finalizeTodo(repoPath, todoID string, status Status) error {
	switch status {
	case StatusCompleted:
		return finishTodo(repoPath, todoID)
	case StatusFailed, StatusAbandoned:
		return reopenTodo(repoPath, todoID)
	default:
		return nil
	}
}

func finishTodo(repoPath, todoID string) error {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		return err
	}
	_, err = store.Finish([]string{todoID})
	releaseErr := store.Release()
	if err != nil {
		return errors.Join(err, releaseErr)
	}
	return releaseErr
}

func reopenTodo(repoPath, todoID string) error {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		return err
	}
	_, err = store.Reopen([]string{todoID})
	releaseErr := store.Release()
	if err != nil {
		return errors.Join(err, releaseErr)
	}
	return releaseErr
}

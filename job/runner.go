package job

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/jj"
	internalopencode "github.com/amonks/incrementum/internal/opencode"
	"github.com/amonks/incrementum/todo"
	"github.com/amonks/incrementum/workspace"
)

const (
	feedbackFilename             = ".incrementum-feedback"
	commitMessageFilename        = ".incrementum-commit-message"
	opencodeSessionLookupTimeout = 5 * time.Second
)

var promptMessagePattern = regexp.MustCompile(`\{\{[^}]*\.Message[^}]*\}\}`)

// RunOptions configures job execution.
type RunOptions struct {
	OnStart         func(StartInfo)
	OnStageChange   func(Stage)
	Now             func() time.Time
	LoadConfig      func(string) (*config.Config, error)
	RunTests        func(string, []string) ([]TestCommandResult, error)
	RunOpencode     func(opencodeRunOptions) (OpencodeRunResult, error)
	CurrentCommitID func(string) (string, error)
	Commit          func(string, string) error
	UpdateStale     func(string) error
}

// RunResult captures the output of running a job.
type RunResult struct {
	Job            Job
	CommitMessage  string
	CommitMessages []string
}

// OpencodeRunResult captures output from running opencode.
type OpencodeRunResult struct {
	SessionID string
	ExitCode  int
}

type reviewScope int

const (
	reviewScopeStep reviewScope = iota
	reviewScopeProject
)

type ImplementingStageResult struct {
	Job           Job
	CommitMessage string
	Changed       bool
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
	finalJob, err := runJobStages(&runCtx, created, interrupts)
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
	reviewScope   reviewScope
	commitMessage string
	workComplete  bool
}

func runJobStages(ctx *runContext, current Job, interrupts <-chan os.Signal) (Job, error) {
	ctx.reviewScope = reviewScopeStep
	for current.Status == StatusActive {
		if current.Stage != StageImplementing {
			return current, fmt.Errorf("invalid job stage: %s", current.Stage)
		}

		next, stageErr := ctx.runStageWithInterrupt(current, ctx.runImplementingStage(current), interrupts)
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
		if ctx.workComplete {
			ctx.reviewScope = reviewScopeProject
		}

		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runTestingStage(current), interrupts)
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
		if current.Stage == StageImplementing {
			ctx.reviewScope = reviewScopeStep
			continue
		}

		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runReviewingStage(current), interrupts)
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
		if current.Stage == StageImplementing {
			ctx.reviewScope = reviewScopeStep
			continue
		}
		if ctx.reviewScope == reviewScopeProject {
			continue
		}

		next, stageErr = ctx.runStageWithInterrupt(current, ctx.runCommittingStage(current), interrupts)
		if stageErr != nil && errors.Is(stageErr, ErrJobInterrupted) {
			return next, stageErr
		}
		current, stageErr = ctx.handleStageOutcome(current, next, stageErr)
		if stageErr != nil {
			return current, stageErr
		}
	}

	return current, nil
}

func (ctx *runContext) runStageWithInterrupt(current Job, stageFn func() (Job, error), interrupts <-chan os.Signal) (Job, error) {
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

func (ctx *runContext) handleInterrupt(current Job) (Job, error) {
	status := StatusFailed
	updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
	return updated, errors.Join(ErrJobInterrupted, updateErr)
}

func (ctx *runContext) handleStageOutcome(current, next Job, stageErr error) (Job, error) {
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

func (ctx *runContext) runImplementingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		result, err := runImplementingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts)
		if err != nil {
			return Job{}, err
		}
		ctx.commitMessage = result.CommitMessage
		ctx.workComplete = !result.Changed
		return result.Job, nil
	}
}

func (ctx *runContext) runTestingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		return runTestingStage(ctx.manager, current, ctx.repoPath, ctx.workspacePath, ctx.opts)
	}
}

func (ctx *runContext) runReviewingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		return runReviewingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts, ctx.commitMessage, ctx.reviewScope)
	}
}

func (ctx *runContext) runCommittingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		return runCommittingStage(CommittingStageOptions{
			Manager:       ctx.manager,
			Current:       current,
			Item:          ctx.item,
			RepoPath:      ctx.repoPath,
			WorkspacePath: ctx.workspacePath,
			RunOptions:    ctx.opts,
			Result:        ctx.result,
			CommitMessage: ctx.commitMessage,
		})
	}
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
	if opts.CurrentCommitID == nil {
		client := jj.New()
		opts.CurrentCommitID = client.CurrentCommitID
	}
	if opts.Commit == nil {
		client := jj.New()
		opts.Commit = client.Commit
	}
	if opts.UpdateStale == nil {
		client := jj.New()
		opts.UpdateStale = client.WorkspaceUpdateStale
	}
	return opts
}

func runImplementingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions) (ImplementingStageResult, error) {
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return ImplementingStageResult{}, err
	}

	beforeCommitID, err := opts.CurrentCommitID(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	prompt, err := renderPromptTemplate(item, current.Feedback, "", "prompt-implementation.tmpl", workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	opencodeResult, err := opts.RunOpencode(opencodeRunOptions{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		Prompt:        prompt,
		StartedAt:     opts.Now(),
	})
	if err != nil {
		return ImplementingStageResult{}, err
	}

	append := OpencodeSession{Purpose: "implement", ID: opencodeResult.SessionID}
	updated, err := manager.Update(current.ID, UpdateOptions{AppendOpencodeSession: &append}, opts.Now())
	if err != nil {
		return ImplementingStageResult{}, err
	}

	if opencodeResult.ExitCode != 0 {
		return ImplementingStageResult{}, fmt.Errorf("opencode implement failed with exit code %d", opencodeResult.ExitCode)
	}

	afterCommitID, err := opts.CurrentCommitID(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	changed := beforeCommitID != afterCommitID
	message := ""
	if changed {
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		fallbackMessagePath := filepath.Join(repoPath, commitMessageFilename)
		message, err = readCommitMessageWithFallback(messagePath, fallbackMessagePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ImplementingStageResult{}, fmt.Errorf(
					"commit message missing after opencode implementation; expected opencode to write %s (or %s) because the workspace changed: %w",
					messagePath,
					fallbackMessagePath,
					err,
				)
			}
			return ImplementingStageResult{}, err
		}
	} else {
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		if err := removeFileIfExists(messagePath); err != nil {
			return ImplementingStageResult{}, err
		}
		fallbackMessagePath := filepath.Join(repoPath, commitMessageFilename)
		if err := removeFileIfExists(fallbackMessagePath); err != nil {
			return ImplementingStageResult{}, err
		}
	}

	nextStage := StageTesting
	updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage}, opts.Now())
	if err != nil {
		return ImplementingStageResult{}, err
	}
	return ImplementingStageResult{Job: updated, CommitMessage: message, Changed: changed}, nil
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

func runReviewingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions, commitMessage string, scope reviewScope) (Job, error) {
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return Job{}, err
	}

	message, err := resolveReviewCommitMessage(commitMessage, workspacePath, repoPath)
	if err != nil {
		return Job{}, err
	}

	promptName := "prompt-commit-review.tmpl"
	purpose := "review"
	if scope == reviewScopeProject {
		promptName = "prompt-project-review.tmpl"
		purpose = "project-review"
	}

	promptTemplate, err := LoadPrompt(workspacePath, promptName)
	if err != nil {
		return Job{}, err
	}
	promptTemplate = ensureCommitMessageInPrompt(promptTemplate, message)
	prompt, err := RenderPrompt(promptTemplate, PromptData{Todo: item, Feedback: "", Message: message, WorkspacePath: workspacePath})
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

	append := OpencodeSession{Purpose: purpose, ID: opencodeResult.SessionID}
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
		if scope == reviewScopeProject {
			status := StatusCompleted
			updated, err = manager.Update(updated.ID, UpdateOptions{Status: &status}, opts.Now())
			if err != nil {
				return Job{}, err
			}
			return updated, nil
		}
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
	CommitMessage string
}

func runCommittingStage(opts CommittingStageOptions) (Job, error) {
	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	message := strings.TrimSpace(opts.CommitMessage)
	if message == "" {
		return Job{}, fmt.Errorf("commit message is required")
	}

	finalMessage, err := renderPromptTemplate(opts.Item, "", message, "commit-message.tmpl", opts.WorkspacePath)
	if err != nil {
		return Job{}, err
	}
	opts.Result.CommitMessage = finalMessage
	opts.Result.CommitMessages = append(opts.Result.CommitMessages, finalMessage)

	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	if err := opts.RunOptions.Commit(opts.WorkspacePath, finalMessage); err != nil {
		return Job{}, err
	}

	nextStage := StageImplementing
	updated, err := opts.Manager.Update(opts.Current.ID, UpdateOptions{Stage: &nextStage}, opts.RunOptions.Now())
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

func ensureCommitMessageInPrompt(prompt, message string) string {
	if strings.TrimSpace(message) == "" {
		return prompt
	}
	if promptMessagePattern.MatchString(prompt) {
		return prompt
	}
	trimmed := strings.TrimRight(prompt, "\n")
	return trimmed + "\n\n<commit_message>\n{{.Message}}\n</commit_message>\n"
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

func resolveReviewCommitMessage(commitMessage, workspacePath, repoPath string) (string, error) {
	if strings.TrimSpace(commitMessage) != "" {
		return commitMessage, nil
	}
	if strings.TrimSpace(workspacePath) == "" {
		return "", nil
	}
	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	fallbackMessagePath := ""
	if strings.TrimSpace(repoPath) != "" {
		fallbackMessagePath = filepath.Join(repoPath, commitMessageFilename)
	}
	message, err := readCommitMessageWithFallback(messagePath, fallbackMessagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
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
	storage, err := opencodeStorage()
	if err != nil {
		return OpencodeRunResult{}, err
	}

	runCmd := exec.Command("opencode", "run", opts.Prompt)
	runCmd.Dir = opts.WorkspacePath
	runCmd.Env = replaceEnvVar(os.Environ(), "PWD", opts.WorkspacePath)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	if err := runCmd.Start(); err != nil {
		return OpencodeRunResult{}, err
	}

	session, sessionErr := ensureOpencodeSession(pool, storage, opts.RepoPath, opts.StartedAt, opts.Prompt)
	exitCode, runErr := runExitCode(runCmd)
	completedAt := time.Now()

	if sessionErr != nil {
		session, sessionErr = ensureOpencodeSession(pool, storage, opts.RepoPath, opts.StartedAt, opts.Prompt)
	}
	if sessionErr != nil {
		if runErr != nil {
			return OpencodeRunResult{}, errors.Join(runErr, sessionErr)
		}
		return OpencodeRunResult{}, sessionErr
	}

	duration := int(completedAt.Sub(session.StartedAt).Seconds())
	status := workspace.OpencodeSessionCompleted
	if exitCode != 0 {
		status = workspace.OpencodeSessionFailed
	}
	if _, err := pool.CompleteOpencodeSession(opts.RepoPath, session.ID, status, completedAt, &exitCode, duration); err != nil {
		return OpencodeRunResult{}, err
	}
	if runErr != nil {
		return OpencodeRunResult{}, runErr
	}
	return OpencodeRunResult{SessionID: session.ID, ExitCode: exitCode}, nil
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
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

func ensureOpencodeSession(pool *workspace.Pool, storage internalopencode.Storage, repoPath string, startedAt time.Time, prompt string) (workspace.OpencodeSession, error) {
	metadata, err := storage.FindSessionForRunWithRetry(repoPath, startedAt, prompt, opencodeSessionLookupTimeout)
	if err != nil {
		return workspace.OpencodeSession{}, err
	}

	sessionStartedAt := startedAt
	if !metadata.CreatedAt.IsZero() {
		sessionStartedAt = metadata.CreatedAt
	}

	if existing, err := pool.FindOpencodeSession(repoPath, metadata.ID); err == nil {
		if existing.Status == workspace.OpencodeSessionActive {
			return existing, nil
		}
	} else if !errors.Is(err, workspace.ErrOpencodeSessionNotFound) {
		return workspace.OpencodeSession{}, err
	}

	return pool.CreateOpencodeSession(repoPath, metadata.ID, prompt, sessionStartedAt)
}

func opencodeStorage() (internalopencode.Storage, error) {
	root, err := internalopencode.DefaultRoot()
	if err != nil {
		return internalopencode.Storage{}, err
	}
	return internalopencode.Storage{Root: root}, nil
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

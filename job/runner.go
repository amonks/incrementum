package job

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/config"
	"github.com/amonks/incrementum/internal/jj"
	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/opencode"
	"github.com/amonks/incrementum/todo"
)

const (
	feedbackFilename      = ".incrementum-feedback"
	commitMessageFilename = ".incrementum-commit-message"
	opencodeConfigEnvVar  = "OPENCODE_CONFIG_CONTENT"
	opencodeConfigContent = `{"permission":{"question":"deny"}}`
)

var promptMessagePattern = regexp.MustCompile(`\{\{[^}]*\.(Message|CommitMessageBlock)[^}]*\}\}`)

// RunOptions configures job execution.
type RunOptions struct {
	OnStart       func(StartInfo)
	OnStageChange func(Stage)
	// EventStream receives job events as they are recorded. The channel is closed
	// when Run completes.
	EventStream chan<- Event
	// WorkspacePath is the path to run the job from.
	// Defaults to repoPath when empty.
	WorkspacePath string
	// Interrupts delivers signals that should interrupt the job.
	// If nil, os.Interrupt is used.
	Interrupts <-chan os.Signal
	Now        func() time.Time
	LoadConfig func(string) (*config.Config, error)
	// Config provides loaded configuration for the job run.
	// When nil, LoadConfig is used.
	Config      *config.Config
	RunTests    func(string, []string) ([]TestCommandResult, error)
	RunOpencode func(opencodeRunOptions) (OpencodeRunResult, error)
	// OpencodeAgent overrides agent selection for all stages when set.
	OpencodeAgent       string
	CurrentCommitID     func(string) (string, error)
	CurrentChangeEmpty  func(string) (bool, error)
	DiffStat            func(string, string, string) (string, error)
	CommitIDAt          func(string, string) (string, error)
	Commit              func(string, string) error
	RestoreWorkspace    func(string, string) error
	UpdateStale         func(string) error
	Snapshot            func(string) error
	OpencodeTranscripts func(string, []OpencodeSession) ([]OpencodeTranscript, error)
	EventLog            *EventLog
	EventLogOptions     EventLogOptions
	Logger              Logger
}

// RunResult captures the output of running a job.
type RunResult struct {
	Job           Job
	CommitMessage string
	CommitLog     []CommitLogEntry
}

// OpencodeRunResult captures output from running opencode.
type OpencodeRunResult struct {
	SessionID    string
	ExitCode     int
	ServeCommand string
	RunCommand   string
	Stderr       string
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
	Agent         string
	StartedAt     time.Time
	EventLog      *EventLog
	Env           []string
}

// Run creates and executes a job for the given todo.
func Run(repoPath, todoID string, opts RunOptions) (*RunResult, error) {
	if internalstrings.IsBlank(todoID) {
		return nil, fmt.Errorf("todo id is required")
	}

	opts = normalizeRunOptions(opts)
	if opts.EventStream != nil {
		defer close(opts.EventStream)
	}
	result := &RunResult{}
	repoPath = filepath.Clean(repoPath)
	if abs, absErr := filepath.Abs(repoPath); absErr == nil {
		repoPath = abs
	}
	if opts.Config == nil {
		cfg, err := opts.LoadConfig(repoPath)
		if err != nil {
			return result, fmt.Errorf("load config: %w", err)
		}
		if cfg == nil {
			cfg = &config.Config{}
		}
		opts.Config = cfg
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
	if !internalstrings.IsBlank(opts.WorkspacePath) {
		workspacePath = opts.WorkspacePath
	}
	workspacePath = filepath.Clean(workspacePath)
	workspaceAbs := workspacePath
	if abs, absErr := filepath.Abs(workspacePath); absErr == nil {
		workspaceAbs = abs
	}
	workspacePath = workspaceAbs
	manager, err := Open(repoPath, OpenOptions{})
	if err != nil {
		reopenErr := reopenTodo(repoPath, item.ID)
		return result, errors.Join(err, reopenErr)
	}

	implementModel := resolveOpencodeAgentForPurpose(opts.Config, opts.OpencodeAgent, "implement", item)
	codeReviewModel := resolveOpencodeAgentForPurpose(opts.Config, opts.OpencodeAgent, "review", item)
	projectReviewModel := resolveOpencodeAgentForPurpose(opts.Config, opts.OpencodeAgent, "project-review", item)
	created, err := manager.Create(item.ID, startedAt, CreateOptions{
		Agent:               implementModel,
		ImplementationModel: implementModel,
		CodeReviewModel:     codeReviewModel,
		ProjectReviewModel:  projectReviewModel,
	})
	if err != nil {
		reopenErr := reopenTodo(repoPath, item.ID)
		return result, errors.Join(err, reopenErr)
	}
	result.Job = created

	if opts.OnStart != nil {
		opts.OnStart(StartInfo{
			JobID:   created.ID,
			Workdir: workspaceAbs,
			Todo:    item,
		})
	}

	createdEventLog := false
	if opts.EventLog == nil {
		eventLog, err := OpenEventLog(created.ID, opts.EventLogOptions)
		if err != nil {
			status := StatusFailed
			updated, updateErr := manager.Update(created.ID, UpdateOptions{Status: &status}, opts.Now())
			result.Job = updated
			finalizeErr := finalizeTodo(repoPath, item.ID, StatusFailed)
			return result, errors.Join(err, updateErr, finalizeErr)
		}
		opts.EventLog = eventLog
		createdEventLog = true
	}
	if createdEventLog {
		defer func() {
			_ = opts.EventLog.Close()
		}()
	}
	if opts.EventStream != nil {
		opts.EventLog.SetStream(opts.EventStream)
	}
	if err := appendJobEvent(opts.EventLog, jobEventStage, stageEventData{Stage: created.Stage}); err != nil {
		status := StatusFailed
		updated, updateErr := manager.Update(created.ID, UpdateOptions{Status: &status}, opts.Now())
		result.Job = updated
		finalizeErr := finalizeTodo(repoPath, item.ID, StatusFailed)
		return result, errors.Join(err, updateErr, finalizeErr)
	}
	if opts.OnStageChange != nil {
		opts.OnStageChange(created.Stage)
	}

	interrupts := opts.Interrupts
	if interrupts == nil {
		localInterrupts := make(chan os.Signal, 1)
		signal.Notify(localInterrupts, os.Interrupt)
		defer signal.Stop(localInterrupts)
		interrupts = localInterrupts
	}

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

		if current.Stage == StageTesting {
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
		status := StatusFailed
		updated, updateErr := ctx.manager.Update(current.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
		ctx.result.Job = updated
		return updated, errors.Join(stageErr, updateErr)
	}
	if next.ID != "" {
		if next.Stage != current.Stage {
			if err := appendJobEvent(ctx.opts.EventLog, jobEventStage, stageEventData{Stage: next.Stage}); err != nil {
				status := StatusFailed
				updated, updateErr := ctx.manager.Update(next.ID, UpdateOptions{Status: &status}, ctx.opts.Now())
				ctx.result.Job = updated
				return updated, errors.Join(err, updateErr)
			}
			if ctx.opts.OnStageChange != nil {
				ctx.opts.OnStageChange(next.Stage)
			}
		}
		current = next
		ctx.result.Job = next
	}
	return current, nil
}

func (ctx *runContext) runImplementingStage(current Job) func() (Job, error) {
	return func() (Job, error) {
		result, err := runImplementingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts, ctx.result.CommitLog, ctx.commitMessage)
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
		return runReviewingStage(ctx.manager, current, ctx.item, ctx.repoPath, ctx.workspacePath, ctx.opts, ctx.commitMessage, ctx.result.CommitLog, ctx.reviewScope)
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
			store, err := opencode.Open()
			if err != nil {
				return OpencodeRunResult{}, err
			}
			return runOpencodeSession(store, runOpts)
		}
	}
	if opts.CurrentCommitID == nil {
		client := jj.New()
		opts.CurrentCommitID = client.CurrentCommitID
	}
	if opts.CurrentChangeEmpty == nil {
		client := jj.New()
		opts.CurrentChangeEmpty = client.CurrentChangeEmpty
	}
	if opts.DiffStat == nil {
		client := jj.New()
		opts.DiffStat = client.DiffStat
	}
	if opts.CommitIDAt == nil {
		client := jj.New()
		opts.CommitIDAt = client.CommitIDAt
	}
	if opts.Commit == nil {
		client := jj.New()
		opts.Commit = client.Commit
	}
	if opts.RestoreWorkspace == nil {
		client := jj.New()
		opts.RestoreWorkspace = client.Edit
	}
	if opts.UpdateStale == nil {
		client := jj.New()
		opts.UpdateStale = client.WorkspaceUpdateStale
	}
	if opts.Snapshot == nil {
		client := jj.New()
		opts.Snapshot = client.Snapshot
	}
	if opts.OpencodeTranscripts == nil {
		opts.OpencodeTranscripts = opencodeTranscripts
	}
	opts.Logger = resolveLogger(opts.Logger)
	return opts
}

func resolveOpencodeAgentForPurpose(cfg *config.Config, override, purpose string, item todo.Todo) string {
	if !internalstrings.IsBlank(override) {
		return internalstrings.TrimSpace(override)
	}
	modelOverride := todoModelForPurpose(item, purpose)
	if !internalstrings.IsBlank(modelOverride) {
		return internalstrings.TrimSpace(modelOverride)
	}
	if cfg == nil {
		return ""
	}
	model := ""
	switch purpose {
	case "implement":
		model = cfg.Job.ImplementationModel
	case "review":
		model = cfg.Job.CodeReviewModel
	case "project-review":
		model = cfg.Job.ProjectReviewModel
	default:
		model = cfg.Job.Agent
	}
	if internalstrings.IsBlank(model) {
		model = cfg.Job.Agent
	}
	return internalstrings.TrimSpace(model)
}

func todoModelForPurpose(item todo.Todo, purpose string) string {
	switch purpose {
	case "implement":
		return item.ImplementationModel
	case "review":
		return item.CodeReviewModel
	case "project-review":
		return item.ProjectReviewModel
	default:
		return ""
	}
}

func runImplementingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions, commitLog []CommitLogEntry, previousMessage string) (ImplementingStageResult, error) {
	logger := resolveLogger(opts.Logger)
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return ImplementingStageResult{}, err
	}

	beforeCommitID, err := opts.CurrentCommitID(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	promptName := "prompt-implementation.tmpl"
	if !internalstrings.IsBlank(current.Feedback) {
		promptName = "prompt-feedback.tmpl"
	}
	prompt, err := renderPromptTemplate(item, current.Feedback, previousMessage, commitLog, nil, promptName, workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}
	if err := appendJobEvent(opts.EventLog, jobEventPrompt, promptEventData{Purpose: "implement", Template: promptName, Prompt: prompt}); err != nil {
		return ImplementingStageResult{}, err
	}

	updated := current
	agent := resolveOpencodeAgentForPurpose(opts.Config, opts.OpencodeAgent, "implement", item)
	runAttempt := func() (OpencodeRunResult, error) {
		result, err := runOpencodeWithEvents(opts, opencodeRunOptions{
			RepoPath:      repoPath,
			WorkspacePath: workspacePath,
			Prompt:        prompt,
			Agent:         agent,
			StartedAt:     opts.Now(),
			EventLog:      opts.EventLog,
			Env:           applyOpencodeConfigEnv(nil),
		}, "implement")
		if err != nil {
			return OpencodeRunResult{}, err
		}

		append := OpencodeSession{Purpose: "implement", ID: result.SessionID}
		updated, err = manager.Update(updated.ID, UpdateOptions{AppendOpencodeSession: &append}, opts.Now())
		if err != nil {
			return OpencodeRunResult{}, err
		}
		transcript := loadOpencodeTranscript(opts.OpencodeTranscripts, repoPath, append)
		if !internalstrings.IsBlank(transcript) {
			if err := appendJobEvent(opts.EventLog, jobEventTranscript, transcriptEventData{Purpose: "implement", Transcript: transcript}); err != nil {
				return OpencodeRunResult{}, err
			}
		}
		logger.Prompt(PromptLog{Purpose: "implement", Template: promptName, Prompt: prompt, Transcript: transcript})
		return result, nil
	}

	opencodeResult, err := runAttempt()
	if err != nil {
		return ImplementingStageResult{}, err
	}

	retryCount := 0
	for opencodeResult.ExitCode != 0 {
		afterCommitID := ""
		var afterCommitErr error
		if opts.CurrentCommitID != nil && !internalstrings.IsBlank(workspacePath) {
			afterCommitID, afterCommitErr = opts.CurrentCommitID(workspacePath)
		}
		restored := false
		var restoreErr error
		if opencodeResult.ExitCode < 0 && afterCommitErr == nil && afterCommitID != "" && beforeCommitID != "" && afterCommitID != beforeCommitID {
			if opts.RestoreWorkspace != nil {
				restoreErr = opts.RestoreWorkspace(workspacePath, beforeCommitID)
				if restoreErr == nil {
					restored = true
				}
			}
		}
		if restored && retryCount == 0 {
			retryCount++
			opencodeResult, err = runAttempt()
			if err != nil {
				return ImplementingStageResult{}, err
			}
			continue
		}
		return ImplementingStageResult{}, errors.New(buildOpencodeFailureMessage("implement", promptName, opencodeResult, opencodeRunOptions{
			RepoPath:      repoPath,
			WorkspacePath: workspacePath,
			Prompt:        prompt,
			Agent:         agent,
		}, beforeCommitID, afterCommitID, afterCommitErr, restored, restoreErr, retryCount))
	}

	afterCommitID, err := opts.CurrentCommitID(workspacePath)
	if err != nil {
		return ImplementingStageResult{}, err
	}

	changed := beforeCommitID != afterCommitID
	if changed {
		if opts.CurrentChangeEmpty == nil {
			return ImplementingStageResult{}, fmt.Errorf("current change empty check is required")
		}
		empty, err := opts.CurrentChangeEmpty(workspacePath)
		if err != nil {
			return ImplementingStageResult{}, err
		}
		if empty {
			changed = false
		}
	}
	message := ""
	if changed {
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		message, err = readCommitMessage(messagePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ImplementingStageResult{}, fmt.Errorf(
					"commit message missing after opencode implementation; opencode session %s was instructed to write %s because the workspace changed from %s to %s: %w",
					opencodeResult.SessionID,
					messagePath,
					beforeCommitID,
					afterCommitID,
					err,
				)
			}
			return ImplementingStageResult{}, err
		}
		logger.CommitMessage(CommitMessageLog{Label: "Draft", Message: message})
		if err := appendJobEvent(opts.EventLog, jobEventCommitMessage, commitMessageEventData{Label: "Draft", Message: message}); err != nil {
			return ImplementingStageResult{}, err
		}
	} else {
		messagePath := filepath.Join(workspacePath, commitMessageFilename)
		if err := removeFileIfExists(messagePath); err != nil {
			return ImplementingStageResult{}, err
		}
	}

	nextStage := StageTesting
	if !changed {
		nextStage = StageReviewing
	}
	updated, err = manager.Update(updated.ID, UpdateOptions{Stage: &nextStage}, opts.Now())
	if err != nil {
		return ImplementingStageResult{}, err
	}
	return ImplementingStageResult{Job: updated, CommitMessage: message, Changed: changed}, nil
}

func runTestingStage(manager *Manager, current Job, repoPath, workspacePath string, opts RunOptions) (Job, error) {
	logger := resolveLogger(opts.Logger)
	cfg := opts.Config
	if cfg == nil {
		var err error
		cfg, err = opts.LoadConfig(repoPath)
		if err != nil {
			return Job{}, fmt.Errorf("load config: %w", err)
		}
	}
	if len(cfg.Job.TestCommands) < 1 {
		return Job{}, fmt.Errorf("job test-commands must be configured")
	}

	results, err := opts.RunTests(workspacePath, cfg.Job.TestCommands)
	if err != nil {
		return Job{}, err
	}
	logger.Tests(TestLog{Results: results})
	if err := appendJobEvent(opts.EventLog, jobEventTests, buildTestsEventData(results)); err != nil {
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

func runReviewingStage(manager *Manager, current Job, item todo.Todo, repoPath, workspacePath string, opts RunOptions, commitMessage string, commitLog []CommitLogEntry, scope reviewScope) (Job, error) {
	logger := resolveLogger(opts.Logger)
	updateStaleWorkspace(opts.UpdateStale, workspacePath)
	feedbackPath := filepath.Join(workspacePath, feedbackFilename)
	if err := removeFileIfExists(feedbackPath); err != nil {
		return Job{}, err
	}

	message, err := resolveReviewCommitMessage(commitMessage, workspacePath, scope == reviewScopeStep)
	if err != nil {
		return Job{}, err
	}

	promptName := "prompt-commit-review.tmpl"
	purpose := "review"
	if scope == reviewScopeProject {
		promptName = "prompt-project-review.tmpl"
		purpose = "project-review"
	}
	agent := resolveOpencodeAgentForPurpose(opts.Config, opts.OpencodeAgent, purpose, item)

	promptTemplate, err := LoadPrompt(workspacePath, promptName)
	if err != nil {
		return Job{}, err
	}
	promptTemplate = ensureCommitMessageInPrompt(promptTemplate, message)
	prompt, err := RenderPrompt(workspacePath, promptTemplate, newPromptData(item, "", message, commitLog, nil, workspacePath))
	if err != nil {
		return Job{}, err
	}
	if err := appendJobEvent(opts.EventLog, jobEventPrompt, promptEventData{Purpose: purpose, Template: promptName, Prompt: prompt}); err != nil {
		return Job{}, err
	}

	opencodeResult, err := runOpencodeWithEvents(opts, opencodeRunOptions{
		RepoPath:      repoPath,
		WorkspacePath: workspacePath,
		Prompt:        prompt,
		Agent:         agent,
		StartedAt:     opts.Now(),
		EventLog:      opts.EventLog,
		Env:           applyOpencodeConfigEnv(nil),
	}, purpose)
	if err != nil {
		return Job{}, err
	}

	append := OpencodeSession{Purpose: purpose, ID: opencodeResult.SessionID}
	updated, err := manager.Update(current.ID, UpdateOptions{AppendOpencodeSession: &append}, opts.Now())
	if err != nil {
		return Job{}, err
	}
	transcript := loadOpencodeTranscript(opts.OpencodeTranscripts, repoPath, append)
	if !internalstrings.IsBlank(transcript) {
		if err := appendJobEvent(opts.EventLog, jobEventTranscript, transcriptEventData{Purpose: purpose, Transcript: transcript}); err != nil {
			return Job{}, err
		}
	}
	logger.Prompt(PromptLog{Purpose: purpose, Template: promptName, Prompt: prompt, Transcript: transcript})

	if opencodeResult.ExitCode != 0 {
		return Job{}, fmt.Errorf("opencode review failed with exit code %d", opencodeResult.ExitCode)
	}

	feedback, err := ReadReviewFeedback(feedbackPath)
	if err != nil {
		return Job{}, err
	}
	logger.Review(ReviewLog{Purpose: purpose, Feedback: feedback})
	if err := appendJobEvent(opts.EventLog, jobEventReview, reviewEventData{Purpose: purpose, Outcome: feedback.Outcome, Details: feedback.Details}); err != nil {
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
	logger := resolveLogger(opts.RunOptions.Logger)
	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	if opts.RunOptions.DiffStat == nil {
		return Job{}, fmt.Errorf("diff stat is required")
	}
	diffStat, err := opts.RunOptions.DiffStat(opts.WorkspacePath, "@-", "@")
	if err != nil {
		return Job{}, err
	}
	if !diffStatHasChanges(diffStat) {
		nextStage := StageImplementing
		updated, err := opts.Manager.Update(opts.Current.ID, UpdateOptions{Stage: &nextStage}, opts.RunOptions.Now())
		if err != nil {
			return Job{}, err
		}
		return updated, nil
	}
	message := internalstrings.TrimSpace(opts.CommitMessage)
	if message == "" {
		return Job{}, fmt.Errorf("commit message is required")
	}

	finalMessage := formatCommitMessage(opts.Item, message)
	logMessage := formatCommitMessageWithWidth(opts.Item, message, lineWidth-subdocumentIndent)
	opts.Result.CommitMessage = finalMessage
	logger.CommitMessage(CommitMessageLog{Label: "Final", Message: logMessage, Preformatted: true})
	if err := appendJobEvent(opts.RunOptions.EventLog, jobEventCommitMessage, commitMessageEventData{Label: "Final", Message: logMessage, Preformatted: true}); err != nil {
		return Job{}, err
	}

	updateStaleWorkspace(opts.RunOptions.UpdateStale, opts.WorkspacePath)
	if err := opts.RunOptions.Commit(opts.WorkspacePath, finalMessage); err != nil {
		return Job{}, err
	}

	commitID, err := opts.RunOptions.CommitIDAt(opts.WorkspacePath, "@-")
	if err != nil {
		return Job{}, err
	}
	opts.Result.CommitLog = append(opts.Result.CommitLog, CommitLogEntry{ID: commitID, Message: finalMessage})

	nextStage := StageImplementing
	updated, err := opts.Manager.Update(opts.Current.ID, UpdateOptions{Stage: &nextStage}, opts.RunOptions.Now())
	if err != nil {
		return Job{}, err
	}
	return updated, nil
}

type opencodeTranscriptEntry struct {
	Purpose    string
	Session    opencode.OpencodeSession
	Transcript string
}

func loadOpencodeTranscript(fetch func(string, []OpencodeSession) ([]OpencodeTranscript, error), repoPath string, session OpencodeSession) string {
	if fetch == nil {
		return ""
	}
	transcripts, err := fetch(repoPath, []OpencodeSession{session})
	if err != nil || len(transcripts) == 0 {
		return ""
	}
	return transcripts[0].Transcript
}

func opencodeTranscripts(repoPath string, sessions []OpencodeSession) ([]OpencodeTranscript, error) {
	if len(sessions) == 0 {
		return nil, nil
	}

	store, err := opencode.Open()
	if err != nil {
		return nil, err
	}

	entries := make([]opencodeTranscriptEntry, 0, len(sessions))
	for _, session := range sessions {
		opencodeSession, err := store.FindSession(repoPath, session.ID)
		if err != nil {
			return nil, err
		}
		transcript, err := store.TranscriptSnapshot(opencodeSession.ID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, opencodeTranscriptEntry{Purpose: session.Purpose, Session: opencodeSession, Transcript: transcript})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Session.StartedAt.Equal(entries[j].Session.StartedAt) {
			return entries[i].Session.ID < entries[j].Session.ID
		}
		return entries[i].Session.StartedAt.Before(entries[j].Session.StartedAt)
	})

	transcripts := make([]OpencodeTranscript, 0, len(entries))
	for _, entry := range entries {
		text := internalstrings.TrimTrailingNewlines(entry.Transcript)
		if text == "" {
			text = "-"
		}
		transcripts = append(transcripts, OpencodeTranscript{Purpose: entry.Purpose, ID: entry.Session.ID, Transcript: text})
	}
	return transcripts, nil
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
	return StageImplementing, FormatTestFeedback(results)
}

func diffStatHasChanges(diffStat string) bool {
	lines := strings.Split(diffStat, "\n")
	seenChangeLine := false
	seenSummary := false
	changedSummary := false
	for _, line := range lines {
		line = internalstrings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "No changes") {
			return false
		}
		if strings.Contains(line, " file changed") || strings.Contains(line, " files changed") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				count, err := strconv.Atoi(fields[0])
				if err == nil {
					seenSummary = true
					changedSummary = count != 0
				}
			}
			continue
		}
		if strings.Contains(line, " | ") {
			seenChangeLine = true
		}
	}
	if seenSummary {
		return changedSummary || seenChangeLine
	}
	return seenChangeLine
}

func renderPromptTemplate(item todo.Todo, feedback, message string, commitLog []CommitLogEntry, transcripts []OpencodeTranscript, name, workspacePath string) (string, error) {
	prompt, err := LoadPrompt(workspacePath, name)
	if err != nil {
		return "", err
	}
	return RenderPrompt(workspacePath, prompt, newPromptData(item, feedback, message, commitLog, transcripts, workspacePath))
}

func runOpencodeWithEvents(opts RunOptions, runOpts opencodeRunOptions, purpose string) (OpencodeRunResult, error) {
	snapshotWorkspace(opts.Snapshot, runOpts.WorkspacePath)
	if err := appendJobEvent(opts.EventLog, jobEventOpencodeStart, opencodeStartEventData{Purpose: purpose}); err != nil {
		return OpencodeRunResult{}, err
	}
	result, err := opts.RunOpencode(runOpts)
	if err != nil {
		logErr := appendJobEvent(opts.EventLog, jobEventOpencodeError, opencodeErrorEventData{Purpose: purpose, Error: err.Error()})
		if logErr != nil {
			return OpencodeRunResult{}, errors.Join(err, logErr)
		}
		return OpencodeRunResult{}, err
	}
	if err := appendJobEvent(opts.EventLog, jobEventOpencodeEnd, opencodeEndEventData{Purpose: purpose, SessionID: result.SessionID, ExitCode: result.ExitCode}); err != nil {
		return OpencodeRunResult{}, err
	}
	return result, nil
}

func buildOpencodeFailureMessage(purpose, promptName string, result OpencodeRunResult, runOpts opencodeRunOptions, beforeCommitID, afterCommitID string, afterCommitErr error, restored bool, restoreErr error, retryCount int) string {
	parts := []string{}
	if !internalstrings.IsBlank(result.SessionID) {
		parts = append(parts, fmt.Sprintf("session %s", result.SessionID))
	}
	if !internalstrings.IsBlank(runOpts.Agent) {
		parts = append(parts, fmt.Sprintf("agent %q", runOpts.Agent))
	}
	if !internalstrings.IsBlank(promptName) {
		parts = append(parts, fmt.Sprintf("prompt %s", promptName))
	}
	if !internalstrings.IsBlank(result.RunCommand) {
		parts = append(parts, fmt.Sprintf("run %s", result.RunCommand))
	}
	if !internalstrings.IsBlank(result.ServeCommand) {
		parts = append(parts, fmt.Sprintf("serve %s", result.ServeCommand))
	}
	if !internalstrings.IsBlank(runOpts.RepoPath) {
		parts = append(parts, fmt.Sprintf("repo %s", runOpts.RepoPath))
	}
	if !internalstrings.IsBlank(runOpts.WorkspacePath) {
		parts = append(parts, fmt.Sprintf("workspace %s", runOpts.WorkspacePath))
	}
	if !internalstrings.IsBlank(beforeCommitID) {
		parts = append(parts, fmt.Sprintf("before %s", beforeCommitID))
	}
	if !internalstrings.IsBlank(afterCommitID) {
		parts = append(parts, fmt.Sprintf("after %s", afterCommitID))
	}
	if afterCommitErr != nil {
		parts = append(parts, fmt.Sprintf("after_commit_error %v", afterCommitErr))
	}
	if restored {
		parts = append(parts, fmt.Sprintf("restored %s", beforeCommitID))
	}
	if restoreErr != nil {
		parts = append(parts, fmt.Sprintf("restore_error %v", restoreErr))
	}
	if retryCount > 0 {
		parts = append(parts, fmt.Sprintf("retry %d", retryCount))
	}
	if !internalstrings.IsBlank(result.Stderr) {
		parts = append(parts, fmt.Sprintf("stderr: %s", internalstrings.TrimSpace(result.Stderr)))
	}
	message := fmt.Sprintf("opencode %s failed with exit code %d", purpose, result.ExitCode)
	if result.ExitCode < 0 {
		message += " (process did not exit cleanly)"
	}
	if len(parts) == 0 {
		return message
	}
	return fmt.Sprintf("%s: %s", message, strings.Join(parts, ", "))
}

func ensureCommitMessageInPrompt(prompt, message string) string {
	if internalstrings.IsBlank(message) {
		return prompt
	}
	if promptMessagePattern.MatchString(prompt) {
		return prompt
	}
	trimmed := internalstrings.TrimTrailingNewlines(prompt)
	return trimmed + "\n\n{{.CommitMessageBlock}}\n"
}

type commitMessageMissingError struct {
	Path string
	Err  error
}

func (err commitMessageMissingError) Error() string {
	return fmt.Sprintf("commit message missing; expected at %s: %v", err.Path, err.Err)
}

func (err commitMessageMissingError) Unwrap() error {
	return err.Err
}

func readCommitMessage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", commitMessageMissingError{Path: path, Err: err}
		}
		return "", fmt.Errorf("read commit message: %w", err)
	}
	removeErr := removeFileIfExists(path)
	if removeErr != nil {
		removeErr = fmt.Errorf("remove commit message: %w", removeErr)
	}
	message := normalizeCommitMessage(string(data))
	if internalstrings.IsBlank(message) {
		return "", errors.Join(fmt.Errorf("commit message is empty"), removeErr)
	}
	if removeErr != nil {
		return "", removeErr
	}
	return message, nil
}

func resolveReviewCommitMessage(commitMessage, workspacePath string, requireMessage bool) (string, error) {
	if !internalstrings.IsBlank(commitMessage) {
		return commitMessage, nil
	}
	if internalstrings.IsBlank(workspacePath) {
		return "", nil
	}
	messagePath := filepath.Join(workspacePath, commitMessageFilename)
	message, err := readCommitMessage(messagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !requireMessage {
				return "", nil
			}
			return "", fmt.Errorf(
				"commit message missing before opencode review; opencode implementation was instructed to write %s: %w",
				messagePath,
				err,
			)
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

func snapshotWorkspace(snapshot func(string) error, workspacePath string) {
	if snapshot == nil {
		return
	}
	_ = snapshot(workspacePath)
}

func applyOpencodeConfigEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	return replaceEnvVar(env, opencodeConfigEnvVar, opencodeConfigContent)
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

func runOpencodeSession(store *opencode.Store, opts opencodeRunOptions) (OpencodeRunResult, error) {
	var stderrBuf strings.Builder
	handle, err := store.Run(opencode.RunOptions{
		RepoPath:  opts.RepoPath,
		WorkDir:   opts.WorkspacePath,
		Prompt:    opts.Prompt,
		Agent:     opts.Agent,
		StartedAt: opts.StartedAt,
		Stdout:    io.Discard,
		Stderr:    &stderrBuf,
		Env:       applyOpencodeConfigEnv(opts.Env),
	})
	if err != nil {
		return OpencodeRunResult{}, err
	}

	eventErrCh := recordOpencodeEvents(opts.EventLog, handle.Events)
	result, err := handle.Wait()
	eventErr := <-eventErrCh
	if err != nil {
		return OpencodeRunResult{}, errors.Join(err, eventErr)
	}
	if eventErr != nil {
		return OpencodeRunResult{}, eventErr
	}
	return OpencodeRunResult{
		SessionID:    result.SessionID,
		ExitCode:     result.ExitCode,
		ServeCommand: result.ServeCommand,
		RunCommand:   result.RunCommand,
		Stderr:       stderrBuf.String(),
	}, nil
}

func recordOpencodeEvents(log *EventLog, events <-chan opencode.Event) <-chan error {
	done := make(chan error, 1)
	if events == nil {
		done <- nil
		return done
	}
	go func() {
		var recordErr error
		for event := range events {
			if log == nil || recordErr != nil {
				continue
			}
			recordErr = log.Append(Event{ID: event.ID, Name: event.Name, Data: event.Data})
		}
		done <- recordErr
	}()
	return done
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

func updateTodoStatus(repoPath, todoID string, update func(*todo.Store, string) ([]todo.Todo, error)) error {
	store, err := todo.Open(repoPath, todo.OpenOptions{CreateIfMissing: false, PromptToCreate: false})
	if err != nil {
		return err
	}
	_, err = update(store, todoID)
	releaseErr := store.Release()
	if err != nil {
		return errors.Join(err, releaseErr)
	}
	return releaseErr
}

func finishTodo(repoPath, todoID string) error {
	return updateTodoStatus(repoPath, todoID, func(store *todo.Store, id string) ([]todo.Todo, error) {
		return store.Finish([]string{id})
	})
}

func reopenTodo(repoPath, todoID string) error {
	return updateTodoStatus(repoPath, todoID, func(store *todo.Store, id string) ([]todo.Todo, error) {
		return store.Reopen([]string{id})
	})
}

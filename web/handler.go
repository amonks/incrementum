package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/internal/todoenv"
	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
)

// Options configures the swarm web handler.
type Options struct {
	BaseURL string
}

// Handler serves the swarm web client.
type Handler struct {
	baseURL   string
	client    *http.Client
	mux       *http.ServeMux
	templates *templateWrapper

	mu        sync.Mutex
	todos     []todo.Todo
	jobs      []job.Job
	todoDraft *todoFormDraft
	jobDraft  *jobFormDraft
}

// NewHandler creates a new web handler.
func NewHandler(opts Options) *Handler {
	handler := &Handler{
		baseURL:   internalstrings.TrimTrailingSlash(opts.BaseURL),
		client:    &http.Client{},
		templates: newTemplateWrapper(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/web/todos", handler.handleTodos)
	mux.HandleFunc("/web/todos/create", handler.handleTodosCreate)
	mux.HandleFunc("/web/todos/update", handler.handleTodosUpdate)
	mux.HandleFunc("/web/jobs", handler.handleJobs)
	mux.HandleFunc("/web/jobs/start", handler.handleJobsStart)
	mux.HandleFunc("/web/jobs/kill", handler.handleJobsKill)
	mux.HandleFunc("/web/jobs/refresh", handler.handleJobsRefresh)
	handler.mux = mux

	if handler.baseURL != "" {
		go handler.seed(handler.baseURL)
	}
	return handler
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type templateWrapper struct {
	tmpl *template.Template
}

func newTemplateWrapper() *templateWrapper {
	return &templateWrapper{tmpl: newTemplates()}
}

func (tw *templateWrapper) Render(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tw.tmpl.ExecuteTemplate(w, "page", data)
}

type selectOption struct {
	Value string
	Label string
}

type pageData struct {
	ActiveTab       string
	Todos           []todo.Todo
	Jobs            []job.Job
	SelectedTodo    *todo.Todo
	SelectedJob     *job.Job
	SelectedTodoID  string
	SelectedJobID   string
	Create          bool
	TodoForm        todoFormValues
	JobLogHTML      template.HTML
	TodoError       string
	JobError        string
	StatusOptions   []selectOption
	PriorityOptions []selectOption
	TypeOptions     []selectOption
}

type todoFormValues struct {
	Title               string
	Description         string
	Status              string
	Priority            string
	Type                string
	ImplementationModel string
	CodeReviewModel     string
	ProjectReviewModel  string
}

type todoFormDraft struct {
	mode      string
	id        string
	err       string
	values    todoFormValues
	hasValues bool
}

type jobFormDraft struct {
	id  string
	err string
}

func (h *Handler) handleTodos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	baseURL := h.requestBaseURL(r)
	todos, err := h.refreshTodos(r.Context(), baseURL)
	fetchError := ""
	if err != nil {
		fetchError = err.Error()
	}

	createMode := r.URL.Query().Get("create") == "1"
	selectedID := trimmedQueryValue(r, "id")
	selectedTodo := (*todo.Todo)(nil)
	if !createMode {
		selectedTodo = selectTodo(todos, selectedID)
		if selectedTodo == nil && len(todos) > 0 {
			selectedTodo = &todos[0]
			selectedID = selectedTodo.ID
		}
	} else {
		selectedID = ""
	}

	formValues := defaultTodoFormValues()
	if selectedTodo != nil {
		formValues = todoFormValuesFromTodo(*selectedTodo)
	}

	todoError := fetchError
	if draft := h.consumeTodoDraft(createMode, selectedID); draft != nil {
		if draft.err != "" {
			todoError = draft.err
		}
		if draft.hasValues {
			formValues = draft.values
		}
		if draft.mode == "create" {
			createMode = true
			selectedTodo = nil
			selectedID = ""
		}
	}

	data := pageData{
		ActiveTab:       "todos",
		Todos:           todos,
		SelectedTodo:    selectedTodo,
		SelectedTodoID:  selectedID,
		Create:          createMode,
		TodoForm:        formValues,
		TodoError:       todoError,
		StatusOptions:   statusOptions(),
		PriorityOptions: priorityOptions(),
		TypeOptions:     typeOptions(),
	}
	h.templates.Render(w, data)
}

func (h *Handler) handleTodosCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.setTodoDraft(todoFormDraft{mode: "create", err: "invalid form input"})
		http.Redirect(w, r, "/web/todos?create=1", http.StatusSeeOther)
		return
	}
	values := todoFormValuesFromRequest(r)
	options, err := values.createOptions()
	if err != nil {
		h.setTodoDraft(todoFormDraft{mode: "create", err: err.Error(), values: values, hasValues: true})
		http.Redirect(w, r, "/web/todos?create=1", http.StatusSeeOther)
		return
	}

	var response todosCreateResponse
	request := todosCreateRequest{Title: values.Title, Options: options}
	if err := postJSON(r.Context(), h.client, h.requestBaseURL(r), "/todos/create", request, &response); err != nil {
		h.setTodoDraft(todoFormDraft{mode: "create", err: err.Error(), values: values, hasValues: true})
		http.Redirect(w, r, "/web/todos?create=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/web/todos?id="+response.Todo.ID, http.StatusSeeOther)
}

func (h *Handler) handleTodosUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	todoID := trimmedQueryValue(r, "id")
	if err := r.ParseForm(); err != nil {
		h.setTodoDraft(todoFormDraft{mode: "update", id: todoID, err: "invalid form input"})
		http.Redirect(w, r, todoRedirectPath(todoID), http.StatusSeeOther)
		return
	}
	values := todoFormValuesFromRequest(r)
	if todoID == "" {
		h.setTodoDraft(todoFormDraft{mode: "update", err: "todo id is required", values: values, hasValues: true})
		http.Redirect(w, r, todoRedirectPath(todoID), http.StatusSeeOther)
		return
	}
	options, err := values.updateOptions()
	if err != nil {
		h.setTodoDraft(todoFormDraft{mode: "update", id: todoID, err: err.Error(), values: values, hasValues: true})
		http.Redirect(w, r, "/web/todos?id="+todoID, http.StatusSeeOther)
		return
	}
	var response todosUpdateResponse
	request := todosUpdateRequest{IDs: []string{todoID}, Options: options}
	if err := postJSON(r.Context(), h.client, h.requestBaseURL(r), "/todos/update", request, &response); err != nil {
		h.setTodoDraft(todoFormDraft{mode: "update", id: todoID, err: err.Error(), values: values, hasValues: true})
		http.Redirect(w, r, "/web/todos?id="+todoID, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/web/todos?id="+todoID, http.StatusSeeOther)
}

func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	baseURL := h.requestBaseURL(r)
	jobs, err := h.refreshJobs(r.Context(), baseURL)
	fetchError := ""
	if err != nil {
		fetchError = err.Error()
	}

	selectedID := trimmedQueryValue(r, "id")
	selectedJob := selectJob(jobs, selectedID)
	if selectedJob == nil && len(jobs) > 0 {
		selectedJob = &jobs[0]
		selectedID = selectedJob.ID
	}

	jobError := fetchError
	if draft := h.consumeJobDraft(selectedID); draft != nil && draft.err != "" {
		jobError = draft.err
	}

	jobLog := template.HTML("")
	if selectedJob != nil {
		events, err := h.fetchJobEvents(r.Context(), baseURL, selectedJob.ID)
		if err != nil {
			jobError = err.Error()
		} else {
			formatted, err := formatJobEvents(events)
			if err != nil {
				jobError = err.Error()
			} else {
				jobLog = formatted
			}
		}
	}

	data := pageData{
		ActiveTab:     "jobs",
		Jobs:          jobs,
		SelectedJob:   selectedJob,
		SelectedJobID: selectedID,
		JobLogHTML:    jobLog,
		JobError:      jobError,
	}
	h.templates.Render(w, data)
}

func (h *Handler) handleJobsStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	todoID := trimmedQueryValue(r, "id")
	if err := r.ParseForm(); err != nil {
		h.setTodoDraft(todoFormDraft{mode: "update", id: todoID, err: "invalid form input"})
		http.Redirect(w, r, todoRedirectPath(todoID), http.StatusSeeOther)
		return
	}
	if todoID == "" {
		h.setTodoDraft(todoFormDraft{mode: "update", err: "todo id is required"})
		http.Redirect(w, r, todoRedirectPath(todoID), http.StatusSeeOther)
		return
	}
	if r.FormValue("confirm") != "yes" {
		h.setTodoDraft(todoFormDraft{mode: "update", id: todoID, err: "confirm start before launching"})
		http.Redirect(w, r, "/web/todos?id="+todoID, http.StatusSeeOther)
		return
	}
	var response doResponse
	request := doRequest{TodoID: todoID}
	if err := postJSON(r.Context(), h.client, h.requestBaseURL(r), "/do", request, &response); err != nil {
		h.setTodoDraft(todoFormDraft{mode: "update", id: todoID, err: err.Error()})
		http.Redirect(w, r, "/web/todos?id="+todoID, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/web/jobs?id="+response.JobID, http.StatusSeeOther)
}

func (h *Handler) handleJobsKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	jobID := trimmedQueryValue(r, "id")
	if jobID == "" {
		h.setJobDraft(jobFormDraft{err: "job id is required"})
		http.Redirect(w, r, "/web/jobs", http.StatusSeeOther)
		return
	}
	request := killRequest{JobID: jobID}
	if err := postJSON(r.Context(), h.client, h.requestBaseURL(r), "/kill", request, &emptyResponse{}); err != nil {
		h.setJobDraft(jobFormDraft{id: jobID, err: err.Error()})
	}
	http.Redirect(w, r, "/web/jobs?id="+jobID, http.StatusSeeOther)
}

func (h *Handler) handleJobsRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	jobID := trimmedQueryValue(r, "id")
	if jobID == "" {
		h.setJobDraft(jobFormDraft{err: "job id is required"})
		http.Redirect(w, r, "/web/jobs", http.StatusSeeOther)
		return
	}
	if _, err := h.fetchJobEvents(r.Context(), h.requestBaseURL(r), jobID); err != nil {
		h.setJobDraft(jobFormDraft{id: jobID, err: err.Error()})
	}
	http.Redirect(w, r, "/web/jobs?id="+jobID, http.StatusSeeOther)
}

func (h *Handler) requestBaseURL(r *http.Request) string {
	if h.baseURL != "" {
		return h.baseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (h *Handler) refreshTodos(ctx context.Context, baseURL string) ([]todo.Todo, error) {
	var response todosListResponse
	err := postJSON(ctx, h.client, baseURL, "/todos/list", todosListRequest{Filter: todo.ListFilter{}}, &response)
	if err != nil {
		h.mu.Lock()
		cached := append([]todo.Todo(nil), h.todos...)
		h.mu.Unlock()
		return cached, err
	}
	ordered := orderTodosForDisplay(response.Todos)
	h.mu.Lock()
	h.todos = append([]todo.Todo(nil), ordered...)
	h.mu.Unlock()
	return ordered, nil
}

func (h *Handler) refreshJobs(ctx context.Context, baseURL string) ([]job.Job, error) {
	var response listResponse
	err := postJSON(ctx, h.client, baseURL, "/list", listRequest{Filter: job.ListFilter{}}, &response)
	if err != nil {
		h.mu.Lock()
		cached := append([]job.Job(nil), h.jobs...)
		h.mu.Unlock()
		return cached, err
	}
	h.mu.Lock()
	h.jobs = append([]job.Job(nil), response.Jobs...)
	h.mu.Unlock()
	return response.Jobs, nil
}

func (h *Handler) fetchJobEvents(ctx context.Context, baseURL, jobID string) ([]job.Event, error) {
	var response logsResponse
	if err := postJSON(ctx, h.client, baseURL, "/logs", logsRequest{JobID: jobID}, &response); err != nil {
		return nil, err
	}
	return response.Events, nil
}

func (h *Handler) seed(baseURL string) {
	for i := 0; i < 25; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, todosErr := h.refreshTodos(ctx, baseURL)
		_, jobsErr := h.refreshJobs(ctx, baseURL)
		cancel()
		if todosErr == nil && jobsErr == nil {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
}

func (h *Handler) consumeTodoDraft(createMode bool, selectedID string) *todoFormDraft {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.todoDraft == nil {
		return nil
	}
	draft := h.todoDraft
	match := false
	if draft.mode == "create" && createMode {
		match = true
	}
	if draft.mode == "update" {
		if draft.id == "" && !createMode {
			match = true
		}
		if draft.id != "" && draft.id == selectedID {
			match = true
		}
	}
	if !match {
		return nil
	}
	h.todoDraft = nil
	return draft
}

func (h *Handler) setTodoDraft(draft todoFormDraft) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.todoDraft = &draft
}

func (h *Handler) consumeJobDraft(selectedID string) *jobFormDraft {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.jobDraft == nil {
		return nil
	}
	if h.jobDraft.id != "" && selectedID != "" && h.jobDraft.id != selectedID {
		return nil
	}
	draft := h.jobDraft
	h.jobDraft = nil
	return draft
}

func (h *Handler) setJobDraft(draft jobFormDraft) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.jobDraft = &draft
}

func defaultTodoFormValues() todoFormValues {
	return todoFormValues{
		Status:   string(defaultTodoStatus()),
		Priority: strconv.Itoa(todo.PriorityMedium),
		Type:     string(todo.TypeTask),
	}
}

func todoFormValuesFromTodo(item todo.Todo) todoFormValues {
	return todoFormValues{
		Title:               item.Title,
		Description:         item.Description,
		Status:              string(item.Status),
		Priority:            strconv.Itoa(item.Priority),
		Type:                string(item.Type),
		ImplementationModel: item.ImplementationModel,
		CodeReviewModel:     item.CodeReviewModel,
		ProjectReviewModel:  item.ProjectReviewModel,
	}
}

func todoFormValuesFromRequest(r *http.Request) todoFormValues {
	return todoFormValues{
		Title:               trimmedFormValue(r, "title"),
		Description:         r.FormValue("description"),
		Status:              trimmedFormValue(r, "status"),
		Priority:            trimmedFormValue(r, "priority"),
		Type:                trimmedFormValue(r, "type"),
		ImplementationModel: trimmedFormValue(r, "implementation_model"),
		CodeReviewModel:     trimmedFormValue(r, "code_review_model"),
		ProjectReviewModel:  trimmedFormValue(r, "project_review_model"),
	}
}

func (values todoFormValues) createOptions() (todo.CreateOptions, error) {
	status, err := parseStatus(values.Status, true, defaultTodoStatus())
	if err != nil {
		return todo.CreateOptions{}, err
	}
	priority, err := parsePriority(values.Priority, true, todo.PriorityMedium)
	if err != nil {
		return todo.CreateOptions{}, err
	}
	typ, err := parseType(values.Type, true, todo.TypeTask)
	if err != nil {
		return todo.CreateOptions{}, err
	}
	return todo.CreateOptions{
		Status:              status,
		Priority:            &priority,
		Type:                typ,
		Description:         values.Description,
		ImplementationModel: values.ImplementationModel,
		CodeReviewModel:     values.CodeReviewModel,
		ProjectReviewModel:  values.ProjectReviewModel,
	}, nil
}

func (values todoFormValues) updateOptions() (todo.UpdateOptions, error) {
	if internalstrings.IsBlank(values.Title) {
		return todo.UpdateOptions{}, fmt.Errorf("title is required")
	}
	status, err := parseStatus(values.Status, false, "")
	if err != nil {
		return todo.UpdateOptions{}, err
	}
	priority, err := parsePriority(values.Priority, false, 0)
	if err != nil {
		return todo.UpdateOptions{}, err
	}
	typ, err := parseType(values.Type, false, "")
	if err != nil {
		return todo.UpdateOptions{}, err
	}
	title := values.Title
	description := values.Description
	statusPtr := status
	priorityPtr := priority
	typePtr := typ
	return todo.UpdateOptions{
		Title:               &title,
		Description:         &description,
		Status:              &statusPtr,
		Priority:            &priorityPtr,
		Type:                &typePtr,
		ImplementationModel: &values.ImplementationModel,
		CodeReviewModel:     &values.CodeReviewModel,
		ProjectReviewModel:  &values.ProjectReviewModel,
	}, nil
}

func trimmedRequired(value, field string, allowEmpty bool) (string, bool, error) {
	trimmed := trimmedValue(value)
	if trimmed == "" {
		if allowEmpty {
			return "", true, nil
		}
		return "", false, fmt.Errorf("%s is required", field)
	}
	return trimmed, false, nil
}

func trimmedValue(value string) string {
	return internalstrings.TrimSpace(value)
}

func parseStatus(value string, allowEmpty bool, fallback todo.Status) (todo.Status, error) {
	trimmed, empty, err := trimmedRequired(value, "status", allowEmpty)
	if err != nil {
		return "", err
	}
	if empty {
		return fallback, nil
	}
	status := todo.Status(trimmed)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid status %q", trimmed)
	}
	return status, nil
}

func parseType(value string, allowEmpty bool, fallback todo.TodoType) (todo.TodoType, error) {
	trimmed, empty, err := trimmedRequired(value, "type", allowEmpty)
	if err != nil {
		return "", err
	}
	if empty {
		return fallback, nil
	}
	typ := todo.TodoType(trimmed)
	if !typ.IsValid() {
		return "", fmt.Errorf("invalid type %q", trimmed)
	}
	return typ, nil
}

func parsePriority(value string, allowEmpty bool, fallback int) (int, error) {
	trimmed, empty, err := trimmedRequired(value, "priority", allowEmpty)
	if err != nil {
		return 0, err
	}
	if empty {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("priority must be a number")
	}
	if err := todo.ValidatePriority(parsed); err != nil {
		return 0, err
	}
	return parsed, nil
}

func trimmedQueryValue(r *http.Request, key string) string {
	return trimmedValue(r.URL.Query().Get(key))
}

func trimmedFormValue(r *http.Request, key string) string {
	return trimmedValue(r.FormValue(key))
}

func selectTodo(todos []todo.Todo, id string) *todo.Todo {
	if id == "" {
		return nil
	}
	for i := range todos {
		if todos[i].ID == id {
			return &todos[i]
		}
	}
	return nil
}

func orderTodosForDisplay(todos []todo.Todo) []todo.Todo {
	if len(todos) == 0 {
		return nil
	}
	proposed := make([]todo.Todo, 0, len(todos))
	open := make([]todo.Todo, 0, len(todos))
	waiting := make([]todo.Todo, 0, len(todos))
	done := make([]todo.Todo, 0, len(todos))
	other := make([]todo.Todo, 0, len(todos))
	for _, item := range todos {
		switch item.Status {
		case todo.StatusProposed:
			proposed = append(proposed, item)
		case "", todo.StatusOpen, todo.StatusInProgress:
			open = append(open, item)
		case todo.StatusWaiting:
			waiting = append(waiting, item)
		case todo.StatusDone, todo.StatusClosed:
			done = append(done, item)
		default:
			other = append(other, item)
		}
	}
	ordered := make([]todo.Todo, 0, len(todos))
	ordered = append(ordered, proposed...)
	ordered = append(ordered, open...)
	ordered = append(ordered, waiting...)
	ordered = append(ordered, done...)
	ordered = append(ordered, other...)
	return ordered
}

func isDoneTodoStatus(status todo.Status) bool {
	switch status {
	case todo.StatusDone, todo.StatusClosed:
		return true
	default:
		return false
	}
}

func selectJob(jobs []job.Job, id string) *job.Job {
	if id == "" {
		return nil
	}
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i]
		}
	}
	return nil
}

func formatJobEvents(events []job.Event) (template.HTML, error) {
	formatter := job.NewEventHTMLFormatter()
	var builder strings.Builder
	for _, event := range events {
		chunk, err := formatter.Append(event)
		if err != nil {
			return "", err
		}
		if chunk != "" {
			builder.WriteString(string(chunk))
		}
	}
	return template.HTML(builder.String()), nil
}

func statusOptions() []selectOption {
	options := make([]selectOption, 0, len(todo.ValidStatuses()))
	for _, status := range todo.ValidStatuses() {
		options = append(options, selectOption{Value: string(status), Label: string(status)})
	}
	return options
}

func priorityOptions() []selectOption {
	values := []int{
		todo.PriorityCritical,
		todo.PriorityHigh,
		todo.PriorityMedium,
		todo.PriorityLow,
		todo.PriorityBacklog,
	}
	options := make([]selectOption, 0, len(values))
	for _, value := range values {
		label := fmt.Sprintf("%s (%d)", todo.PriorityName(value), value)
		options = append(options, selectOption{Value: strconv.Itoa(value), Label: label})
	}
	return options
}

func typeOptions() []selectOption {
	options := make([]selectOption, 0, len(todo.ValidTodoTypes()))
	for _, typ := range todo.ValidTodoTypes() {
		options = append(options, selectOption{Value: string(typ), Label: string(typ)})
	}
	return options
}

func todoRedirectPath(todoID string) string {
	if internalstrings.IsBlank(todoID) {
		return "/web/todos"
	}
	return "/web/todos?id=" + todoID
}

func defaultTodoStatus() todo.Status {
	return todoenv.DefaultStatus()
}

func writeMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

package swarmtui

import (
	"context"
	"fmt"
	"strings"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/swarm"
	"github.com/amonks/incrementum/todo"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tabKind int

const (
	tabTodo tabKind = iota
	tabJobs
)

type focusPane int

const (
	focusList focusPane = iota
	focusDetail
)

type statusLevel int

const (
	statusNone statusLevel = iota
	statusInfo
	statusError
)

type modalKind int

const (
	modalNone modalKind = iota
	modalStartJob
	modalDiscardEdits
)

type model struct {
	ctx            context.Context
	client         *swarm.Client
	width          int
	height         int
	activeTab      tabKind
	focus          focusPane
	todoList       list.Model
	jobList        list.Model
	todoDetail     todoDetailModel
	jobDetail      jobDetailModel
	modal          confirmModal
	status         string
	statusLevel    statusLevel
	selectedTodoID string
	selectedJobID  string
	pendingJobID   string
	tailJobID      string
	tailEvents     <-chan job.Event
	tailErrors     <-chan error
	tailCancel     context.CancelFunc
}

type confirmModal struct {
	kind        modalKind
	message     string
	confirmText string
	cancelText  string
	selected    int
}

func Run(ctx context.Context, client *swarm.Client) error {
	if client == nil {
		return fmt.Errorf("swarm client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	program := tea.NewProgram(newModel(ctx, client), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := program.Run()
	return err
}

func newModel(ctx context.Context, client *swarm.Client) model {
	todoList := list.New(nil, newTodoItemDelegate(), 0, 0)
	todoList.Title = "Todos"
	todoList.SetShowStatusBar(false)
	todoList.SetFilteringEnabled(false)
	todoList.SetShowHelp(false)
	todoList.SetShowPagination(false)

	jobList := list.New(nil, newJobItemDelegate(), 0, 0)
	jobList.Title = "Jobs"
	jobList.SetShowStatusBar(false)
	jobList.SetFilteringEnabled(false)
	jobList.SetShowHelp(false)
	jobList.SetShowPagination(false)

	return model{
		ctx:        ctx,
		client:     client,
		activeTab:  tabTodo,
		focus:      focusList,
		todoList:   todoList,
		jobList:    jobList,
		todoDetail: newTodoDetailModel(),
		jobDetail:  newJobDetailModel(),
		modal:      confirmModal{kind: modalNone},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.loadTodosCmd(), m.loadJobsCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modal.kind != modalNone {
		return m.updateModal(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case tea.KeyMsg:
		updated, cmd, handled := m.handleKey(msg)
		if handled {
			return updated, cmd
		}
		m = updated
	case todosLoadedMsg:
		m.handleTodosLoaded(msg)
	case jobsLoadedMsg:
		return m.handleJobsLoaded(msg)
	case todoSavedMsg:
		return m.handleTodoSaved(msg)
	case jobStartedMsg:
		return m.handleJobStarted(msg)
	case jobLogsMsg:
		return m.handleJobLogs(msg)
	case jobTailEventMsg:
		return m.handleJobTailEvent(msg)
	case jobTailErrMsg:
		return m.handleJobTailErr(msg)
	}

	var cmd tea.Cmd
	if m.activeTab == tabTodo {
		cmd = m.updateTodoTab(msg)
	} else {
		cmd = m.updateJobsTab(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading swarm UI..."
	}
	statusLine := m.renderStatusLine()
	contentHeight := m.height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}
	leftWidth, rightWidth := splitWidths(m.width)

	listContent := m.todoList.View()
	detailContent := m.todoDetail.View()
	if m.activeTab == tabJobs {
		listContent = m.jobList.View()
		detailContent = m.jobDetail.View()
	}

	listPane := m.renderPane(listContent, leftWidth, contentHeight, m.focus == focusList)
	detailPane := m.renderPane(detailContent, rightWidth, contentHeight, m.focus == focusDetail)
	content := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	view := strings.Join([]string{m.renderTabs(), content, statusLine}, "\n")
	if m.modal.kind != modalNone {
		view = m.renderModalOverlay(view)
	}
	return view
}

func (m model) updateTodoTab(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.focus == focusList {
		m.todoList, cmd = m.todoList.Update(msg)
		if m.updateTodoSelection() {
			return tea.Batch(cmd)
		}
		return cmd
	}

	updated, detailCmd, saveRequested := m.todoDetail.Update(msg)
	m.todoDetail = updated
	if saveRequested {
		return tea.Batch(detailCmd, m.saveTodoCmd())
	}
	return detailCmd
}

func (m model) updateJobsTab(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	if m.focus == focusList {
		m.jobList, cmd = m.jobList.Update(msg)
		if m.updateJobSelection() {
			return tea.Batch(cmd, m.loadJobLogsCmd(m.selectedJobID))
		}
		return cmd
	}

	var detailCmd tea.Cmd
	m.jobDetail, detailCmd = m.jobDetail.Update(msg)
	return detailCmd
}

func (m model) handleKey(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.stopJobTail()
		return m, tea.Quit, true
	case "[":
		updated, cmd := m.switchTab(-1)
		return updated, cmd, true
	case "]":
		updated, cmd := m.switchTab(1)
		return updated, cmd, true
	case "enter":
		if m.focus == focusList {
			return m.enterDetail(), nil, true
		}
	case "esc":
		return m.exitDetail(), nil, true
	case "c":
		if m.activeTab == tabTodo && m.focus == focusList {
			return m.startTodoDraft(), nil, true
		}
	case "s":
		if m.activeTab == tabTodo && m.focus == focusList {
			return m.promptStartJob(), nil, true
		}
	}

	return m, nil, false
}

func (m model) switchTab(delta int) (model, tea.Cmd) {
	if delta == 0 {
		return m, nil
	}
	newTab := m.activeTab
	if delta < 0 {
		if newTab == tabTodo {
			newTab = tabJobs
		} else {
			newTab = tabTodo
		}
	} else {
		if newTab == tabJobs {
			newTab = tabTodo
		} else {
			newTab = tabJobs
		}
	}

	if newTab == m.activeTab {
		return m, nil
	}
	if m.activeTab == tabJobs {
		m.stopJobTail()
	}
	if m.focus == focusDetail {
		m = m.setFocus(focusList)
	}
	m.activeTab = newTab
	if m.activeTab == tabJobs {
		if m.updateJobSelection() {
			return m, m.loadJobLogsCmd(m.selectedJobID)
		}
		return m, nil
	}
	m.updateTodoSelection()
	return m, nil
}

func (m model) enterDetail() model {
	if m.focus == focusDetail {
		return m
	}
	return m.setFocus(focusDetail)
}

func (m model) exitDetail() model {
	if m.focus != focusDetail {
		return m
	}
	if m.activeTab == tabTodo && m.todoDetail.IsDirty() {
		m.modal = confirmModal{
			kind:        modalDiscardEdits,
			message:     "Discard unsaved todo changes?",
			confirmText: "Discard",
			cancelText:  "Keep editing",
			selected:    1,
		}
		return m
	}
	return m.setFocus(focusList)
}

func (m model) setFocus(target focusPane) model {
	if m.focus == target {
		return m
	}
	m.focus = target
	if m.activeTab == tabTodo {
		if target == focusDetail {
			m.todoDetail.Focus()
		} else {
			m.todoDetail.Blur()
		}
	}
	return m
}

func (m model) startTodoDraft() model {
	for i, item := range m.todoList.Items() {
		if todoItem, ok := item.(todoItem); ok && todoItem.isDraft {
			m.todoList.Select(i)
			m.todoDetail.SetTodo(todoItem.todo, true)
			return m.setFocus(focusDetail)
		}
	}

	draft := todo.Todo{
		Status:   todo.StatusOpen,
		Type:     todo.TypeTask,
		Priority: todo.PriorityMedium,
	}
	items := append([]list.Item{todoItem{todo: draft, isDraft: true}}, m.todoList.Items()...)
	m.todoList.SetItems(items)
	m.todoList.Select(0)
	m.selectedTodoID = ""
	m.todoDetail.SetTodo(draft, true)
	m.setStatus("Editing new todo", statusInfo)
	return m.setFocus(focusDetail)
}

func (m model) promptStartJob() model {
	item, ok := m.currentTodoItem()
	if !ok || item.isDraft || item.todo.ID == "" {
		m.setStatus("Save the todo before starting a job", statusError)
		return m
	}
	m.modal = confirmModal{
		kind:        modalStartJob,
		message:     fmt.Sprintf("Start swarm job for todo %s?", item.todo.ID),
		confirmText: "Start",
		cancelText:  "Cancel",
		selected:    1,
	}
	return m
}

func (m *model) updateTodoSelection() bool {
	item, ok := m.currentTodoItem()
	selectedID := ""
	if ok && !item.isDraft {
		selectedID = item.todo.ID
	}
	if selectedID == m.selectedTodoID && ok {
		return false
	}
	if ok {
		m.todoDetail.SetTodo(item.todo, item.isDraft)
	} else {
		m.todoDetail.SetTodo(todo.Todo{}, false)
	}
	m.selectedTodoID = selectedID
	return true
}

func (m *model) updateJobSelection() bool {
	item, ok := m.currentJobItem()
	selectedID := ""
	if ok {
		selectedID = item.job.ID
	}
	if selectedID == m.selectedJobID && ok {
		return false
	}
	if ok {
		m.jobDetail.SetJob(item.job)
	} else {
		m.jobDetail.SetJob(job.Job{})
	}
	m.selectedJobID = selectedID
	m.stopJobTail()
	if ok && selectedID != "" && m.activeTab == tabJobs {
		return true
	}
	return false
}

func (m *model) handleTodosLoaded(msg todosLoadedMsg) {
	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Todo load failed: %v", msg.err), statusError)
		return
	}
	items := make([]list.Item, 0, len(msg.todos))
	for _, item := range msg.todos {
		items = append(items, todoItem{todo: item})
	}
	if m.todoDetail.isDraft {
		items = append([]list.Item{todoItem{todo: m.todoDetail.todo, isDraft: true}}, items...)
	}
	m.todoList.SetItems(items)
	if m.selectedTodoID != "" {
		m.selectTodoByID(m.selectedTodoID)
	}
	if len(m.todoList.Items()) > 0 && m.todoList.Index() < 0 {
		m.todoList.Select(0)
	}
	m.updateTodoSelection()
}

func (m model) handleJobsLoaded(msg jobsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Job load failed: %v", msg.err), statusError)
		return m, nil
	}
	items := make([]list.Item, 0, len(msg.jobs))
	for _, item := range msg.jobs {
		items = append(items, jobItem{job: item})
	}
	m.jobList.SetItems(items)
	if m.pendingJobID != "" {
		m.selectJobByID(m.pendingJobID)
		m.pendingJobID = ""
	} else if m.selectedJobID != "" {
		m.selectJobByID(m.selectedJobID)
	}
	if len(m.jobList.Items()) > 0 && m.jobList.Index() < 0 {
		m.jobList.Select(0)
	}
	if m.updateJobSelection() {
		return m, m.loadJobLogsCmd(m.selectedJobID)
	}
	return m, nil
}

func (m model) handleTodoSaved(msg todoSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Save failed: %v", msg.err), statusError)
		return m, nil
	}
	m.todoDetail.SetTodo(msg.todo, false)
	m.setStatus("Todo saved", statusInfo)
	return m, m.loadTodosCmd()
}

func (m model) handleJobStarted(msg jobStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Start job failed: %v", msg.err), statusError)
		return m, nil
	}
	m.pendingJobID = msg.jobID
	m.activeTab = tabJobs
	m.focus = focusList
	m.setStatus(fmt.Sprintf("Started job %s", msg.jobID), statusInfo)
	return m, m.loadJobsCmd()
}

func (m model) handleJobLogs(msg jobLogsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Log load failed: %v", msg.err), statusError)
		return m, nil
	}
	if msg.jobID != m.selectedJobID {
		return m, nil
	}
	if err := m.jobDetail.SetEvents(msg.events); err != nil {
		m.setStatus(fmt.Sprintf("Log parse failed: %v", err), statusError)
		return m, nil
	}
	return m, m.startJobTail()
}

func (m model) handleJobTailEvent(msg jobTailEventMsg) (tea.Model, tea.Cmd) {
	if msg.jobID != m.selectedJobID {
		return m, nil
	}
	if err := m.jobDetail.AppendEvent(msg.event); err != nil {
		m.setStatus(fmt.Sprintf("Log update failed: %v", err), statusError)
		return m, nil
	}
	return m, m.waitForJobEventCmd()
}

func (m model) handleJobTailErr(msg jobTailErrMsg) (tea.Model, tea.Cmd) {
	if msg.jobID != m.selectedJobID {
		return m, nil
	}
	if msg.err != nil {
		m.setStatus(fmt.Sprintf("Job stream error: %v", msg.err), statusError)
		return m, nil
	}
	return m, nil
}

func (m model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	selection := m.modal.selected
	switch key.String() {
	case "left", "right", "tab", "shift+tab", "backtab":
		if selection == 0 {
			selection = 1
		} else {
			selection = 0
		}
		m.modal.selected = selection
		return m, nil
	case "enter":
		confirm := selection == 0
		return m.resolveModal(confirm)
	case "esc":
		return m.resolveModal(false)
	}
	return m, nil
}

func (m model) resolveModal(confirm bool) (tea.Model, tea.Cmd) {
	kind := m.modal.kind
	m.modal = confirmModal{kind: modalNone}
	if !confirm {
		return m, nil
	}
	switch kind {
	case modalStartJob:
		return m, m.startJobCmd()
	case modalDiscardEdits:
		m = m.discardTodoEdits()
		return m, nil
	default:
		return m, nil
	}
}

func (m model) discardTodoEdits() model {
	if m.todoDetail.isDraft {
		items := make([]list.Item, 0, len(m.todoList.Items()))
		for _, item := range m.todoList.Items() {
			if todoItem, ok := item.(todoItem); ok && todoItem.isDraft {
				continue
			}
			items = append(items, item)
		}
		m.todoList.SetItems(items)
		m.todoDetail.SetTodo(todo.Todo{}, false)
		m.todoList.Select(0)
		m.selectedTodoID = ""
	} else {
		if item, ok := m.currentTodoItem(); ok {
			m.todoDetail.SetTodo(item.todo, false)
		}
	}
	m.todoDetail.Blur()
	m.focus = focusList
	m.setStatus("Edits discarded", statusInfo)
	return m
}

func (m model) currentTodoItem() (todoItem, bool) {
	item := m.todoList.SelectedItem()
	if item == nil {
		return todoItem{}, false
	}
	current, ok := item.(todoItem)
	return current, ok
}

func (m model) currentJobItem() (jobItem, bool) {
	item := m.jobList.SelectedItem()
	if item == nil {
		return jobItem{}, false
	}
	current, ok := item.(jobItem)
	return current, ok
}

func (m *model) resize() {
	contentHeight := m.height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}
	leftWidth, rightWidth := splitWidths(m.width)
	listHeight := contentHeight - 2
	if listHeight < 1 {
		listHeight = 1
	}
	listWidth := leftWidth - 4
	if listWidth < 1 {
		listWidth = 1
	}
	innerDetailWidth := rightWidth - 4
	if innerDetailWidth < 1 {
		innerDetailWidth = 1
	}
	innerDetailHeight := contentHeight - 2
	if innerDetailHeight < 1 {
		innerDetailHeight = 1
	}
	m.todoList.SetSize(listWidth, listHeight)
	m.jobList.SetSize(listWidth, listHeight)
	m.todoDetail.SetSize(innerDetailWidth, innerDetailHeight)
	m.jobDetail.SetSize(innerDetailWidth, innerDetailHeight)
}

func splitWidths(width int) (int, int) {
	left := width / 3
	if left < 30 {
		left = 30
	}
	if left > width-20 {
		left = width / 2
	}
	right := width - left
	if right < 20 {
		right = 20
		left = width - right
	}
	return left, right
}

func (m model) renderTabs() string {
	labels := []string{"Todo", "Jobs"}
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		style := tabInactiveStyle
		if (i == 0 && m.activeTab == tabTodo) || (i == 1 && m.activeTab == tabJobs) {
			style = tabActiveStyle
		}
		parts = append(parts, style.Render(label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m model) renderPane(content string, width, height int, focused bool) string {
	style := paneStyle
	if focused {
		style = paneActiveStyle
	}
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return style.Width(width).Height(height).Render(content)
}

func (m model) renderStatusLine() string {
	text := m.status
	if strings.TrimSpace(text) == "" {
		return ""
	}
	style := valueMuted
	if m.statusLevel == statusError {
		style = statusErrorStyle
	} else if m.statusLevel == statusInfo {
		style = statusSuccessStyle
	}
	return style.Render(text)
}

func (m *model) setStatus(text string, level statusLevel) {
	m.status = text
	m.statusLevel = level
}

func (m model) renderModalOverlay(content string) string {
	if m.modal.kind == modalNone {
		return content
	}
	modal := m.modalView()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m model) modalView() string {
	options := []string{m.modal.confirmText, m.modal.cancelText}
	if len(options) < 2 {
		options = []string{"OK", "Cancel"}
	}
	buttons := make([]string, 0, 2)
	for i, option := range options {
		style := valueMuted
		if i == m.modal.selected {
			style = selectedBorder
		}
		buttons = append(buttons, style.Render("["+option+"]"))
	}
	content := strings.Join([]string{m.modal.message, "", strings.Join(buttons, " ")}, "\n")
	modalStyle := lipgloss.NewStyle().Border(borderASCII).Padding(1, 2)
	return modalStyle.Render(content)
}

func (m model) loadTodosCmd() tea.Cmd {
	return func() tea.Msg {
		todos, err := m.client.ListTodos(m.ctx, todo.ListFilter{})
		return todosLoadedMsg{todos: todos, err: err}
	}
}

func (m model) loadJobsCmd() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.client.List(m.ctx)
		return jobsLoadedMsg{jobs: jobs, err: err}
	}
}

func (m model) saveTodoCmd() tea.Cmd {
	return func() tea.Msg {
		if m.todoDetail.isDraft {
			title, opts, err := m.todoDetail.buildCreateOptions()
			if err != nil {
				return todoSavedMsg{err: err}
			}
			created, err := m.client.CreateTodo(m.ctx, title, opts)
			if err != nil {
				return todoSavedMsg{err: err}
			}
			return todoSavedMsg{todo: *created}
		}

		opts, err := m.todoDetail.buildUpdateOptions()
		if err != nil {
			return todoSavedMsg{err: err}
		}
		updated, err := m.client.UpdateTodos(m.ctx, []string{m.todoDetail.todo.ID}, opts)
		if err != nil {
			return todoSavedMsg{err: err}
		}
		if len(updated) == 0 {
			return todoSavedMsg{err: fmt.Errorf("no todo returned from update")}
		}
		return todoSavedMsg{todo: updated[0]}
	}
}

func (m model) startJobCmd() tea.Cmd {
	item, ok := m.currentTodoItem()
	if !ok || item.todo.ID == "" {
		return func() tea.Msg { return jobStartedMsg{err: fmt.Errorf("no todo selected")} }
	}
	return func() tea.Msg {
		jobID, err := m.client.Do(m.ctx, item.todo.ID)
		return jobStartedMsg{jobID: jobID, err: err}
	}
}

func (m model) loadJobLogsCmd(jobID string) tea.Cmd {
	if strings.TrimSpace(jobID) == "" {
		return nil
	}
	return func() tea.Msg {
		events, err := m.client.Logs(m.ctx, jobID)
		return jobLogsMsg{jobID: jobID, events: events, err: err}
	}
}

func (m *model) startJobTail() tea.Cmd {
	item, ok := m.currentJobItem()
	if !ok || item.job.ID == "" {
		return nil
	}
	if item.job.Status != job.StatusActive {
		return nil
	}
	if m.tailJobID == item.job.ID {
		return m.waitForJobEventCmd()
	}
	m.stopJobTail()
	ctx, cancel := context.WithCancel(m.ctx)
	m.tailCancel = cancel
	m.tailJobID = item.job.ID
	m.tailEvents, m.tailErrors = m.client.Tail(ctx, item.job.ID)
	return m.waitForJobEventCmd()
}

func (m model) waitForJobEventCmd() tea.Cmd {
	if m.tailEvents == nil || m.tailErrors == nil {
		return nil
	}
	jobID := m.tailJobID
	return func() tea.Msg {
		select {
		case event, ok := <-m.tailEvents:
			if !ok {
				return jobTailErrMsg{jobID: jobID}
			}
			return jobTailEventMsg{jobID: jobID, event: event}
		case err, ok := <-m.tailErrors:
			if !ok {
				return jobTailErrMsg{jobID: jobID}
			}
			return jobTailErrMsg{jobID: jobID, err: err}
		}
	}
}

func (m *model) stopJobTail() {
	if m.tailCancel != nil {
		m.tailCancel()
		m.tailCancel = nil
	}
	m.tailJobID = ""
	m.tailEvents = nil
	m.tailErrors = nil
}

func (m *model) selectTodoByID(id string) {
	if id == "" {
		return
	}
	for i, item := range m.todoList.Items() {
		todoItem, ok := item.(todoItem)
		if ok && todoItem.todo.ID == id {
			m.todoList.Select(i)
			return
		}
	}
}

func (m *model) selectJobByID(id string) {
	if id == "" {
		return
	}
	for i, item := range m.jobList.Items() {
		jobItem, ok := item.(jobItem)
		if ok && jobItem.job.ID == id {
			m.jobList.Select(i)
			return
		}
	}
}

type todosLoadedMsg struct {
	todos []todo.Todo
	err   error
}

type jobsLoadedMsg struct {
	jobs []job.Job
	err  error
}

type todoSavedMsg struct {
	todo todo.Todo
	err  error
}

type jobStartedMsg struct {
	jobID string
	err   error
}

type jobLogsMsg struct {
	jobID  string
	events []job.Event
	err    error
}

type jobTailEventMsg struct {
	jobID string
	event job.Event
}

type jobTailErrMsg struct {
	jobID string
	err   error
}

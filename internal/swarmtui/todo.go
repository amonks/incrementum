package swarmtui

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/todo"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type todoItem struct {
	todo    todo.Todo
	isDraft bool
}

func (item todoItem) FilterValue() string {
	if item.isDraft {
		return "draft"
	}
	return item.todo.Title
}

type todoItemDelegate struct {
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	doneStyle     lipgloss.Style
}

func newTodoItemDelegate() todoItemDelegate {
	return todoItemDelegate{
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")),
		doneStyle:     valueMuted,
	}
}

func (d todoItemDelegate) Height() int                             { return 1 }
func (d todoItemDelegate) Spacing() int                            { return 0 }
func (d todoItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d todoItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(todoItem)
	if !ok {
		return
	}

	line := formatTodoItem(item, m.Width())
	style := d.normalStyle
	if index == m.Index() {
		style = d.selectedStyle
	} else if !item.isDraft && isDoneStatus(item.todo.Status) {
		style = d.doneStyle
	}
	fmt.Fprint(w, style.Render(line))
}

func formatTodoItem(item todoItem, width int) string {
	id := item.todo.ID
	if item.isDraft {
		id = "draft"
	}
	title := strings.TrimSpace(item.todo.Title)
	if title == "" {
		title = "(untitled)"
	}
	status := string(item.todo.Status)
	priority := todo.PriorityName(item.todo.Priority)
	typeName := string(item.todo.Type)
	meta := fmt.Sprintf("%s/%s/%s", status, typeName, priority)
	line := fmt.Sprintf("%s  %s  [%s]", id, title, meta)
	return truncateText(line, width)
}

func orderTodosForDisplay(todos []todo.Todo) []todo.Todo {
	if len(todos) == 0 {
		return nil
	}
	proposed := make([]todo.Todo, 0, len(todos))
	open := make([]todo.Todo, 0, len(todos))
	done := make([]todo.Todo, 0, len(todos))
	other := make([]todo.Todo, 0, len(todos))
	for _, item := range todos {
		switch item.Status {
		case todo.StatusProposed:
			proposed = append(proposed, item)
		case "", todo.StatusOpen, todo.StatusInProgress:
			open = append(open, item)
		case todo.StatusDone, todo.StatusClosed:
			done = append(done, item)
		default:
			other = append(other, item)
		}
	}
	ordered := make([]todo.Todo, 0, len(todos))
	ordered = append(ordered, proposed...)
	ordered = append(ordered, open...)
	ordered = append(ordered, done...)
	ordered = append(ordered, other...)
	return ordered
}

func isDoneStatus(status todo.Status) bool {
	switch status {
	case todo.StatusDone, todo.StatusClosed:
		return true
	default:
		return false
	}
}

type todoFieldKind int

const (
	fieldTitle todoFieldKind = iota
	fieldDescription
	fieldStatus
	fieldPriority
	fieldType
)

type todoField struct {
	kind      todoFieldKind
	label     string
	input     textinput.Model
	textarea  textarea.Model
	multiLine bool
}

func newTodoField(kind todoFieldKind, label string, value string) todoField {
	field := todoField{kind: kind, label: label}
	if kind == fieldDescription {
		area := textarea.New()
		area.SetValue(value)
		area.ShowLineNumbers = false
		area.Prompt = ""
		field.textarea = area
		field.multiLine = true
		return field
	}
	input := textinput.New()
	input.SetValue(value)
	input.Prompt = ""
	if kind == fieldTitle {
		input.CharLimit = todo.MaxTitleLength
	}
	field.input = input
	return field
}

func (field todoField) Value() string {
	if field.multiLine {
		return field.textarea.Value()
	}
	return field.input.Value()
}

func (field todoField) Focus() todoField {
	if field.multiLine {
		field.textarea.Focus()
		return field
	}
	field.input.Focus()
	return field
}

func (field todoField) Blur() todoField {
	if field.multiLine {
		field.textarea.Blur()
		return field
	}
	field.input.Blur()
	return field
}

func (field todoField) Update(msg tea.Msg) (todoField, tea.Cmd) {
	var cmd tea.Cmd
	if field.multiLine {
		field.textarea, cmd = field.textarea.Update(msg)
		return field, cmd
	}
	field.input, cmd = field.input.Update(msg)
	return field, cmd
}

func (field todoField) View() string {
	if field.multiLine {
		return field.textarea.View()
	}
	return field.input.View()
}

type todoDetailModel struct {
	todo       todo.Todo
	isDraft    bool
	fields     []todoField
	fieldIndex int
	focused    bool
	dirty      bool
	viewport   viewport.Model
}

func newTodoDetailModel() todoDetailModel {
	return todoDetailModel{viewport: viewport.New(0, 0)}
}

func (model *todoDetailModel) SetTodo(item todo.Todo, isDraft bool) {
	wasFocused := model.focused
	model.todo = item
	model.isDraft = isDraft
	model.fields = buildTodoFields(item)
	model.fieldIndex = 0
	model.focused = false
	model.dirty = false
	if wasFocused {
		model.focused = true
		if len(model.fields) > 0 {
			model.fields[model.fieldIndex] = model.fields[model.fieldIndex].Focus()
		}
	}
	model.refreshViewport(true)
}

func (model *todoDetailModel) SetSize(width, height int) {
	inputWidth := width - 4
	if inputWidth < 10 {
		inputWidth = 10
	}
	for i, field := range model.fields {
		if field.multiLine {
			field.textarea.SetWidth(inputWidth)
			field.textarea.SetHeight(5)
		} else {
			field.input.Width = inputWidth
		}
		model.fields[i] = field
	}
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	model.viewport.Width = width
	model.viewport.Height = height
	model.refreshViewport(false)
}

func (model *todoDetailModel) Focus() {
	if model.focused {
		return
	}
	model.focused = true
	if len(model.fields) > 0 {
		model.fields[model.fieldIndex] = model.fields[model.fieldIndex].Focus()
	}
	model.refreshViewport(false)
}

func (model *todoDetailModel) Blur() {
	model.focused = false
	for i := range model.fields {
		model.fields[i] = model.fields[i].Blur()
	}
	model.refreshViewport(false)
}

func (model todoDetailModel) IsDirty() bool {
	return model.dirty
}

func (model todoDetailModel) Update(msg tea.Msg) (todoDetailModel, tea.Cmd, bool) {
	if !model.focused {
		return model, nil, false
	}

	var cmd tea.Cmd

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab":
			model = model.advanceField(1)
			return model, nil, false
		case "shift+tab", "backtab":
			model = model.advanceField(-1)
			return model, nil, false
		case "ctrl+s":
			return model, nil, true
		}
		if updated, cmd, handled := model.handleViewportKey(key); handled {
			return updated, cmd, false
		}
	}

	if _, ok := msg.(tea.MouseMsg); ok {
		model.viewport, cmd = model.viewport.Update(msg)
		return model, cmd, false
	}

	if len(model.fields) == 0 {
		return model, nil, false
	}

	model.fields[model.fieldIndex], cmd = model.fields[model.fieldIndex].Update(msg)
	model.dirty = model.computeDirty()
	model.refreshViewport(false)
	return model, cmd, false
}

func (model todoDetailModel) advanceField(delta int) todoDetailModel {
	if len(model.fields) == 0 {
		return model
	}
	model.fields[model.fieldIndex] = model.fields[model.fieldIndex].Blur()
	model.fieldIndex = (model.fieldIndex + delta + len(model.fields)) % len(model.fields)
	model.fields[model.fieldIndex] = model.fields[model.fieldIndex].Focus()
	model.refreshViewport(false)
	return model
}

func (model todoDetailModel) computeDirty() bool {
	values := model.valuesByKind()
	trimmedTitle := strings.TrimSpace(values[fieldTitle])
	if trimmedTitle != strings.TrimSpace(model.todo.Title) {
		return true
	}
	if values[fieldDescription] != model.todo.Description {
		return true
	}
	trimmedStatus := strings.TrimSpace(values[fieldStatus])
	if trimmedStatus != string(model.todo.Status) {
		return true
	}
	trimmedType := strings.TrimSpace(values[fieldType])
	if trimmedType != string(model.todo.Type) {
		return true
	}
	trimmedPriority := strings.TrimSpace(values[fieldPriority])
	if trimmedPriority != strconv.Itoa(model.todo.Priority) && trimmedPriority != todo.PriorityName(model.todo.Priority) {
		return true
	}
	return false
}

func (model todoDetailModel) valuesByKind() map[todoFieldKind]string {
	values := make(map[todoFieldKind]string, len(model.fields))
	for _, field := range model.fields {
		values[field.kind] = field.Value()
	}
	return values
}

func (model todoDetailModel) View() string {
	return model.viewport.View()
}

func (model *todoDetailModel) handleViewportKey(key tea.KeyMsg) (todoDetailModel, tea.Cmd, bool) {
	switch key.String() {
	case "up", "down":
		if model.focused && model.currentFieldIsMultiline() {
			return *model, nil, false
		}
	case "pgup", "pgdown", "home", "end":
	default:
		return *model, nil, false
	}
	var cmd tea.Cmd
	model.viewport, cmd = model.viewport.Update(key)
	return *model, cmd, true
}

func (model todoDetailModel) currentFieldIsMultiline() bool {
	if len(model.fields) == 0 {
		return false
	}
	return model.fields[model.fieldIndex].multiLine
}

func (model *todoDetailModel) refreshViewport(reset bool) {
	model.viewport.SetContent(model.renderContent())
	if reset {
		model.viewport.GotoTop()
	}
}

func (model todoDetailModel) renderContent() string {
	if model.todo.ID == "" && !model.isDraft {
		return valueMuted.Render("No todo selected")
	}

	lines := make([]string, 0, len(model.fields)+8)
	lines = append(lines, labelStyle.Render("Editable"))
	for _, field := range model.fields {
		if field.kind == fieldDescription {
			lines = append(lines, fmt.Sprintf("%s:", labelStyle.Render(field.label)))
			lines = append(lines, field.View())
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", labelStyle.Render(field.label), field.View()))
	}

	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Read-only"))
	lines = append(lines, formatDetailRow("ID", model.todo.ID))
	lines = append(lines, formatDetailRow("Created", formatOptionalTime(model.todo.CreatedAt)))
	lines = append(lines, formatDetailRow("Updated", formatOptionalTime(model.todo.UpdatedAt)))
	lines = append(lines, formatDetailRow("Started", formatTimePtr(model.todo.StartedAt)))
	lines = append(lines, formatDetailRow("Closed", formatTimePtr(model.todo.ClosedAt)))
	lines = append(lines, formatDetailRow("Completed", formatTimePtr(model.todo.CompletedAt)))
	lines = append(lines, formatDetailRow("Deleted", formatTimePtr(model.todo.DeletedAt)))
	lines = append(lines, formatDetailRow("Delete reason", valueOrDash(model.todo.DeleteReason)))

	content := strings.Join(lines, "\n")
	width := model.viewport.Width
	if width <= 0 {
		return content
	}
	return lipgloss.NewStyle().Width(width).Render(content)
}

func (model todoDetailModel) buildCreateOptions() (string, todo.CreateOptions, error) {
	values := model.valuesByKind()
	title := strings.TrimSpace(values[fieldTitle])
	status, err := parseStatus(values[fieldStatus])
	if err != nil {
		return "", todo.CreateOptions{}, err
	}
	priority, err := parsePriority(values[fieldPriority])
	if err != nil {
		return "", todo.CreateOptions{}, err
	}
	typeName, err := parseTodoType(values[fieldType])
	if err != nil {
		return "", todo.CreateOptions{}, err
	}
	return title, todo.CreateOptions{
		Status:      status,
		Type:        typeName,
		Priority:    todo.PriorityPtr(priority),
		Description: values[fieldDescription],
	}, nil
}

func (model todoDetailModel) buildUpdateOptions() (todo.UpdateOptions, error) {
	values := model.valuesByKind()
	status, err := parseStatus(values[fieldStatus])
	if err != nil {
		return todo.UpdateOptions{}, err
	}
	priority, err := parsePriority(values[fieldPriority])
	if err != nil {
		return todo.UpdateOptions{}, err
	}
	typeName, err := parseTodoType(values[fieldType])
	if err != nil {
		return todo.UpdateOptions{}, err
	}
	return todo.UpdateOptions{
		Title:       stringPtr(strings.TrimSpace(values[fieldTitle])),
		Description: stringPtr(values[fieldDescription]),
		Status:      &status,
		Priority:    &priority,
		Type:        &typeName,
	}, nil
}

func buildTodoFields(item todo.Todo) []todoField {
	fields := []todoField{
		newTodoField(fieldTitle, "Title", item.Title),
		newTodoField(fieldDescription, "Description", item.Description),
		newTodoField(fieldStatus, "Status", string(defaultStatus(item.Status))),
		newTodoField(fieldPriority, "Priority", defaultPriorityValue(item.Priority)),
		newTodoField(fieldType, "Type", string(defaultType(item.Type))),
	}
	return fields
}

func defaultStatus(value todo.Status) todo.Status {
	if value == "" {
		return todo.StatusOpen
	}
	return value
}

func defaultType(value todo.TodoType) todo.TodoType {
	if value == "" {
		return todo.TypeTask
	}
	return value
}

func defaultPriorityValue(priority int) string {
	if priority < todo.PriorityMin || priority > todo.PriorityMax {
		return strconv.Itoa(todo.PriorityMedium)
	}
	return todo.PriorityName(priority)
}

func parseStatus(value string) (todo.Status, error) {
	normalized := internalstrings.NormalizeLowerTrimSpace(value)
	if normalized == "" {
		return todo.StatusOpen, nil
	}
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	status := todo.Status(normalized)
	if status.IsValid() {
		return status, nil
	}
	return "", fmt.Errorf("invalid status %q", value)
}

func parseTodoType(value string) (todo.TodoType, error) {
	normalized := internalstrings.NormalizeLowerTrimSpace(value)
	if normalized == "" {
		return todo.TypeTask, nil
	}
	item := todo.TodoType(normalized)
	if item.IsValid() {
		return item, nil
	}
	return "", fmt.Errorf("invalid type %q", value)
}

func parsePriority(value string) (int, error) {
	trimmed := internalstrings.NormalizeLowerTrimSpace(value)
	if trimmed == "" {
		return todo.PriorityMedium, nil
	}
	if num, err := strconv.Atoi(trimmed); err == nil {
		if num < todo.PriorityMin || num > todo.PriorityMax {
			return 0, fmt.Errorf("priority must be between %d and %d", todo.PriorityMin, todo.PriorityMax)
		}
		return num, nil
	}
	for _, candidate := range []int{todo.PriorityCritical, todo.PriorityHigh, todo.PriorityMedium, todo.PriorityLow, todo.PriorityBacklog} {
		if trimmed == todo.PriorityName(candidate) {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("invalid priority %q", value)
}

func formatDetailRow(label, value string) string {
	return fmt.Sprintf("%s: %s", labelStyle.Render(label), valueMuted.Render(valueOrDash(value)))
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return value
	}
	return runewidth.Truncate(value, width, "...")
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return formatOptionalTime(*value)
}

func valueOrDash(value string) string {
	if internalstrings.IsBlank(value) {
		return "-"
	}
	return value
}

func stringPtr(value string) *string {
	return &value
}

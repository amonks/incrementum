package swarmtui

import (
	"fmt"
	"io"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
	"github.com/amonks/incrementum/job"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type jobItem struct {
	job job.Job
}

func (item jobItem) FilterValue() string {
	return item.job.ID
}

type jobItemDelegate struct {
	normalStyle   lipgloss.Style
	selectedStyle lipgloss.Style
}

func newJobItemDelegate() jobItemDelegate {
	return jobItemDelegate{
		normalStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")),
	}
}

func (d jobItemDelegate) Height() int                             { return 1 }
func (d jobItemDelegate) Spacing() int                            { return 0 }
func (d jobItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d jobItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(jobItem)
	if !ok {
		return
	}

	line := formatJobItem(item, m.Width())
	style := d.normalStyle
	if index == m.Index() {
		style = d.selectedStyle
	}
	fmt.Fprint(w, style.Render(line))
}

func formatJobItem(item jobItem, width int) string {
	status := string(item.job.Status)
	stage := string(item.job.Stage)
	line := fmt.Sprintf("%s  %s/%s  todo:%s", item.job.ID, status, stage, item.job.TodoID)
	return truncateText(line, width)
}

type jobDetailModel struct {
	job       job.Job
	formatter *job.EventFormatter
	content   string
	viewport  viewport.Model
	active    bool
}

func newJobDetailModel() jobDetailModel {
	return jobDetailModel{viewport: viewport.New(0, 0)}
}

func (model *jobDetailModel) SetSize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	model.viewport.Width = width
	model.viewport.Height = height
}

func (model *jobDetailModel) SetJob(jobItem job.Job) {
	model.job = jobItem
	model.formatter = job.NewEventFormatter()
	model.content = ""
	model.active = jobItem.ID != ""
	model.viewport.SetContent(model.renderContent())
	model.viewport.GotoTop()
}

func (model *jobDetailModel) SetEvents(events []job.Event) error {
	model.formatter = job.NewEventFormatter()
	model.content = ""
	for _, event := range events {
		chunk, err := model.formatter.Append(event)
		if err != nil {
			return err
		}
		model.content += chunk
	}
	model.viewport.SetContent(model.renderContent())
	model.viewport.GotoBottom()
	return nil
}

func (model *jobDetailModel) AppendEvent(event job.Event) error {
	if model.formatter == nil {
		model.formatter = job.NewEventFormatter()
	}
	chunk, err := model.formatter.Append(event)
	if err != nil {
		return err
	}
	if chunk == "" {
		return nil
	}
	stickToBottom := model.viewport.AtBottom()
	model.content += chunk
	model.viewport.SetContent(model.renderContent())
	if stickToBottom {
		model.viewport.GotoBottom()
	}
	return nil
}

func (model jobDetailModel) Update(msg tea.Msg) (jobDetailModel, tea.Cmd) {
	model.viewport, _ = model.viewport.Update(msg)
	return model, nil
}

func (model jobDetailModel) View() string {
	if !model.active {
		return valueMuted.Render("No job selected")
	}
	return model.viewport.View()
}

func (model jobDetailModel) renderContent() string {
	if model.job.ID == "" {
		return ""
	}
	header := fmt.Sprintf("Job %s", model.job.ID)
	meta := fmt.Sprintf("Status: %s  Stage: %s  Todo: %s", model.job.Status, model.job.Stage, model.job.TodoID)
	log := internalstrings.TrimTrailingNewlines(model.content)
	if log == "" {
		log = "-"
	}
	return strings.Join([]string{header, meta, "", log}, "\n")
}

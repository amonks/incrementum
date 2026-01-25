package web

import (
	"html/template"
	"time"

	"github.com/amonks/incrementum/job"
	"github.com/amonks/incrementum/todo"
)

func newTemplates() *template.Template {
	funcs := template.FuncMap{
		"eq":                 func(a, b string) bool { return a == b },
		"formatTime":         formatTime,
		"formatOptionalTime": formatOptionalTime,
		"priorityLabel":      todo.PriorityName,
		"isActiveJob":        func(status job.Status) bool { return status == job.StatusActive },
	}
	return template.Must(template.New("page").Funcs(funcs).Parse(pageTemplate))
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return formatTime(*value)
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Swarm {{if eq .ActiveTab "jobs"}}Jobs{{else}}Todos{{end}}</title>
  <style>
    :root {
      color-scheme: light;
    }
    body {
      margin: 0;
      font-family: "Charter", "Georgia", serif;
      color: #2b2520;
      background: radial-gradient(circle at top left, #f4efe3 0%, #fcfaf6 55%, #f6f2e8 100%);
    }
    header {
      padding: 16px 24px;
      border-bottom: 1px solid #d7cdbd;
      background: rgba(255, 255, 255, 0.72);
      backdrop-filter: blur(6px);
    }
    header h1 {
      margin: 0 0 8px 0;
      font-size: 20px;
      letter-spacing: 0.02em;
    }
    .tabs {
      display: flex;
      gap: 12px;
    }
    .tab {
      padding: 8px 14px;
      border-radius: 999px;
      text-decoration: none;
      color: #5b5148;
      border: 1px solid transparent;
    }
    .tab.active {
      color: #1d1712;
      border-color: #d1c6b6;
      background: #f5efe4;
      font-weight: 600;
    }
    main {
      display: flex;
      gap: 18px;
      padding: 18px 24px 28px;
    }
    .pane {
      background: #ffffff;
      border: 1px solid #d7cdbd;
      border-radius: 14px;
      box-shadow: 0 8px 24px rgba(60, 45, 30, 0.08);
    }
    .list-pane {
      width: 35%;
      min-width: 240px;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .detail-pane {
      flex: 1;
      padding: 18px 22px 22px;
    }
    .list-actions {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
    }
    .button-link {
      display: inline-block;
      padding: 6px 12px;
      border-radius: 8px;
      border: 1px solid #cbbfae;
      background: #f7f2e8;
      text-decoration: none;
      color: #2b2520;
      font-size: 14px;
    }
    .item-list {
      list-style: none;
      padding: 0;
      margin: 0;
      display: flex;
      flex-direction: column;
      gap: 8px;
      overflow-y: auto;
    }
    .list-item a {
      display: block;
      padding: 10px 12px;
      border-radius: 10px;
      border: 1px solid transparent;
      text-decoration: none;
      color: inherit;
    }
    .list-item.active a {
      border-color: #c7baa8;
      background: #f6f0e6;
    }
    .item-title {
      font-weight: 600;
      display: block;
    }
    .item-meta {
      color: #72685f;
      font-size: 12px;
    }
    .field {
      display: flex;
      flex-direction: column;
      gap: 6px;
      margin-bottom: 12px;
    }
    input[type="text"],
    select,
    textarea {
      width: 100%;
      padding: 8px 10px;
      border-radius: 8px;
      border: 1px solid #cbbfae;
      font-family: inherit;
      font-size: 14px;
      background: #fffdf9;
      box-sizing: border-box;
    }
    textarea {
      min-height: 120px;
      resize: vertical;
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 16px;
    }
    button {
      padding: 8px 14px;
      border-radius: 8px;
      border: 1px solid #bfb3a2;
      background: #efe6d7;
      font-family: inherit;
      cursor: pointer;
    }
    button.danger {
      background: #f4d7d2;
      border-color: #d7a7a1;
    }
    .readonly {
      display: grid;
      grid-template-columns: 140px 1fr;
      gap: 6px 12px;
      font-size: 14px;
      margin: 16px 0 8px;
    }
    .readonly dt {
      font-weight: 600;
      color: #4f4540;
    }
    .readonly dd {
      margin: 0;
      color: #2b2520;
    }
    .error {
      padding: 10px 12px;
      border-radius: 8px;
      background: #f7d9d6;
      border: 1px solid #d9a7a2;
      margin-bottom: 12px;
      color: #5b1d17;
    }
    .muted {
      color: #72685f;
    }
    .log {
      background: #fcf8f1;
      border: 1px solid #e0d6c6;
      border-radius: 8px;
      padding: 12px;
      white-space: pre-wrap;
      font-family: "Menlo", "Consolas", monospace;
      font-size: 13px;
    }
    .confirm {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 14px;
    }
    @media (max-width: 900px) {
      main {
        flex-direction: column;
      }
      .list-pane {
        width: auto;
      }
    }
  </style>
</head>
<body>
  <header>
    <h1>Swarm Web Client</h1>
    <nav class="tabs">
      <a class="tab {{if eq .ActiveTab "todos"}}active{{end}}" href="/web/todos">Todos</a>
      <a class="tab {{if eq .ActiveTab "jobs"}}active{{end}}" href="/web/jobs">Jobs</a>
    </nav>
  </header>
  <main>
    <section class="pane list-pane">
      {{if eq .ActiveTab "jobs"}}
        <div class="list-actions">
          <strong>Jobs</strong>
        </div>
        <ul class="item-list">
          {{range .Jobs}}
            <li class="list-item {{if eq .ID $.SelectedJobID}}active{{end}}">
              <a href="/web/jobs?id={{.ID}}">
                <span class="item-title">{{.TodoID}}</span>
                <span class="item-meta">{{.ID}} · {{.Status}}</span>
              </a>
            </li>
          {{else}}
            <li class="muted">No jobs found.</li>
          {{end}}
        </ul>
      {{else}}
        <div class="list-actions">
          <strong>Todos</strong>
          <a class="button-link" href="/web/todos?create=1">Create</a>
        </div>
        <ul class="item-list">
          {{range .Todos}}
            <li class="list-item {{if eq .ID $.SelectedTodoID}}active{{end}}">
              <a href="/web/todos?id={{.ID}}">
                <span class="item-title">{{.Title}}</span>
                <span class="item-meta">{{.ID}} · {{.Status}}</span>
              </a>
            </li>
          {{else}}
            <li class="muted">No todos found.</li>
          {{end}}
        </ul>
      {{end}}
    </section>
    <section class="pane detail-pane">
      {{if eq .ActiveTab "jobs"}}
        {{if .JobError}}<div class="error">{{.JobError}}</div>{{end}}
        {{if .SelectedJob}}
          <h2>Job {{.SelectedJob.ID}}</h2>
          <dl class="readonly">
            <dt>Todo</dt><dd>{{.SelectedJob.TodoID}}</dd>
            <dt>Status</dt><dd>{{.SelectedJob.Status}}</dd>
            <dt>Stage</dt><dd>{{.SelectedJob.Stage}}</dd>
            <dt>Created</dt><dd>{{formatTime .SelectedJob.CreatedAt}}</dd>
            <dt>Updated</dt><dd>{{formatTime .SelectedJob.UpdatedAt}}</dd>
            <dt>Started</dt><dd>{{formatOptionalTime .SelectedJob.StartedAt}}</dd>
            <dt>Completed</dt><dd>{{formatOptionalTime .SelectedJob.CompletedAt}}</dd>
          </dl>
          <div class="actions">
            {{if isActiveJob .SelectedJob.Status}}
              <form method="post" action="/web/jobs/refresh?id={{.SelectedJob.ID}}">
                <button type="submit">Refresh</button>
              </form>
              <form method="post" action="/web/jobs/kill?id={{.SelectedJob.ID}}">
                <button class="danger" type="submit">Kill job</button>
              </form>
            {{end}}
          </div>
          <h3>Log</h3>
          {{if .JobLog}}
            <pre class="log">{{.JobLog}}</pre>
          {{else}}
            <p class="muted">No events yet.</p>
          {{end}}
        {{else}}
          <p class="muted">No job selected.</p>
        {{end}}
      {{else}}
        {{if .TodoError}}<div class="error">{{.TodoError}}</div>{{end}}
        {{if .Create}}
          <h2>Create Todo</h2>
          <form method="post" action="/web/todos/create">
            <div class="field">
              <label for="todo-title">Title</label>
              <input id="todo-title" type="text" name="title" value="{{.TodoForm.Title}}" required>
            </div>
            <div class="field">
              <label for="todo-status">Status</label>
              <select id="todo-status" name="status">
                {{range .StatusOptions}}
                  <option value="{{.Value}}" {{if eq .Value $.TodoForm.Status}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </div>
            <div class="field">
              <label for="todo-priority">Priority</label>
              <select id="todo-priority" name="priority">
                {{range .PriorityOptions}}
                  <option value="{{.Value}}" {{if eq .Value $.TodoForm.Priority}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </div>
            <div class="field">
              <label for="todo-type">Type</label>
              <select id="todo-type" name="type">
                {{range .TypeOptions}}
                  <option value="{{.Value}}" {{if eq .Value $.TodoForm.Type}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </div>
            <div class="field">
              <label for="todo-description">Description</label>
              <textarea id="todo-description" name="description">{{.TodoForm.Description}}</textarea>
            </div>
            <div class="actions">
              <button type="submit">Create todo</button>
            </div>
          </form>
        {{else if .SelectedTodo}}
          <h2>Edit Todo</h2>
          <form method="post" action="/web/todos/update?id={{.SelectedTodo.ID}}">
            <div class="field">
              <label for="todo-title">Title</label>
              <input id="todo-title" type="text" name="title" value="{{.TodoForm.Title}}" required>
            </div>
            <div class="field">
              <label for="todo-status">Status</label>
              <select id="todo-status" name="status">
                {{range .StatusOptions}}
                  <option value="{{.Value}}" {{if eq .Value $.TodoForm.Status}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </div>
            <div class="field">
              <label for="todo-priority">Priority</label>
              <select id="todo-priority" name="priority">
                {{range .PriorityOptions}}
                  <option value="{{.Value}}" {{if eq .Value $.TodoForm.Priority}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </div>
            <div class="field">
              <label for="todo-type">Type</label>
              <select id="todo-type" name="type">
                {{range .TypeOptions}}
                  <option value="{{.Value}}" {{if eq .Value $.TodoForm.Type}}selected{{end}}>{{.Label}}</option>
                {{end}}
              </select>
            </div>
            <div class="field">
              <label for="todo-description">Description</label>
              <textarea id="todo-description" name="description">{{.TodoForm.Description}}</textarea>
            </div>
            <div class="actions">
              <button type="submit">Save changes</button>
            </div>
          </form>
          <dl class="readonly">
            <dt>ID</dt><dd>{{.SelectedTodo.ID}}</dd>
            <dt>Created</dt><dd>{{formatTime .SelectedTodo.CreatedAt}}</dd>
            <dt>Updated</dt><dd>{{formatTime .SelectedTodo.UpdatedAt}}</dd>
            <dt>Started</dt><dd>{{formatOptionalTime .SelectedTodo.StartedAt}}</dd>
            <dt>Closed</dt><dd>{{formatOptionalTime .SelectedTodo.ClosedAt}}</dd>
            <dt>Completed</dt><dd>{{formatOptionalTime .SelectedTodo.CompletedAt}}</dd>
            <dt>Deleted</dt><dd>{{formatOptionalTime .SelectedTodo.DeletedAt}}</dd>
            <dt>Delete Reason</dt><dd>{{if .SelectedTodo.DeleteReason}}{{.SelectedTodo.DeleteReason}}{{else}}-{{end}}</dd>
          </dl>
          <form method="post" action="/web/jobs/start?id={{.SelectedTodo.ID}}">
            <div class="actions">
              <label class="confirm"><input type="checkbox" name="confirm" value="yes">Confirm start</label>
              <button type="submit">Start job</button>
            </div>
          </form>
        {{else}}
          <p class="muted">No todo selected.</p>
        {{end}}
      {{end}}
    </section>
  </main>
</body>
</html>
`

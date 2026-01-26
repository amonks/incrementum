package job

import (
	"encoding/json"
	"fmt"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

type opencodeEventPayload struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

type opencodeMessageUpdated struct {
	Info opencodeMessageInfo `json:"info"`
}

type opencodeMessageInfo struct {
	ID     string `json:"id"`
	Role   string `json:"role"`
	Finish string `json:"finish"`
	Time   struct {
		Completed int64 `json:"completed"`
	} `json:"time"`
}

type opencodeMessagePartUpdated struct {
	Part opencodeMessagePart `json:"part"`
}

type opencodeMessagePart struct {
	ID        string            `json:"id"`
	MessageID string            `json:"messageID"`
	Type      string            `json:"type"`
	Text      string            `json:"text"`
	Tool      string            `json:"tool"`
	State     opencodeToolState `json:"state"`
}

type opencodeToolState struct {
	Status string         `json:"status"`
	Input  map[string]any `json:"input"`
}

type opencodeRenderedEvent struct {
	Kind   string
	Label  string
	Body   string
	Inline string
}

type opencodeMessageState struct {
	textParts       map[string]string
	textOrder       []string
	reasoningParts  map[string]string
	reasoningOrder  []string
	promptEmitted   bool
	responseEmitted bool
	thinkingEmitted bool
}

type opencodeEventInterpreter struct {
	switches     map[string]bool
	messageRoles map[string]string
	messages     map[string]*opencodeMessageState
}

func newOpencodeEventInterpreter(switches map[string]bool) *opencodeEventInterpreter {
	resolved := defaultOpencodeEventSwitches()
	if switches != nil {
		for key, value := range switches {
			resolved[key] = value
		}
	}
	return &opencodeEventInterpreter{
		switches:     resolved,
		messageRoles: make(map[string]string),
		messages:     make(map[string]*opencodeMessageState),
	}
}

func defaultOpencodeEventSwitches() map[string]bool {
	return map[string]bool{
		"server.connected":       false,
		"server.heartbeat":       false,
		"session.created":        false,
		"session.updated":        false,
		"session.status":         false,
		"session.idle":           false,
		"session.diff":           false,
		"message.updated":        false,
		"message.part.updated":   true,
		"file.edited":            false,
		"file.watcher.updated":   false,
		"lsp.updated":            false,
		"lsp.client.diagnostics": false,
		"todo.updated":           false,
	}
}

func (i *opencodeEventInterpreter) Handle(event Event) ([]opencodeRenderedEvent, error) {
	if strings.TrimSpace(event.Data) == "" {
		return nil, nil
	}
	payload, err := parseOpencodeEventPayload(event.Data)
	if err != nil || strings.TrimSpace(payload.Type) == "" {
		return rawOpencodeEvent(opencodeEventLabel(event.Name), event.Data), nil
	}

	switch payload.Type {
	case "message.updated":
		outputs, err := i.handleMessageUpdated(payload.Properties)
		return handleOpencodePayload(payload.Type, event.Data, outputs, err)
	case "message.part.updated":
		outputs, err := i.handleMessagePartUpdated(payload.Properties)
		return handleOpencodePayload(payload.Type, event.Data, outputs, err)
	default:
		if !i.enabled(payload.Type) {
			return nil, nil
		}
		return rawOpencodeEvent(opencodeEventLabel(payload.Type), event.Data), nil
	}
}

func handleOpencodePayload(payloadType, data string, outputs []opencodeRenderedEvent, err error) ([]opencodeRenderedEvent, error) {
	if err != nil {
		return rawOpencodeEvent(opencodeEventLabel(payloadType), data), nil
	}
	return outputs, nil
}

func rawOpencodeEvent(label, body string) []opencodeRenderedEvent {
	return []opencodeRenderedEvent{{Kind: "raw", Label: label, Body: body}}
}

func (i *opencodeEventInterpreter) enabled(eventType string) bool {
	if i == nil {
		return false
	}
	return i.switches[eventType]
}

func (i *opencodeEventInterpreter) handleMessageUpdated(payload json.RawMessage) ([]opencodeRenderedEvent, error) {
	var update opencodeMessageUpdated
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, fmt.Errorf("decode opencode message.updated: %w", err)
	}
	info := update.Info
	if strings.TrimSpace(info.ID) == "" {
		return nil, nil
	}
	role := internalstrings.NormalizeLowerTrimSpace(info.Role)
	if role != "" {
		i.messageRoles[info.ID] = role
	}

	state := i.ensureMessageState(info.ID)
	if role == "user" {
		if prompt := i.maybeEmitPromptEvents(state); prompt != nil {
			return prompt, nil
		}
	}

	if role == "assistant" && !state.responseEmitted && messageCompleted(info) && i.enabled("message.part.updated") {
		events := make([]opencodeRenderedEvent, 0, 2)
		thinking := combineParts(state.reasoningOrder, state.reasoningParts)
		if strings.TrimSpace(thinking) != "" && !state.thinkingEmitted {
			state.thinkingEmitted = true
			events = append(events, renderThinking(thinking))
		}
		response := combineParts(state.textOrder, state.textParts)
		if strings.TrimSpace(response) != "" {
			state.responseEmitted = true
			events = append(events, renderResponse(response))
		}
		return events, nil
	}

	return nil, nil
}

func (i *opencodeEventInterpreter) handleMessagePartUpdated(payload json.RawMessage) ([]opencodeRenderedEvent, error) {
	var update opencodeMessagePartUpdated
	if err := json.Unmarshal(payload, &update); err != nil {
		return nil, fmt.Errorf("decode opencode message.part.updated: %w", err)
	}
	part := update.Part
	if strings.TrimSpace(part.MessageID) == "" {
		return nil, nil
	}
	state := i.ensureMessageState(part.MessageID)
	partType := internalstrings.NormalizeLowerTrimSpace(part.Type)
	switch partType {
	case "text":
		i.storePartText(state, part.ID, part.Text, false)
		role := i.messageRoles[part.MessageID]
		if role == "user" {
			if prompt := i.maybeEmitPromptEvents(state); prompt != nil {
				return prompt, nil
			}
		}
	case "reasoning":
		i.storePartText(state, part.ID, part.Text, true)
	case "tool":
		if !i.enabled("message.part.updated") {
			return nil, nil
		}
		if strings.EqualFold(part.State.Status, "completed") {
			summary := summarizeToolCall(part.Tool, part.State.Input)
			if strings.TrimSpace(summary) != "" {
				return []opencodeRenderedEvent{renderTool(summary)}, nil
			}
		}
	}
	return nil, nil
}

func (i *opencodeEventInterpreter) ensureMessageState(messageID string) *opencodeMessageState {
	if i.messages[messageID] == nil {
		i.messages[messageID] = &opencodeMessageState{
			textParts:      make(map[string]string),
			reasoningParts: make(map[string]string),
		}
	}
	return i.messages[messageID]
}

func (i *opencodeEventInterpreter) storePartText(state *opencodeMessageState, partID, text string, reasoning bool) {
	if state == nil || strings.TrimSpace(partID) == "" {
		return
	}
	if reasoning {
		if _, ok := state.reasoningParts[partID]; !ok {
			state.reasoningOrder = append(state.reasoningOrder, partID)
		}
		state.reasoningParts[partID] = text
		return
	}
	if _, ok := state.textParts[partID]; !ok {
		state.textOrder = append(state.textOrder, partID)
	}
	state.textParts[partID] = text
}

func (i *opencodeEventInterpreter) maybeEmitPrompt(state *opencodeMessageState) *opencodeRenderedEvent {
	if state == nil || state.promptEmitted || !i.enabled("message.part.updated") {
		return nil
	}
	prompt := combineParts(state.textOrder, state.textParts)
	if strings.TrimSpace(prompt) == "" {
		return nil
	}
	state.promptEmitted = true
	event := renderPrompt(prompt)
	return &event
}

func (i *opencodeEventInterpreter) maybeEmitPromptEvents(state *opencodeMessageState) []opencodeRenderedEvent {
	if prompt := i.maybeEmitPrompt(state); prompt != nil {
		return []opencodeRenderedEvent{*prompt}
	}
	return nil
}

func renderTool(summary string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "tool", Label: "Opencode tool:", Inline: summary}
}

func renderPrompt(text string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "prompt", Label: "Opencode prompt:", Body: text}
}

func renderResponse(text string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "response", Label: "Opencode response:", Body: text}
}

func renderThinking(text string) opencodeRenderedEvent {
	return opencodeRenderedEvent{Kind: "thinking", Label: "Opencode thinking:", Body: text}
}

func combineParts(order []string, parts map[string]string) string {
	if len(order) == 0 {
		return ""
	}
	merged := make([]string, 0, len(order))
	for _, key := range order {
		text := internalstrings.TrimTrailingNewlines(parts[key])
		if strings.TrimSpace(text) == "" {
			continue
		}
		merged = append(merged, text)
	}
	return strings.Join(merged, "\n\n")
}

func parseOpencodeEventPayload(data string) (opencodeEventPayload, error) {
	var payload opencodeEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func messageCompleted(info opencodeMessageInfo) bool {
	return info.Time.Completed != 0 || strings.TrimSpace(info.Finish) != ""
}

func summarizeToolCall(tool string, input map[string]any) string {
	name := internalstrings.NormalizeLowerTrimSpace(tool)
	switch name {
	case "read":
		if path := stringFromMap(input, "filePath"); path != "" {
			return fmt.Sprintf("read file %s", quoteForLog(path))
		}
	case "write":
		if path := stringFromMap(input, "filePath"); path != "" {
			return fmt.Sprintf("write file %s", quoteForLog(path))
		}
	case "edit":
		if path := stringFromMap(input, "filePath"); path != "" {
			return fmt.Sprintf("edit file %s", quoteForLog(path))
		}
	case "apply_patch":
		return "apply patch"
	case "glob":
		pattern := stringFromMap(input, "pattern")
		path := stringFromMap(input, "path")
		if pattern != "" && path != "" {
			return fmt.Sprintf("glob %s in %s", quoteForLog(pattern), quoteForLog(path))
		}
		if pattern != "" {
			return fmt.Sprintf("glob %s", quoteForLog(pattern))
		}
	case "grep":
		pattern := stringFromMap(input, "pattern")
		include := stringFromMap(input, "include")
		if pattern != "" && include != "" {
			return fmt.Sprintf("search %s in %s", quoteForLog(pattern), quoteForLog(include))
		}
		if pattern != "" {
			return fmt.Sprintf("search %s", quoteForLog(pattern))
		}
	case "bash":
		if command := stringFromMap(input, "command"); command != "" {
			return fmt.Sprintf("run %s", quoteForLog(truncateForLog(command)))
		}
	case "webfetch":
		if url := stringFromMap(input, "url"); url != "" {
			return fmt.Sprintf("fetch %s", quoteForLog(url))
		}
	case "question":
		if question := firstQuestionText(input); question != "" {
			return fmt.Sprintf("ask %s", quoteForLog(truncateForLog(question)))
		}
	}
	if name != "" {
		return name
	}
	return "tool call"
}

func stringFromMap(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	if value, ok := input[key]; ok {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func firstQuestionText(input map[string]any) string {
	if input == nil {
		return ""
	}
	raw, ok := input["questions"]
	if !ok {
		return ""
	}
	items, ok := raw.([]any)
	if !ok {
		return ""
	}
	if len(items) == 0 {
		return ""
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		return ""
	}
	if text, ok := first["question"].(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func quoteForLog(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, "'", "\\'")
	return "'" + escaped + "'"
}

func truncateForLog(value string) string {
	const maxLen = 160
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen-3] + "..."
}

package job

import (
	"html/template"
	"strings"
)

// EventHTMLFormatter formats job events for HTML output.
type EventHTMLFormatter struct {
	writer logHTMLWriter
}

// NewEventHTMLFormatter creates a new EventHTMLFormatter.
func NewEventHTMLFormatter() *EventHTMLFormatter {
	return &EventHTMLFormatter{}
}

// Append formats a job event and returns the newly added HTML output.
func (formatter *EventHTMLFormatter) Append(event Event) (template.HTML, error) {
	if formatter == nil {
		return "", nil
	}
	start := formatter.writer.Len()
	if err := formatter.writer.Append(event); err != nil {
		return "", err
	}
	output := formatter.writer.String()
	if len(output) <= start {
		return "", nil
	}
	return template.HTML(output[start:]), nil
}

type logHTMLWriter struct {
	builder  strings.Builder
	opencode *opencodeEventInterpreter
}

func (writer *logHTMLWriter) Append(event Event) error {
	if strings.HasPrefix(event.Name, "job.") {
		switch event.Name {
		case jobEventStage:
			data, err := decodeEventData[stageEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeEntry("stage", StageMessage(data.Stage), "")
		case jobEventPrompt:
			data, err := decodeEventData[promptEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeEntry("prompt", promptLabel(data.Purpose), normalizeLogBody(data.Prompt))
		case jobEventCommitMessage:
			data, err := decodeEventData[commitMessageEventData](event.Data)
			if err != nil {
				return err
			}
			label := commitMessageLabel(data.Label)
			writer.writeEntry("commit", label, normalizeLogBody(data.Message))
		case jobEventTranscript:
			data, err := decodeEventData[transcriptEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeEntry("transcript", "Opencode transcript:", normalizeLogBody(data.Transcript))
		case jobEventReview:
			data, err := decodeEventData[reviewEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeEntry("review", reviewLabel(data.Purpose), normalizeLogBody(data.Details))
		case jobEventTests:
			data, err := decodeEventData[testsEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeEntry("tests", "", writer.formatTests(data.Results))
		case jobEventOpencodeError:
			data, err := decodeEventData[opencodeErrorEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeEntry("opencode-error", opencodeErrorLabel(data.Purpose), normalizeLogBody(data.Error))
		case jobEventOpencodeStart, jobEventOpencodeEnd:
			return nil
		default:
			return nil
		}
		return nil
	}

	return writer.appendOpencodeEvent(event)
}

func (writer *logHTMLWriter) appendOpencodeEvent(event Event) error {
	if writer.opencode == nil {
		writer.opencode = newOpencodeEventInterpreter(nil)
	}
	outputs, err := writer.opencode.Handle(event)
	if err != nil {
		return err
	}
	for _, output := range outputs {
		writer.writeOpencodeEntry(output)
	}
	return nil
}

func (writer *logHTMLWriter) writeOpencodeEntry(event opencodeRenderedEvent) {
	kind := "opencode"
	if event.Kind != "" {
		kind = "opencode-" + event.Kind
	}
	if event.Inline != "" {
		writer.writeInline(kind, event.Label, event.Inline)
		return
	}
	writer.writeEntry(kind, event.Label, normalizeLogBody(event.Body))
}

func (writer *logHTMLWriter) writeEntry(kind, label, body string) {
	class := "log-entry"
	if kind != "" {
		class += " log-entry-" + kind
	}
	writer.builder.WriteString("<div class=\"")
	writer.builder.WriteString(class)
	writer.builder.WriteString("\">")
	if strings.TrimSpace(label) != "" {
		writer.builder.WriteString("<div class=\"log-label\">")
		writer.builder.WriteString(template.HTMLEscapeString(label))
		writer.builder.WriteString("</div>")
	}
	if strings.TrimSpace(body) != "" {
		writer.builder.WriteString("<div class=\"log-body\">")
		writer.builder.WriteString(template.HTMLEscapeString(body))
		writer.builder.WriteString("</div>")
	}
	writer.builder.WriteString("</div>")
}

func (writer *logHTMLWriter) writeInline(kind, label, value string) {
	class := "log-entry"
	if kind != "" {
		class += " log-entry-" + kind
	}
	writer.builder.WriteString("<div class=\"")
	writer.builder.WriteString(class)
	writer.builder.WriteString("\">")
	if strings.TrimSpace(label) != "" {
		writer.builder.WriteString("<span class=\"log-label\">")
		writer.builder.WriteString(template.HTMLEscapeString(label))
		writer.builder.WriteString("</span>")
	}
	if strings.TrimSpace(value) != "" {
		writer.builder.WriteString("<span class=\"log-inline\">")
		writer.builder.WriteString(template.HTMLEscapeString(value))
		writer.builder.WriteString("</span>")
	}
	writer.builder.WriteString("</div>")
}

func (writer *logHTMLWriter) formatTests(results []testResultEventData) string {
	formatted := testResultLogsFromEventData(results)
	if len(formatted) == 0 {
		return "-"
	}
	return formatTestLogBody(formatted)
}

func (writer *logHTMLWriter) String() string {
	return writer.builder.String()
}

func (writer *logHTMLWriter) Len() int {
	return writer.builder.Len()
}

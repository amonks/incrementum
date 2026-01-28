package job

import (
	"html/template"
	"strings"

	internalstrings "github.com/amonks/incrementum/internal/strings"
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
	output, err := appendEventOutput(&formatter.writer, event)
	if err != nil {
		return "", err
	}
	return template.HTML(output), nil
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
			writer.writeNormalizedEntry("prompt", promptLabel(data.Purpose), data.Prompt)
		case jobEventCommitMessage:
			data, err := decodeEventData[commitMessageEventData](event.Data)
			if err != nil {
				return err
			}
			label := commitMessageLabel(data.Label)
			writer.writeNormalizedEntry("commit", label, data.Message)
		case jobEventTranscript:
			data, err := decodeEventData[transcriptEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeNormalizedEntry("transcript", "Opencode transcript:", data.Transcript)
		case jobEventReview:
			data, err := decodeEventData[reviewEventData](event.Data)
			if err != nil {
				return err
			}
			writer.writeNormalizedEntry("review", reviewLabel(data.Purpose), data.Details)
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
			writer.writeNormalizedEntry("opencode-error", opencodeErrorLabel(data.Purpose), data.Error)
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
	writer.writeNormalizedEntry(kind, event.Label, event.Body)
}

func (writer *logHTMLWriter) writeEntry(kind, label, body string) {
	writer.startEntry(kind)
	writer.writeElement("div", "log-label", label)
	writer.writeElement("div", "log-body", body)
	writer.endEntry()
}

func (writer *logHTMLWriter) writeNormalizedEntry(kind, label, body string) {
	writer.writeEntry(kind, label, normalizeLogBody(body))
}

func (writer *logHTMLWriter) writeInline(kind, label, value string) {
	writer.startEntry(kind)
	writer.writeElement("span", "log-label", label)
	writer.writeElement("span", "log-inline", value)
	writer.endEntry()
}

func (writer *logHTMLWriter) writeElement(tag, className, value string) {
	if internalstrings.IsBlank(value) {
		return
	}
	writer.builder.WriteString("<")
	writer.builder.WriteString(tag)
	writer.builder.WriteString(" class=\"")
	writer.builder.WriteString(className)
	writer.builder.WriteString("\">")
	writer.builder.WriteString(template.HTMLEscapeString(value))
	writer.builder.WriteString("</")
	writer.builder.WriteString(tag)
	writer.builder.WriteString(">")
}

func (writer *logHTMLWriter) startEntry(kind string) {
	class := "log-entry"
	if kind != "" {
		class += " log-entry-" + kind
	}
	writer.builder.WriteString("<div class=\"")
	writer.builder.WriteString(class)
	writer.builder.WriteString("\">")
}

func (writer *logHTMLWriter) endEntry() {
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

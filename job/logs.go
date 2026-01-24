package job

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/amonks/incrementum/internal/ui"
)

// LogSnapshot returns the stored job event log.
func LogSnapshot(jobID string, opts EventLogOptions) (string, error) {
	path, err := eventLogPath(jobID, opts)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	writer := &logSnapshotWriter{}
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			var event Event
			if unmarshalErr := json.Unmarshal([]byte(line), &event); unmarshalErr != nil {
				return "", fmt.Errorf("decode job event: %w", unmarshalErr)
			}
			if appendErr := writer.Append(event); appendErr != nil {
				return "", appendErr
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return strings.TrimRight(writer.String(), "\n"), nil
}

type logSnapshotWriter struct {
	builder     strings.Builder
	started     bool
	skipSpacing bool
}

func (writer *logSnapshotWriter) Append(event Event) error {
	switch event.Name {
	case jobEventStage:
		data, err := decodeEventData[stageEventData](event.Data)
		if err != nil {
			return err
		}
		writer.writeStage(StageMessage(data.Stage))
	case jobEventPrompt:
		data, err := decodeEventData[promptEventData](event.Data)
		if err != nil {
			return err
		}
		writer.writeBlock(
			formatLogLabel(promptLabel(data.Purpose), documentIndent),
			formatLogBody(data.Prompt, subdocumentIndent, true),
		)
	case jobEventCommitMessage:
		data, err := decodeEventData[commitMessageEventData](event.Data)
		if err != nil {
			return err
		}
		label := "Commit message:"
		if strings.TrimSpace(data.Label) != "" {
			label = fmt.Sprintf("%s commit message:", data.Label)
		}
		writer.writeBlock(
			formatLogLabel(label, documentIndent),
			formatLogBody(data.Message, subdocumentIndent, true),
		)
	case jobEventTranscript:
		data, err := decodeEventData[transcriptEventData](event.Data)
		if err != nil {
			return err
		}
		writer.skipSpacing = true
		writer.writeBlock(
			formatLogLabel("Opencode transcript:", documentIndent),
			formatLogBody(data.Transcript, subdocumentIndent, true),
		)
	case jobEventReview:
		data, err := decodeEventData[reviewEventData](event.Data)
		if err != nil {
			return err
		}
		writer.writeBlock(
			formatLogLabel(reviewLabel(data.Purpose), documentIndent),
			formatLogBody(data.Details, subdocumentIndent, true),
		)
	case jobEventTests:
		data, err := decodeEventData[testsEventData](event.Data)
		if err != nil {
			return err
		}
		writer.writeTests(data.Results)
	case jobEventOpencodeStart, jobEventOpencodeEnd:
		return nil
	default:
		return nil
	}
	return nil
}

func (writer *logSnapshotWriter) writeStage(value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if writer.started {
		writer.builder.WriteString("\n")
	}
	writer.builder.WriteString(value)
	writer.builder.WriteString("\n")
	writer.started = true
	writer.skipSpacing = true
}

func (writer *logSnapshotWriter) writeBlock(lines ...string) {
	if len(lines) == 0 {
		return
	}
	if writer.started && !writer.skipSpacing {
		writer.builder.WriteString("\n")
	}
	writer.skipSpacing = false
	writer.started = true
	for _, line := range lines {
		writer.builder.WriteString(line)
		writer.builder.WriteString("\n")
	}
}

func (writer *logSnapshotWriter) writeTests(results []testResultEventData) {
	if len(results) == 0 {
		writer.writeBlock(formatLogBody("-", documentIndent, false))
		return
	}
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{result.Command, fmt.Sprintf("%d", result.ExitCode)})
	}
	body := ui.FormatTable([]string{"Command", "Exit Code"}, rows)
	writer.writeBlock(formatLogBody(body, documentIndent, false))
}

func (writer *logSnapshotWriter) String() string {
	return writer.builder.String()
}

func decodeEventData[T any](payload string) (T, error) {
	var data T
	if strings.TrimSpace(payload) == "" {
		return data, nil
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return data, fmt.Errorf("decode job event data: %w", err)
	}
	return data, nil
}

package job

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestEventLogAppendsEvents(t *testing.T) {
	eventsDir := t.TempDir()
	log, err := OpenEventLog("job-events", EventLogOptions{EventsDir: eventsDir})
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}

	if err := log.Append(Event{Name: "job.stage", Data: "{\"stage\":\"implementing\"}"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Append(Event{ID: "2", Name: "job.prompt", Data: "prompt"}); err != nil {
		_ = log.Close()
		t.Fatalf("append event: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	path := filepath.Join(eventsDir, "job-events.jsonl")
	events := readEventLog(t, path)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "job.stage" || events[0].Data == "" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].ID != "2" || events[1].Name != "job.prompt" || events[1].Data != "prompt" {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
}

func readEventLog(t *testing.T, path string) []Event {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close event log: %v", err)
		}
	}()

	decoder := json.NewDecoder(file)
	var events []Event
	for {
		var event Event
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("decode event: %v", err)
		}
		events = append(events, event)
	}
	return events
}

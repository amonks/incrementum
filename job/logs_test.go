package job

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogSnapshotReadsJobEventLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	jobID := "job-logs"
	eventsDir := filepath.Join(home, ".local", "share", "incrementum", "jobs", "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatalf("create events dir: %v", err)
	}

	path := filepath.Join(eventsDir, jobID+".jsonl")
	log := "{\"name\":\"job.stage\",\"data\":\"{\\\"stage\\\":\\\"implementing\\\"}\"}\n"
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	snapshot, err := LogSnapshot(jobID, EventLogOptions{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot != log {
		t.Fatalf("expected snapshot %q, got %q", log, snapshot)
	}
}

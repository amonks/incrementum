package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpencodeLogTailStreamsUpdates(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "session.log")
	if err := os.WriteFile(logPath, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var output bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- opencodeLogTail(ctx, logPath, &output, 5*time.Millisecond)
	}()

	time.Sleep(10 * time.Millisecond)
	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	if _, err := file.WriteString("second\n"); err != nil {
		_ = file.Close()
		t.Fatalf("append log: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if strings.Contains(output.String(), "second\n") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected tail to include appended content, got %q", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(output.String(), "first\n") {
		t.Fatalf("expected tail to include initial content, got %q", output.String())
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("tail error: %v", err)
	}
}

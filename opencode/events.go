package opencode

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Event represents a single opencode server-sent event.
type Event struct {
	ID   string
	Name string
	Data string
}

// RunHandle represents an in-flight opencode run.
type RunHandle struct {
	Events <-chan Event
	wait   func() (RunResult, error)
}

// Wait blocks until the run completes.
func (h *RunHandle) Wait() (RunResult, error) {
	return h.wait()
}

// DrainEvents consumes events until the channel closes.
func DrainEvents(events <-chan Event) <-chan struct{} {
	done := make(chan struct{})
	if events == nil {
		close(done)
		return done
	}
	go func() {
		for range events {
		}
		close(done)
	}()
	return done
}

type eventStorage struct {
	Root string
}

func (s eventStorage) LogSnapshot(sessionID string) (string, error) {
	data, err := os.ReadFile(s.eventPath(sessionID))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s eventStorage) eventPath(sessionID string) string {
	return filepath.Join(s.Root, sessionID+".sse")
}

func (s eventStorage) newRecorder() (*eventRecorder, error) {
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return nil, fmt.Errorf("create opencode events dir: %w", err)
	}

	file, err := os.CreateTemp(s.Root, "opencode-*.sse")
	if err != nil {
		return nil, fmt.Errorf("create opencode event log: %w", err)
	}

	return &eventRecorder{
		dir:    s.Root,
		path:   file.Name(),
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

type eventRecorder struct {
	dir       string
	path      string
	file      *os.File
	writer    *bufio.Writer
	sessionID string
	mu        sync.Mutex
}

func (r *eventRecorder) Write(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer == nil {
		return fmt.Errorf("event recorder is closed")
	}
	if len(data) == 0 {
		return nil
	}
	_, err := r.writer.Write(data)
	return err
}

func (r *eventRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer == nil {
		return nil
	}
	flushErr := r.writer.Flush()
	closeErr := r.file.Close()
	r.writer = nil
	r.file = nil
	return errors.Join(flushErr, closeErr)
}

func (r *eventRecorder) SetSessionID(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessionID != "" {
		return nil
	}

	finalPath := filepath.Join(r.dir, sessionID+".sse")
	if err := os.Rename(r.path, finalPath); err != nil {
		return fmt.Errorf("rename opencode event log: %w", err)
	}

	r.path = finalPath
	r.sessionID = sessionID
	return nil
}

func readEventStream(ctx context.Context, reader io.Reader, recorder *eventRecorder, events chan<- Event) error {
	if reader == nil {
		return fmt.Errorf("event stream is nil")
	}
	defer recorder.Close()

	buf := bufio.NewReader(reader)
	builder := eventBuilder{}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := buf.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		if writeErr := recorder.Write([]byte(line)); writeErr != nil {
			return writeErr
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if errors.Is(err, io.EOF) {
			if trimmed != "" {
				builder.handleLine(trimmed)
			}
			if builder.hasContent() {
				if sendErr := sendEvent(ctx, events, builder.event()); sendErr != nil {
					return sendErr
				}
			}
			return nil
		}

		if trimmed == "" {
			if builder.hasContent() {
				if sendErr := sendEvent(ctx, events, builder.event()); sendErr != nil {
					return sendErr
				}
				builder.reset()
			}
			continue
		}

		builder.handleLine(trimmed)
	}
}

func sendEvent(ctx context.Context, events chan<- Event, event Event) error {
	select {
	case <-ctx.Done():
		return nil
	case events <- event:
		return nil
	}
}

type eventBuilder struct {
	id   string
	name string
	data []string
}

func (b *eventBuilder) handleLine(line string) {
	if strings.HasPrefix(line, ":") {
		return
	}

	field, value, ok := strings.Cut(line, ":")
	if !ok {
		return
	}
	if strings.HasPrefix(value, " ") {
		value = strings.TrimPrefix(value, " ")
	}

	switch field {
	case "event":
		b.name = value
	case "data":
		b.data = append(b.data, value)
	case "id":
		b.id = value
	}
}

func (b *eventBuilder) hasContent() bool {
	return b.id != "" || b.name != "" || len(b.data) > 0
}

func (b *eventBuilder) event() Event {
	return Event{ID: b.id, Name: b.name, Data: strings.Join(b.data, "\n")}
}

func (b *eventBuilder) reset() {
	b.id = ""
	b.name = ""
	b.data = nil
}

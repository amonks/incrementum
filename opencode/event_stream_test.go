package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConnectEventStreamTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	start := time.Now()
	_, err := connectEventStream(server.URL, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected connectEventStream to return error")
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatalf("expected connectEventStream to timeout quickly, took %s", time.Since(start))
	}
}

func TestConnectEventStreamKeepsRequestContext(t *testing.T) {
	started := make(chan struct{}, 1)
	ended := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
		close(ended)
	}))
	defer server.Close()

	resp, err := connectEventStream(server.URL, 250*time.Millisecond)
	if err != nil {
		t.Fatalf("connect event stream: %v", err)
	}

	select {
	case <-started:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event stream request to start")
	}

	select {
	case <-ended:
		t.Fatal("expected request context to remain active")
	case <-time.After(100 * time.Millisecond):
	}

	_ = resp.Body.Close()
	select {
	case <-ended:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected request context to end after closing stream")
	}
}

func TestReadEventStreamBlocksOnFullChannel(t *testing.T) {
	storage := eventStorage{Root: t.TempDir()}
	recorder, err := storage.newRecorder()
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}

	events := make(chan Event, 1)
	stream := "event: one\n" +
		"data: first\n" +
		"\n" +
		"event: two\n" +
		"data: second\n" +
		"\n"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- readEventStream(ctx, strings.NewReader(stream), recorder, events)
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		if len(events) == 1 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			err := <-done
			t.Fatalf("expected first event to arrive: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read event stream: %v", err)
		}
		t.Fatal("expected readEventStream to block on full channel")
	case <-time.After(50 * time.Millisecond):
	}

	first := <-events
	if first.Name != "one" || first.Data != "first" {
		t.Fatalf("unexpected first event: %#v", first)
	}

	select {
	case second := <-events:
		if second.Name != "two" || second.Data != "second" {
			t.Fatalf("unexpected second event: %#v", second)
		}
	case <-time.After(100 * time.Millisecond):
		cancel()
		err := <-done
		t.Fatalf("expected second event to arrive: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read event stream: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		cancel()
		err := <-done
		t.Fatalf("readEventStream did not finish: %v", err)
	}

	data, err := os.ReadFile(recorder.path)
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	if string(data) != stream {
		t.Fatalf("expected log to match stream\nwant: %q\n got: %q", stream, string(data))
	}
}

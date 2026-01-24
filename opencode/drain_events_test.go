package opencode

import (
	"testing"
	"time"
)

func TestDrainEventsClosesImmediatelyForNilChannel(t *testing.T) {
	done := DrainEvents(nil)
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected drain to complete immediately")
	}
}

func TestDrainEventsCompletesAfterChannelClose(t *testing.T) {
	events := make(chan Event, 1)
	done := DrainEvents(events)

	events <- Event{ID: "1"}
	close(events)

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected drain to complete after channel closes")
	}
}

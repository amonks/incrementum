package ids

import "testing"

func TestGenerate(t *testing.T) {
	id := Generate("todo-123", 8)

	if len(id) != 8 {
		t.Fatalf("expected ID length 8, got %d: %q", len(id), id)
	}

	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= '2' && c <= '7')) {
			t.Errorf("ID contains invalid character %q: %q", c, id)
		}
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	id1 := Generate("todo-123", 10)
	id2 := Generate("todo-123", 10)

	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: got %q and %q", id1, id2)
	}
}

func TestGenerate_DifferentInputs(t *testing.T) {
	id1 := Generate("todo-123", 10)
	id2 := Generate("todo-999", 10)

	if id1 == id2 {
		t.Error("different inputs should produce different IDs")
	}
}

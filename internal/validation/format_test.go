package validation

import "testing"

func TestFormatValidValues(t *testing.T) {
	type sample string

	const (
		first  sample = "first"
		second sample = "second"
	)

	got := FormatValidValues([]sample{first, second})
	want := "first, second"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

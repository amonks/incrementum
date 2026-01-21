package ids

import "testing"

func TestUniquePrefixLengths(t *testing.T) {
	ids := []string{"2u3iutfd", "2a9k1111", "abc12345"}
	lengths := UniquePrefixLengths(ids)

	if got := lengths["2u3iutfd"]; got != 2 {
		t.Fatalf("expected 2u3iutfd prefix length 2, got %d", got)
	}
	if got := lengths["2a9k1111"]; got != 2 {
		t.Fatalf("expected 2a9k1111 prefix length 2, got %d", got)
	}
	if got := lengths["abc12345"]; got != 1 {
		t.Fatalf("expected abc12345 prefix length 1, got %d", got)
	}
}

func TestUniquePrefixLengthsIsCaseInsensitive(t *testing.T) {
	ids := []string{"Abc", "aBD"}
	lengths := UniquePrefixLengths(ids)

	if got := lengths["abc"]; got != 3 {
		t.Fatalf("expected abc prefix length 3, got %d", got)
	}
	if got := lengths["abd"]; got != 3 {
		t.Fatalf("expected abd prefix length 3, got %d", got)
	}
}

func TestUniquePrefixLengthsSkipsDuplicatesAndEmpty(t *testing.T) {
	ids := []string{"abc", "", "ABC"}
	lengths := UniquePrefixLengths(ids)

	if len(lengths) != 1 {
		t.Fatalf("expected 1 unique ID, got %d", len(lengths))
	}
	if got := lengths["abc"]; got != 1 {
		t.Fatalf("expected abc prefix length 1, got %d", got)
	}
}

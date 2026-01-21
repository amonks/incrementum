package main

import "testing"

func TestRootCommandName(t *testing.T) {
	if rootCmd.Use != "ii" {
		t.Fatalf("expected root command name ii, got %q", rootCmd.Use)
	}
}

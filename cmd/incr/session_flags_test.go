package main

import "testing"

func TestSessionStartRevFlagDefault(t *testing.T) {
	flag := sessionStartCmd.Flags().Lookup("rev")
	if flag == nil {
		t.Fatal("expected session start rev flag")
	}
	if flag.DefValue != "@" {
		t.Fatalf("expected default rev '@', got %q", flag.DefValue)
	}
}

func TestSessionRunRevFlagDefault(t *testing.T) {
	flag := sessionRunCmd.Flags().Lookup("rev")
	if flag == nil {
		t.Fatal("expected session run rev flag")
	}
	if flag.DefValue != "@" {
		t.Fatalf("expected default rev '@', got %q", flag.DefValue)
	}
}

func TestSessionStartRevFlagSet(t *testing.T) {
	if err := sessionStartCmd.Flags().Set("rev", "main"); err != nil {
		t.Fatalf("set rev flag: %v", err)
	}
	if sessionStartRev != "main" {
		t.Fatalf("expected sessionStartRev 'main', got %q", sessionStartRev)
	}
	if err := sessionStartCmd.Flags().Set("rev", "@"); err != nil {
		t.Fatalf("reset rev flag: %v", err)
	}
}

func TestSessionRunRevFlagSet(t *testing.T) {
	if err := sessionRunCmd.Flags().Set("rev", "main"); err != nil {
		t.Fatalf("set rev flag: %v", err)
	}
	if sessionRunRev != "main" {
		t.Fatalf("expected sessionRunRev 'main', got %q", sessionRunRev)
	}
	if err := sessionRunCmd.Flags().Set("rev", "@"); err != nil {
		t.Fatalf("reset rev flag: %v", err)
	}
}

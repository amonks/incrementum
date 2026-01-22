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

func TestShouldUseEditor(t *testing.T) {
	cases := []struct {
		name           string
		hasCreateFlags bool
		editFlag       bool
		noEditFlag     bool
		interactive    bool
		want           bool
	}{
		{
			name:        "no flags interactive",
			interactive: true,
			want:        true,
		},
		{
			name:        "no flags non-interactive",
			interactive: false,
			want:        false,
		},
		{
			name:           "create flags interactive",
			hasCreateFlags: true,
			interactive:    true,
			want:           false,
		},
		{
			name:           "create flags with edit",
			hasCreateFlags: true,
			editFlag:       true,
			interactive:    true,
			want:           true,
		},
		{
			name:        "no flags with edit",
			editFlag:    true,
			interactive: false,
			want:        true,
		},
		{
			name:        "no flags with no-edit",
			noEditFlag:  true,
			interactive: true,
			want:        false,
		},
		{
			name:           "create flags with no-edit",
			hasCreateFlags: true,
			noEditFlag:     true,
			interactive:    true,
			want:           false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldUseEditor(tc.hasCreateFlags, tc.editFlag, tc.noEditFlag, tc.interactive)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

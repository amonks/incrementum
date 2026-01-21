package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestTodoReasonFlagOnlyOnDelete(t *testing.T) {
	flag := todoDeleteCmd.Flags().Lookup("reason")
	if flag == nil {
		t.Fatal("expected todo delete to have --reason flag")
	}
	if flag.DefValue != "" {
		t.Fatalf("expected todo delete reason default empty, got %q", flag.DefValue)
	}

	cases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "close", cmd: todoCloseCmd},
		{name: "finish", cmd: todoFinishCmd},
		{name: "reopen", cmd: todoReopenCmd},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cmd.Flags().Lookup("reason") != nil {
				t.Fatalf("did not expect --reason flag for todo %s", tc.name)
			}
		})
	}
}

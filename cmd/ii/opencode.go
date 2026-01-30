package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/opencode"
	"github.com/spf13/cobra"
)

var opencodeCmd = &cobra.Command{
	Use:   "opencode",
	Short: "Manage opencode sessions",
}

var opencodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List opencode sessions",
	RunE:  runOpencodeList,
}

var opencodeLogsCmd = &cobra.Command{
	Use:   "logs <session-id>",
	Short: "Show opencode session logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpencodeLogs,
}

var opencodeListJSON bool
var opencodeListAll bool

func init() {
	rootCmd.AddCommand(opencodeCmd)
	opencodeCmd.AddCommand(opencodeListCmd, opencodeLogsCmd)

	opencodeListCmd.Flags().BoolVar(&opencodeListJSON, "json", false, "Output as JSON")
	listflags.AddAllFlag(opencodeListCmd, &opencodeListAll)
}

func runOpencodeList(cmd *cobra.Command, args []string) error {
	store, repoPath, err := openOpencodeStoreAndRepoPath()
	if err != nil {
		return err
	}

	sessions, err := store.ListSessions(repoPath)
	if err != nil {
		return fmt.Errorf("list opencode sessions: %w", err)
	}

	allSessions := sessions
	sessions = opencode.FilterSessionsForList(sessions, opencodeListAll)

	if opencodeListJSON {
		return encodeJSONToStdout(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println(opencodeEmptyListMessage(len(allSessions), opencodeListAll))
		return nil
	}

	prefixLengths := opencodeSessionPrefixLengths(allSessions)
	fmt.Print(formatOpencodeTable(sessions, ui.HighlightID, time.Now(), prefixLengths))
	return nil
}

func runOpencodeLogs(cmd *cobra.Command, args []string) error {
	store, repoPath, err := openOpencodeStoreAndRepoPath()
	if err != nil {
		return err
	}

	snapshot, err := store.Logs(repoPath, args[0])
	if err != nil {
		return err
	}

	fmt.Print(snapshot)
	return nil
}

func formatOpencodeTable(sessions []opencode.OpencodeSession, highlight func(string, int) string, now time.Time, prefixLengths map[string]int) string {
	rows := make([][]string, 0, len(sessions))
	highlight, prefixLengths = normalizeOpencodeTableInputs(sessions, highlight, prefixLengths)

	for _, session := range sessions {
		age := formatOpencodeAge(session, now)
		duration := formatOpencodeDuration(session, now)
		exit := "-"
		if session.ExitCode != nil {
			exit = strconv.Itoa(*session.ExitCode)
		}
		prefixLen := ui.PrefixLength(prefixLengths, session.ID)

		rows = append(rows, []string{
			highlight(session.ID, prefixLen),
			string(session.Status),
			age,
			duration,
			exit,
		})
	}

	return ui.FormatTable([]string{"SESSION", "STATUS", "AGE", "DURATION", "EXIT"}, rows)
}

func opencodeSessionPrefixLengths(sessions []opencode.OpencodeSession) map[string]int {
	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}
	return ui.UniqueIDPrefixLengths(sessionIDs)
}

func normalizeOpencodeTableInputs(sessions []opencode.OpencodeSession, highlight func(string, int) string, prefixLengths map[string]int) (func(string, int) string, map[string]int) {
	if highlight == nil {
		highlight = func(value string, prefix int) string { return value }
	}
	if prefixLengths == nil {
		prefixLengths = opencodeSessionPrefixLengths(sessions)
	}
	return highlight, prefixLengths
}

func formatOpencodeAge(session opencode.OpencodeSession, now time.Time) string {
	return formatOptionalDuration(opencode.AgeData(session, now))
}

func formatOpencodeDuration(session opencode.OpencodeSession, now time.Time) string {
	return formatOptionalDuration(opencode.DurationData(session, now))
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

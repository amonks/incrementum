package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
	internalopencode "github.com/amonks/incrementum/internal/opencode"
	"github.com/amonks/incrementum/internal/ui"
	"github.com/amonks/incrementum/workspace"
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
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getOpencodeRepoPath()
	if err != nil {
		return err
	}

	sessions, err := pool.ListOpencodeSessions(repoPath)
	if err != nil {
		return fmt.Errorf("list opencode sessions: %w", err)
	}

	allSessions := sessions
	sessions = filterOpencodeSessionsForList(sessions, opencodeListAll)

	if opencodeListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
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
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getOpencodeRepoPath()
	if err != nil {
		return err
	}

	session, err := pool.FindOpencodeSession(repoPath, args[0])
	if err != nil {
		return err
	}

	storage, err := opencodeStorage()
	if err != nil {
		return err
	}

	snapshot, err := opencodeLogSnapshot(storage, session.ID)
	if err != nil {
		return err
	}

	fmt.Print(snapshot)
	return nil
}

func formatOpencodeTable(sessions []workspace.OpencodeSession, highlight func(string, int) string, now time.Time, prefixLengths map[string]int) string {
	if highlight == nil {
		highlight = func(value string, prefix int) string { return value }
	}

	rows := make([][]string, 0, len(sessions))
	if prefixLengths == nil {
		prefixLengths = opencodeSessionPrefixLengths(sessions)
	}
	promptWidth := opencodePromptColumnWidth(sessions, highlight, now, prefixLengths)
	promptHeader := "PROMPT"
	if promptWidth > 0 {
		promptHeader = ui.TruncateTableCellToWidth(promptHeader, promptWidth)
	} else {
		promptHeader = ""
	}

	for _, session := range sessions {
		prompt := opencodePromptLine(session.Prompt)
		prompt = ui.TruncateTableCellToWidth(prompt, promptWidth)
		age := formatOpencodeAge(session, now)
		duration := formatOpencodeDuration(session, now)
		exit := "-"
		if session.ExitCode != nil {
			exit = strconv.Itoa(*session.ExitCode)
		}
		prefixLen := prefixLengths[strings.ToLower(session.ID)]

		rows = append(rows, []string{
			highlight(session.ID, prefixLen),
			string(session.Status),
			age,
			duration,
			prompt,
			exit,
		})
	}

	return ui.FormatTable([]string{"SESSION", "STATUS", "AGE", "DURATION", promptHeader, "EXIT"}, rows)
}

func opencodePromptColumnWidth(sessions []workspace.OpencodeSession, highlight func(string, int) string, now time.Time, prefixLengths map[string]int) int {
	viewportWidth := ui.TableViewportWidth()
	if viewportWidth <= 0 {
		return ui.TableCellMaxWidth()
	}

	headers := []string{"SESSION", "STATUS", "AGE", "DURATION", "PROMPT", "EXIT"}
	columnWidths := make([]int, len(headers))
	for i, header := range headers {
		columnWidths[i] = ui.TableCellWidth(header)
	}

	const promptIndex = 4
	for _, session := range sessions {
		age := formatOpencodeAge(session, now)
		duration := formatOpencodeDuration(session, now)
		exit := "-"
		if session.ExitCode != nil {
			exit = strconv.Itoa(*session.ExitCode)
		}
		prefixLen := prefixLengths[strings.ToLower(session.ID)]
		values := []string{
			highlight(session.ID, prefixLen),
			string(session.Status),
			age,
			duration,
			"",
			exit,
		}

		for i, value := range values {
			if i == promptIndex {
				continue
			}
			if width := ui.TableCellWidth(value); width > columnWidths[i] {
				columnWidths[i] = width
			}
		}
	}

	fixedWidth := 0
	for i, width := range columnWidths {
		if i == promptIndex {
			continue
		}
		fixedWidth += width
	}

	padding := 2 * (len(headers) - 1)
	remaining := viewportWidth - fixedWidth - padding
	if remaining <= 0 {
		return 0
	}

	if maxWidth := ui.TableCellMaxWidth(); maxWidth > 0 && remaining > maxWidth {
		remaining = maxWidth
	}
	return remaining
}

func opencodeSessionPrefixLengths(sessions []workspace.OpencodeSession) map[string]int {
	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}
	return ui.UniqueIDPrefixLengths(sessionIDs)
}

func opencodePromptLine(prompt string) string {
	if prompt == "" {
		return "-"
	}
	line := strings.SplitN(prompt, "\n", 2)[0]
	line = strings.TrimSuffix(line, "\r")
	if strings.TrimSpace(line) == "" {
		return "-"
	}
	return line
}

func formatOpencodeAge(session workspace.OpencodeSession, now time.Time) string {
	age, ok := workspace.AgeData(session, now)
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(age)
}

func formatOpencodeDuration(session workspace.OpencodeSession, now time.Time) string {
	duration, ok := workspace.DurationData(session, now)
	if !ok {
		return "-"
	}
	return ui.FormatDurationShort(duration)
}

func opencodeLogSnapshot(storage internalopencode.Storage, sessionID string) (string, error) {
	snapshot, err := storage.SessionLogText(sessionID)
	if err != nil {
		return "", err
	}
	return snapshot, nil
}

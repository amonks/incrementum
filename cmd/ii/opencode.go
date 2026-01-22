package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/listflags"
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

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	sessions, err := pool.ListOpencodeSessions(repoPath)
	if err != nil {
		return fmt.Errorf("list opencode sessions: %w", err)
	}

	sessions = filterOpencodeSessionsForList(sessions, opencodeListAll)

	if opencodeListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println("No opencode sessions found.")
		return nil
	}

	fmt.Print(formatOpencodeTable(sessions, ui.HighlightID, time.Now()))
	return nil
}

func filterOpencodeSessionsForList(sessions []workspace.OpencodeSession, includeAll bool) []workspace.OpencodeSession {
	if includeAll {
		return sessions
	}

	filtered := sessions[:0]
	for _, session := range sessions {
		if session.Status != workspace.OpencodeSessionActive {
			continue
		}
		filtered = append(filtered, session)
	}
	return filtered
}

func runOpencodeLogs(cmd *cobra.Command, args []string) error {
	pool, err := workspace.Open()
	if err != nil {
		return err
	}

	repoPath, err := getRepoPath()
	if err != nil {
		return err
	}

	logPath, err := opencodeSessionLogPath(pool, repoPath, args[0])
	if err != nil {
		return err
	}

	snapshot, err := opencodeLogSnapshot(logPath)
	if err != nil {
		return err
	}

	fmt.Print(snapshot)
	return nil
}

func formatOpencodeTable(sessions []workspace.OpencodeSession, highlight func(string, int) string, now time.Time) string {
	if highlight == nil {
		highlight = func(value string, prefix int) string { return value }
	}

	rows := make([][]string, 0, len(sessions))
	sessionIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
	}
	prefixLengths := ui.UniqueIDPrefixLengths(sessionIDs)

	for _, session := range sessions {
		prompt := opencodePromptLine(session.Prompt)
		prompt = truncateTableCell(prompt)
		age := formatOpencodeAge(session, now)
		exit := "-"
		if session.ExitCode != nil {
			exit = strconv.Itoa(*session.ExitCode)
		}
		prefixLen := prefixLengths[strings.ToLower(session.ID)]

		rows = append(rows, []string{
			highlight(session.ID, prefixLen),
			string(session.Status),
			age,
			prompt,
			exit,
		})
	}

	return formatTable([]string{"SESSION", "STATUS", "AGE", "PROMPT", "EXIT"}, rows)
}

func opencodePromptLine(prompt string) string {
	if prompt == "" {
		return "-"
	}
	line := strings.SplitN(prompt, "\n", 2)[0]
	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		return "-"
	}
	return line
}

func formatOpencodeAge(session workspace.OpencodeSession, now time.Time) string {
	if session.StartedAt.IsZero() && session.DurationSeconds == 0 {
		return "-"
	}
	return ui.FormatDurationShort(workspace.OpencodeSessionAge(session, now))
}

func opencodeSessionLogPath(pool *workspace.Pool, repoPath, sessionID string) (string, error) {
	session, err := pool.FindOpencodeSession(repoPath, sessionID)
	if err != nil {
		return "", err
	}
	if session.LogPath == "" {
		return "", fmt.Errorf("opencode session log path missing")
	}
	return session.LogPath, nil
}

func opencodeLogSnapshot(logPath string) (string, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("read opencode log: %w", err)
	}
	return string(data), nil
}

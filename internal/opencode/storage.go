package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/amonks/incrementum/internal/paths"
	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// Storage represents the opencode data directory.
type Storage struct {
	Root string
}

// SessionMetadata describes an opencode session stored on disk.
type SessionMetadata struct {
	ID        string
	ProjectID string
	Directory string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LogEntry is a single textual entry extracted from session storage.
type LogEntry struct {
	ID   string
	Text string
}

const toolOutputIndent = 4

var errSessionNotFound = errors.New("opencode session not found")

// DefaultRoot returns the default opencode data directory.
func DefaultRoot() (string, error) {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "opencode"), nil
	}
	home, err := paths.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "opencode"), nil
}

// FindSessionForRun locates the session created for the run.
func (s Storage) FindSessionForRun(repoPath string, startedAt time.Time, prompt string) (SessionMetadata, error) {
	entries, err := s.sessionsForRepo(repoPath)
	if err != nil {
		return SessionMetadata{}, err
	}
	return s.selectSession(entries, repoPath, startedAt, prompt)
}

// FindSessionForRunWithRetry locates the session created for the run with retries.
func (s Storage) FindSessionForRunWithRetry(repoPath string, startedAt time.Time, prompt string, timeout time.Duration) (SessionMetadata, error) {
	if timeout <= 0 {
		return s.FindSessionForRun(repoPath, startedAt, prompt)
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		metadata, err := s.FindSessionForRun(repoPath, startedAt, prompt)
		if err == nil {
			return metadata, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return SessionMetadata{}, lastErr
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// SessionLogText returns the session transcript as a single string.
func (s Storage) SessionLogText(sessionID string) (string, error) {
	return s.sessionText(sessionID, extractPartText)
}

// SessionProseLogText returns the session transcript without tool output.
func (s Storage) SessionProseLogText(sessionID string) (string, error) {
	return s.sessionText(sessionID, extractProsePartText)
}

// SessionLogEntries returns textual log entries for a session.
func (s Storage) SessionLogEntries(sessionID string) ([]LogEntry, error) {
	entries := make([]LogEntry, 0)
	if err := s.forEachSessionPart(sessionID, func(part partInfo) error {
		text, ok := extractPartText(part)
		if !ok {
			return nil
		}
		entries = append(entries, LogEntry{ID: part.ID, Text: text})
		return nil
	}); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s Storage) sessionText(sessionID string, extract func(partInfo) (string, bool)) (string, error) {
	var builder strings.Builder
	if err := s.forEachSessionPart(sessionID, func(part partInfo) error {
		text, ok := extract(part)
		if !ok {
			return nil
		}
		builder.WriteString(text)
		return nil
	}); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func (s Storage) forEachSessionPart(sessionID string, fn func(partInfo) error) error {
	messages, err := s.listMessages(sessionID)
	if err != nil {
		return err
	}
	for _, message := range messages {
		parts, err := s.listParts(message.ID)
		if err != nil {
			return err
		}
		for _, part := range parts {
			if err := fn(part); err != nil {
				return err
			}
		}
	}
	return nil
}

type projectRecord struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
}

func (s Storage) projectIDsForRepo(repoPath string) ([]string, error) {
	projectsDir := filepath.Join(s.storageDir(), "project")
	entries, err := listJSONEntries(projectsDir, "opencode projects")
	if err != nil {
		return nil, err
	}

	repoPath = cleanPath(repoPath)
	ids := make([]string, 0)
	for _, entry := range entries {
		var record projectRecord
		if err := decodeJSONEntry(projectsDir, "opencode project", entry, &record); err != nil {
			return nil, err
		}
		if cleanPath(record.Worktree) != repoPath {
			continue
		}
		ids = append(ids, storageRecordID(record.ID, entry.Name()))
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("opencode project not found")
	}
	return ids, nil
}

type sessionRecord struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

func (s Storage) listSessions(projectID string) ([]SessionMetadata, error) {
	sessionsDir := filepath.Join(s.storageDir(), "session", projectID)
	entries, err := listJSONEntries(sessionsDir, "opencode sessions")
	if err != nil {
		return nil, err
	}

	items := make([]SessionMetadata, 0, len(entries))
	for _, entry := range entries {
		var record sessionRecord
		if err := decodeJSONEntry(sessionsDir, "opencode session", entry, &record); err != nil {
			return nil, err
		}
		id := storageRecordID(record.ID, entry.Name())
		item := SessionMetadata{
			ID:        id,
			ProjectID: record.ProjectID,
			Directory: record.Directory,
		}
		item.CreatedAt = timeFromMillis(record.Time.Created)
		item.UpdatedAt = timeFromMillis(record.Time.Updated)
		items = append(items, item)
	}
	return items, nil
}

func (s Storage) sessionsForRepo(repoPath string) ([]SessionMetadata, error) {
	projectIDs, err := s.projectIDsForRepo(repoPath)
	if err == nil && len(projectIDs) > 0 {
		entries := make([]SessionMetadata, 0)
		for _, projectID := range projectIDs {
			projectSessions, err := s.listSessions(projectID)
			if err != nil {
				return nil, err
			}
			entries = append(entries, projectSessions...)
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}

	entries, readErr := s.listAllSessions()
	if readErr != nil {
		return nil, readErr
	}
	return entries, nil
}

func (s Storage) listAllSessions() ([]SessionMetadata, error) {
	baseDir := filepath.Join(s.storageDir(), "session")
	projects, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read opencode sessions: %w", err)
	}
	var entries []SessionMetadata
	for _, entry := range projects {
		if !entry.IsDir() {
			continue
		}
		projectSessions, err := s.listSessions(entry.Name())
		if err != nil {
			return nil, err
		}
		entries = append(entries, projectSessions...)
	}
	return entries, nil
}

func (s Storage) selectSession(entries []SessionMetadata, repoPath string, startedAt time.Time, prompt string) (SessionMetadata, error) {
	repoPath = cleanPath(repoPath)
	cutoff := startedAt.Add(-5 * time.Second)
	trimmedPrompt := internalstrings.TrimSpace(prompt)
	hasPrompt := trimmedPrompt != ""

	repoEntries := entries
	if repoPath != "" {
		repoEntries = make([]SessionMetadata, 0, len(entries))
		for _, session := range entries {
			if !sessionMatchesRepo(session, repoPath) {
				continue
			}
			repoEntries = append(repoEntries, session)
		}
	}

	var candidates []SessionMetadata
	for _, session := range repoEntries {
		if !sessionAfterCutoff(session, cutoff) {
			continue
		}
		candidates = append(candidates, session)
	}

	if len(candidates) == 0 && len(repoEntries) > 0 {
		if hasPrompt {
			match, err := s.findPromptMatch(repoEntries, repoPath, trimmedPrompt)
			if err != nil {
				return SessionMetadata{}, err
			}
			if match != nil {
				return *match, nil
			}
		}
		latest := latestSession(repoEntries, repoPath)
		if latest != nil {
			return *latest, nil
		}
	}

	if len(candidates) == 0 {
		repoLabel := repoPath
		if repoLabel == "" {
			repoLabel = "unknown"
		}
		return SessionMetadata{}, fmt.Errorf(
			"%w (repo=%s started=%s cutoff=%s total=%d storage=%s)",
			errSessionNotFound,
			repoLabel,
			formatTimeLabel(startedAt),
			formatTimeLabel(cutoff),
			len(entries),
			s.storageDir(),
		)
	}

	sortByCreatedAt(candidates, func(item SessionMetadata) time.Time {
		return sessionSortTime(item, cutoff)
	}, func(item SessionMetadata) string {
		return item.ID
	})

	if hasPrompt {
		for _, session := range candidates {
			matches, err := s.sessionContainsPrompt(session.ID, trimmedPrompt)
			if err != nil {
				return SessionMetadata{}, err
			}
			if matches {
				return session, nil
			}
		}
	}

	return candidates[0], nil
}

func sessionAfterCutoff(session SessionMetadata, cutoff time.Time) bool {
	if cutoff.IsZero() {
		return true
	}
	created := session.CreatedAt
	updated := session.UpdatedAt
	if created.IsZero() && updated.IsZero() {
		return true
	}
	if !created.IsZero() && !created.Before(cutoff) {
		return true
	}
	if !updated.IsZero() && !updated.Before(cutoff) {
		return true
	}
	return false
}

func sessionSortTime(session SessionMetadata, cutoff time.Time) time.Time {
	if !session.CreatedAt.IsZero() {
		if !cutoff.IsZero() && session.CreatedAt.Before(cutoff) && !session.UpdatedAt.IsZero() {
			return session.UpdatedAt
		}
		return session.CreatedAt
	}
	return session.UpdatedAt
}

func sessionLatestTime(session SessionMetadata) time.Time {
	if session.CreatedAt.IsZero() {
		return session.UpdatedAt
	}
	if session.UpdatedAt.IsZero() {
		return session.CreatedAt
	}
	if session.UpdatedAt.After(session.CreatedAt) {
		return session.UpdatedAt
	}
	return session.CreatedAt
}

func sessionMatchesRepo(session SessionMetadata, repoPath string) bool {
	if repoPath == "" {
		return true
	}
	if session.Directory == "" {
		return true
	}
	sessionPath := cleanPath(session.Directory)
	if sessionPath == repoPath {
		return true
	}
	if pathContains(repoPath, sessionPath) {
		return true
	}
	if pathContains(sessionPath, repoPath) {
		return true
	}
	return false
}

func pathContains(base, target string) bool {
	if base == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func (s Storage) findPromptMatch(entries []SessionMetadata, repoPath, prompt string) (*SessionMetadata, error) {
	var match SessionMetadata
	found := false
	for _, session := range entries {
		if !sessionMatchesRepo(session, repoPath) {
			continue
		}
		matches, err := s.sessionContainsPrompt(session.ID, prompt)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}
		if !found || sessionLatestTime(session).After(sessionLatestTime(match)) {
			match = session
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	return &match, nil
}

func latestSession(entries []SessionMetadata, repoPath string) *SessionMetadata {
	var match SessionMetadata
	found := false
	for _, session := range entries {
		if !sessionMatchesRepo(session, repoPath) {
			continue
		}
		if !found || sessionLatestTime(session).After(sessionLatestTime(match)) {
			match = session
			found = true
		}
	}
	if !found {
		return nil
	}
	return &match
}

type messageRecord struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	Time struct {
		Created int64 `json:"created"`
	} `json:"time"`
}

type messageInfo struct {
	ID        string
	Role      string
	CreatedAt time.Time
}

func (s Storage) listMessages(sessionID string) ([]messageInfo, error) {
	messageDir := filepath.Join(s.storageDir(), "message", sessionID)
	entries, err := listJSONEntries(messageDir, "opencode messages")
	if err != nil {
		return nil, err
	}

	items := make([]messageInfo, 0, len(entries))
	for _, entry := range entries {
		var record messageRecord
		if err := decodeJSONEntry(messageDir, "opencode message", entry, &record); err != nil {
			return nil, err
		}
		id := storageRecordID(record.ID, entry.Name())
		item := messageInfo{ID: id, Role: record.Role}
		item.CreatedAt = timeFromMillis(record.Time.Created)
		items = append(items, item)
	}

	sortByCreatedAt(items, func(item messageInfo) time.Time {
		return item.CreatedAt
	}, func(item messageInfo) string {
		return item.ID
	})
	return items, nil
}

type partRecord struct {
	ID    string         `json:"id"`
	Type  string         `json:"type"`
	Text  string         `json:"text"`
	Tool  string         `json:"tool"`
	State map[string]any `json:"state"`
	Time  struct {
		Start int64 `json:"start"`
	} `json:"time"`
}

type partInfo struct {
	ID      string
	Type    string
	Text    string
	Tool    string
	State   map[string]any
	SortKey int64
	Name    string
}

func (s Storage) listParts(messageID string) ([]partInfo, error) {
	partDir := filepath.Join(s.storageDir(), "part", messageID)
	entries, err := listJSONEntries(partDir, "opencode parts")
	if err != nil {
		return nil, err
	}

	items := make([]partInfo, 0, len(entries))
	for _, entry := range entries {
		var record partRecord
		if err := decodeJSONEntry(partDir, "opencode part", entry, &record); err != nil {
			return nil, err
		}
		id := storageRecordID(record.ID, entry.Name())
		info := partInfo{
			ID:    id,
			Type:  record.Type,
			Text:  record.Text,
			Tool:  record.Tool,
			State: record.State,
			Name:  entry.Name(),
		}
		if record.Time.Start != 0 {
			info.SortKey = record.Time.Start
		}
		if info.SortKey == 0 {
			if entryInfo, err := entry.Info(); err == nil {
				info.SortKey = entryInfo.ModTime().UnixNano()
			}
		}
		items = append(items, info)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].SortKey != items[j].SortKey {
			if items[i].SortKey == 0 {
				return false
			}
			if items[j].SortKey == 0 {
				return true
			}
			return items[i].SortKey < items[j].SortKey
		}
		leftRank := partTypeRank(items[i].Type)
		rightRank := partTypeRank(items[j].Type)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s Storage) sessionContainsPrompt(sessionID, prompt string) (bool, error) {
	messages, err := s.listMessages(sessionID)
	if err != nil {
		return false, err
	}
	for _, message := range messages {
		if internalstrings.NormalizeLower(message.Role) != "user" {
			continue
		}
		text, err := s.messageText(message.ID)
		if err != nil {
			return false, err
		}
		if strings.Contains(text, prompt) {
			return true, nil
		}
	}
	return false, nil
}

func (s Storage) messageText(messageID string) (string, error) {
	parts, err := s.listParts(messageID)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	appendPartText(&builder, parts, extractPartText)
	return builder.String(), nil
}

func appendPartText(builder *strings.Builder, parts []partInfo, extract func(partInfo) (string, bool)) {
	if builder == nil {
		return
	}
	for _, part := range parts {
		text, ok := extract(part)
		if !ok {
			continue
		}
		builder.WriteString(text)
	}
}

func extractPartText(part partInfo) (string, bool) {
	switch normalizePartType(part.Type) {
	case "text":
		if part.Text == "" {
			return "", false
		}
		return part.Text, true
	case "tool":
		output, ok := part.State["output"]
		if !ok {
			return "", false
		}
		stdout, stderr, ok := extractToolOutput(output)
		if !ok {
			return "", false
		}
		text := formatToolOutput(stdout, stderr)
		if text == "" {
			return "", false
		}
		return text, true
	case "reasoning":
		return "", false
	default:
		if part.Text != "" {
			return part.Text, true
		}
		return "", false
	}
}

func extractProsePartText(part partInfo) (string, bool) {
	partType := normalizePartType(part.Type)
	if partType != "text" && partType != "" {
		return "", false
	}
	if part.Text == "" {
		return "", false
	}
	return part.Text, true
}

func normalizePartType(partType string) string {
	return internalstrings.NormalizeLower(partType)
}

func partTypeRank(partType string) int {
	switch normalizePartType(partType) {
	case "tool":
		return 0
	case "text":
		return 1
	default:
		return 2
	}
}

func stringifyOutput(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", false
		}
		return typed, true
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return "", false
		}
		text := string(data)
		if text == "" || text == "null" {
			return "", false
		}
		return text, true
	}
}

func extractToolOutput(value any) (string, string, bool) {
	if value == nil {
		return "", "", false
	}
	switch typed := value.(type) {
	case map[string]any:
		stdout, _ := stringifyOutput(typed["stdout"])
		stderr, _ := stringifyOutput(typed["stderr"])
		if stdout == "" && stderr == "" {
			return "", "", false
		}
		return stdout, stderr, true
	case map[string]string:
		stdout := typed["stdout"]
		stderr := typed["stderr"]
		if stdout == "" && stderr == "" {
			return "", "", false
		}
		return stdout, stderr, true
	default:
		text, ok := stringifyOutput(value)
		if !ok {
			return "", "", false
		}
		return text, "", true
	}
}

func formatToolOutput(stdout, stderr string) string {
	sections := make([]string, 0, 2)
	if section := formatToolOutputSection("Stdout:", stdout); section != "" {
		sections = append(sections, section)
	}
	if section := formatToolOutputSection("Stderr:", stderr); section != "" {
		sections = append(sections, section)
	}
	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n") + "\n"
}

func formatToolOutputSection(label, text string) string {
	text = internalstrings.TrimTrailingNewlines(text)
	if internalstrings.IsBlank(text) {
		return ""
	}
	return label + "\n" + indentToolOutput(text)
}

func indentToolOutput(text string) string {
	return internalstrings.IndentBlock(text, toolOutputIndent)
}

func decodeJSONFile(path, label string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", label, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

func decodeJSONEntry(dir, label string, entry os.DirEntry, dest any) error {
	path := filepath.Join(dir, entry.Name())
	return decodeJSONFile(path, label, dest)
}

func listJSONEntries(dir, label string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	filtered := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
}

func storageRecordID(recordID, filename string) string {
	if recordID != "" {
		return recordID
	}
	return strings.TrimSuffix(filename, ".json")
}

func (s Storage) storageDir() string {
	return filepath.Join(s.Root, "storage")
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

func formatTimeLabel(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format(time.RFC3339Nano)
}

func timeFromMillis(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	absValue := value
	if absValue < 0 {
		absValue = -absValue
	}
	if absValue < 1e11 {
		return time.Unix(value, 0)
	}
	if absValue < 1e14 {
		return time.UnixMilli(value)
	}
	if absValue < 1e17 {
		return time.UnixMicro(value)
	}
	return time.Unix(0, value)
}

func sortByCreatedAt[T any](items []T, createdAt func(T) time.Time, tiebreak func(T) string) {
	sort.Slice(items, func(i, j int) bool {
		left := createdAt(items[i])
		right := createdAt(items[j])
		if left.Equal(right) {
			return tiebreak(items[i]) < tiebreak(items[j])
		}
		if left.IsZero() {
			return false
		}
		if right.IsZero() {
			return true
		}
		return left.Before(right)
	})
}

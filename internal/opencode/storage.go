package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// DefaultRoot returns the default opencode data directory.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
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
	entries, err := s.SessionLogEntries(sessionID)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(entry.Text)
	}
	return builder.String(), nil
}

// SessionProseLogText returns the session transcript without tool output.
func (s Storage) SessionProseLogText(sessionID string) (string, error) {
	messages, err := s.listMessages(sessionID)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, message := range messages {
		parts, err := s.listParts(message.ID)
		if err != nil {
			return "", err
		}
		for _, part := range parts {
			text, ok := extractProsePartText(part)
			if !ok {
				continue
			}
			builder.WriteString(text)
		}
	}
	return builder.String(), nil
}

// SessionLogEntries returns textual log entries for a session.
func (s Storage) SessionLogEntries(sessionID string) ([]LogEntry, error) {
	messages, err := s.listMessages(sessionID)
	if err != nil {
		return nil, err
	}

	entries := make([]LogEntry, 0)
	for _, message := range messages {
		parts, err := s.listParts(message.ID)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			text, ok := extractPartText(part)
			if !ok {
				continue
			}
			entries = append(entries, LogEntry{ID: part.ID, Text: text})
		}
	}
	return entries, nil
}

type projectRecord struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
}

func (s Storage) projectIDForRepo(repoPath string) (string, error) {
	projectsDir := filepath.Join(s.storageDir(), "project")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", fmt.Errorf("read opencode projects: %w", err)
	}

	repoPath = cleanPath(repoPath)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(projectsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read opencode project: %w", err)
		}
		var record projectRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return "", fmt.Errorf("decode opencode project: %w", err)
		}
		if cleanPath(record.Worktree) != repoPath {
			continue
		}
		if record.ID != "" {
			return record.ID, nil
		}
		return strings.TrimSuffix(entry.Name(), ".json"), nil
	}
	return "", fmt.Errorf("opencode project not found")
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
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("read opencode sessions: %w", err)
	}

	items := make([]SessionMetadata, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(sessionsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read opencode session: %w", err)
		}
		var record sessionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode opencode session: %w", err)
		}
		id := record.ID
		if id == "" {
			id = strings.TrimSuffix(entry.Name(), ".json")
		}
		item := SessionMetadata{
			ID:        id,
			ProjectID: record.ProjectID,
			Directory: record.Directory,
		}
		if record.Time.Created != 0 {
			item.CreatedAt = time.UnixMilli(record.Time.Created)
		}
		if record.Time.Updated != 0 {
			item.UpdatedAt = time.UnixMilli(record.Time.Updated)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s Storage) sessionsForRepo(repoPath string) ([]SessionMetadata, error) {
	projectID, err := s.projectIDForRepo(repoPath)
	if err == nil {
		return s.listSessions(projectID)
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

	var candidates []SessionMetadata
	for _, session := range entries {
		if session.Directory != "" && cleanPath(session.Directory) != repoPath {
			continue
		}
		if !session.CreatedAt.IsZero() && session.CreatedAt.Before(cutoff) {
			continue
		}
		candidates = append(candidates, session)
	}
	if len(candidates) == 0 {
		for _, session := range entries {
			if !session.CreatedAt.IsZero() && session.CreatedAt.Before(cutoff) {
				continue
			}
			candidates = append(candidates, session)
		}
	}

	if len(candidates) == 0 {
		return SessionMetadata{}, fmt.Errorf("opencode session not found")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].ID < candidates[j].ID
		}
		if candidates[i].CreatedAt.IsZero() {
			return false
		}
		if candidates[j].CreatedAt.IsZero() {
			return true
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})

	trimmedPrompt := strings.TrimSpace(prompt)
	if trimmedPrompt != "" {
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
	entries, err := os.ReadDir(messageDir)
	if err != nil {
		return nil, fmt.Errorf("read opencode messages: %w", err)
	}

	items := make([]messageInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(messageDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read opencode message: %w", err)
		}
		var record messageRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode opencode message: %w", err)
		}
		id := record.ID
		if id == "" {
			id = strings.TrimSuffix(entry.Name(), ".json")
		}
		item := messageInfo{ID: id, Role: record.Role}
		if record.Time.Created != 0 {
			item.CreatedAt = time.UnixMilli(record.Time.Created)
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		if items[i].CreatedAt.IsZero() {
			return false
		}
		if items[j].CreatedAt.IsZero() {
			return true
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
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
	entries, err := os.ReadDir(partDir)
	if err != nil {
		return nil, fmt.Errorf("read opencode parts: %w", err)
	}

	items := make([]partInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(partDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read opencode part: %w", err)
		}
		var record partRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode opencode part: %w", err)
		}
		id := record.ID
		if id == "" {
			id = strings.TrimSuffix(entry.Name(), ".json")
		}
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
		if strings.ToLower(message.Role) != "user" {
			continue
		}
		text, err := s.messageText(message.ID)
		if err != nil {
			return false, err
		}
		if strings.Contains(strings.TrimSpace(text), prompt) {
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
	for _, part := range parts {
		text, ok := extractPartText(part)
		if !ok {
			continue
		}
		builder.WriteString(text)
	}
	return builder.String(), nil
}

func extractPartText(part partInfo) (string, bool) {
	switch strings.ToLower(part.Type) {
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
		return stringifyOutput(output)
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
	partType := strings.ToLower(part.Type)
	if partType != "text" && partType != "" {
		return "", false
	}
	if part.Text == "" {
		return "", false
	}
	return part.Text, true
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
		text := strings.TrimSpace(string(data))
		if text == "" || text == "null" {
			return "", false
		}
		return string(data), true
	}
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
	return filepath.Clean(path)
}

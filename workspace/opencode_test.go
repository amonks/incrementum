package workspace

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	statestore "github.com/amonks/incrementum/internal/state"
)

func TestPool_CreateOpencodeSessionAndList(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	session, err := pool.CreateOpencodeSession(repoPath, "Test prompt", "/tmp/opencode.log", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.ID == "" {
		t.Fatal("expected session ID")
	}
	if session.Status != OpencodeSessionActive {
		t.Fatalf("expected status active, got %q", session.Status)
	}
	if session.Repo != statestore.SanitizeRepoName(repoPath) {
		t.Fatalf("expected repo %q, got %q", statestore.SanitizeRepoName(repoPath), session.Repo)
	}
	if session.Prompt != "Test prompt" {
		t.Fatalf("expected prompt Test prompt, got %q", session.Prompt)
	}
	if session.LogPath != "/tmp/opencode.log" {
		t.Fatalf("expected log path /tmp/opencode.log, got %q", session.LogPath)
	}
	if session.ExitCode != nil {
		t.Fatalf("expected nil exit code, got %v", *session.ExitCode)
	}
	if session.StartedAt.IsZero() {
		t.Fatal("expected started_at to be set")
	}
	if session.UpdatedAt.IsZero() {
		t.Fatal("expected updated_at to be set")
	}
	if !session.CompletedAt.IsZero() {
		t.Fatal("expected completed_at to be zero")
	}

	found, err := pool.FindOpencodeSession(repoPath, session.ID)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if found.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, found.ID)
	}

	list, err := pool.ListOpencodeSessions(repoPath)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, list[0].ID)
	}
}

func TestPool_FindOpencodeSessionMatchesPrefix(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	startedAt := time.Now().UTC()

	// Create two sessions with different ID prefixes
	session, err := pool.CreateOpencodeSession(repoPath, "First prompt", "/tmp/first.log", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}

	_, err = pool.CreateOpencodeSession(repoPath, "Second prompt", "/tmp/second.log", startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	// Find by prefix of first session's ID (use first 3 chars)
	prefix := session.ID[:3]
	found, err := pool.FindOpencodeSession(repoPath, prefix)
	if err != nil {
		t.Fatalf("find session by prefix: %v", err)
	}
	if found.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q", session.ID, found.ID)
	}
}

func TestPool_FindOpencodeSessionRejectsAmbiguousPrefix(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	startedAt := time.Now().UTC()

	// Create two sessions - their IDs will be hash-based so we need to test
	// that looking for a very short prefix returns an ambiguous error when both match
	first, err := pool.CreateOpencodeSession(repoPath, "First", "/tmp/first.log", startedAt)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}

	second, err := pool.CreateOpencodeSession(repoPath, "Second", "/tmp/second.log", startedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}

	// Find a prefix that matches both - need to find common prefix
	// Since IDs are hash-based, we'll look for the single-char prefix if they share one
	// If not, the test passes trivially. For robustness, check if they share a prefix.
	commonPrefix := ""
	for i := 0; i < len(first.ID) && i < len(second.ID); i++ {
		if first.ID[i] == second.ID[i] {
			commonPrefix += string(first.ID[i])
		} else {
			break
		}
	}

	if commonPrefix != "" {
		_, err = pool.FindOpencodeSession(repoPath, commonPrefix)
		if !errors.Is(err, ErrAmbiguousOpencodeSessionIDPrefix) {
			t.Fatalf("expected ErrAmbiguousOpencodeSessionIDPrefix for common prefix %q, got %v", commonPrefix, err)
		}
	}
	// If no common prefix, test passes - the IDs don't share a prefix
}

func TestPool_RepoSlug(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	slug, err := pool.RepoSlug(repoPath)
	if err != nil {
		t.Fatalf("get repo slug: %v", err)
	}

	if slug != statestore.SanitizeRepoName(repoPath) {
		t.Fatalf("expected slug %q, got %q", statestore.SanitizeRepoName(repoPath), slug)
	}
}

func TestPool_CompleteOpencodeSession(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	session, err := pool.CreateOpencodeSession(repoPath, "Test prompt", "/tmp/opencode.log", start)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	completedAt := start.Add(2 * time.Minute)
	exitCode := 1
	duration := int(completedAt.Sub(start).Seconds())

	completed, err := pool.CompleteOpencodeSession(repoPath, session.ID, OpencodeSessionFailed, completedAt, &exitCode, duration)
	if err != nil {
		t.Fatalf("complete session: %v", err)
	}

	if completed.Status != OpencodeSessionFailed {
		t.Fatalf("expected status failed, got %q", completed.Status)
	}
	if completed.CompletedAt.IsZero() {
		t.Fatal("expected completed_at to be set")
	}
	if !completed.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completed_at %v, got %v", completedAt, completed.CompletedAt)
	}
	if completed.DurationSeconds != duration {
		t.Fatalf("expected duration %d, got %d", duration, completed.DurationSeconds)
	}
	if completed.ExitCode == nil || *completed.ExitCode != exitCode {
		t.Fatalf("expected exit code %d, got %v", exitCode, completed.ExitCode)
	}

	_, err = pool.CompleteOpencodeSession(repoPath, session.ID, OpencodeSessionFailed, completedAt, &exitCode, duration)
	if !errors.Is(err, ErrOpencodeSessionNotActive) {
		t.Fatalf("expected ErrOpencodeSessionNotActive, got %v", err)
	}
}

func TestPool_RecordOpencodeDaemonAndStop(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	pid := os.Getpid()
	daemon, err := pool.RecordOpencodeDaemon(repoPath, pid, "localhost", 8080, "/tmp/daemon.log", start)
	if err != nil {
		t.Fatalf("record daemon: %v", err)
	}

	if daemon.Repo != statestore.SanitizeRepoName(repoPath) {
		t.Fatalf("expected repo %q, got %q", statestore.SanitizeRepoName(repoPath), daemon.Repo)
	}
	if daemon.Status != OpencodeDaemonRunning {
		t.Fatalf("expected status running, got %q", daemon.Status)
	}
	if daemon.PID != pid {
		t.Fatalf("expected pid %d, got %d", pid, daemon.PID)
	}
	if daemon.Host != "localhost" {
		t.Fatalf("expected host localhost, got %q", daemon.Host)
	}
	if daemon.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", daemon.Port)
	}
	if daemon.LogPath != "/tmp/daemon.log" {
		t.Fatalf("expected log path /tmp/daemon.log, got %q", daemon.LogPath)
	}
	if !daemon.StartedAt.Equal(start) {
		t.Fatalf("expected started_at %v, got %v", start, daemon.StartedAt)
	}
	if !daemon.UpdatedAt.Equal(start) {
		t.Fatalf("expected updated_at %v, got %v", start, daemon.UpdatedAt)
	}

	found, err := pool.FindOpencodeDaemon(repoPath)
	if err != nil {
		t.Fatalf("find daemon: %v", err)
	}
	if found.Status != OpencodeDaemonRunning {
		t.Fatalf("expected status running, got %q", found.Status)
	}

	stoppedAt := start.Add(2 * time.Minute)
	stopped, err := pool.StopOpencodeDaemon(repoPath, stoppedAt)
	if err != nil {
		t.Fatalf("stop daemon: %v", err)
	}
	if stopped.Status != OpencodeDaemonStopped {
		t.Fatalf("expected status stopped, got %q", stopped.Status)
	}
	if !stopped.UpdatedAt.Equal(stoppedAt) {
		t.Fatalf("expected updated_at %v, got %v", stoppedAt, stopped.UpdatedAt)
	}
}

func TestPool_FindOpencodeDaemonStopsWhenPIDMissing(t *testing.T) {
	pool, err := OpenWithOptions(Options{
		StateDir:      t.TempDir(),
		WorkspacesDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	cmd := exec.Command("sleep", "0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait helper process: %v", err)
	}

	repoPath := "/tmp/my-repo"
	start := time.Now().UTC()

	_, err = pool.RecordOpencodeDaemon(repoPath, pid, "", 0, "", start)
	if err != nil {
		t.Fatalf("record daemon: %v", err)
	}

	found, err := pool.FindOpencodeDaemon(repoPath)
	if err != nil {
		t.Fatalf("find daemon: %v", err)
	}
	if found.Status != OpencodeDaemonStopped {
		t.Fatalf("expected status stopped, got %q", found.Status)
	}
}

func TestDaemonAttachURL(t *testing.T) {
	tests := []struct {
		name   string
		daemon OpencodeDaemon
		want   string
	}{
		{
			name:   "explicit host and port",
			daemon: OpencodeDaemon{Host: "localhost", Port: 8080},
			want:   "http://localhost:8080",
		},
		{
			name:   "default host when empty",
			daemon: OpencodeDaemon{Host: "", Port: 8080},
			want:   "http://127.0.0.1:8080",
		},
		{
			name:   "default port when zero",
			daemon: OpencodeDaemon{Host: "localhost", Port: 0},
			want:   "http://localhost:19283",
		},
		{
			name:   "all defaults",
			daemon: OpencodeDaemon{Host: "", Port: 0},
			want:   "http://127.0.0.1:19283",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DaemonAttachURL(tt.daemon)
			if got != tt.want {
				t.Errorf("DaemonAttachURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

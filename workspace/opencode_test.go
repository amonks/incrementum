package workspace

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
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
	if session.Repo != sanitizeRepoName(repoPath) {
		t.Fatalf("expected repo %q, got %q", sanitizeRepoName(repoPath), session.Repo)
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
	repoName, err := pool.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("get repo name: %v", err)
	}

	startedAt := time.Now().UTC()
	session := OpencodeSession{
		ID:        "abc123",
		Repo:      repoName,
		Status:    OpencodeSessionActive,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	other := OpencodeSession{
		ID:        "def456",
		Repo:      repoName,
		Status:    OpencodeSessionCompleted,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}

	err = pool.stateStore.update(func(st *state) error {
		st.OpencodeSessions[repoName+"/"+session.ID] = session
		st.OpencodeSessions[repoName+"/"+other.ID] = other
		return nil
	})
	if err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	found, err := pool.FindOpencodeSession(repoPath, "ABc")
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
	repoName, err := pool.stateStore.getOrCreateRepoName(repoPath)
	if err != nil {
		t.Fatalf("get repo name: %v", err)
	}

	startedAt := time.Now().UTC()
	first := OpencodeSession{
		ID:        "abc111",
		Repo:      repoName,
		Status:    OpencodeSessionActive,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}
	second := OpencodeSession{
		ID:        "abc222",
		Repo:      repoName,
		Status:    OpencodeSessionCompleted,
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}

	err = pool.stateStore.update(func(st *state) error {
		st.OpencodeSessions[repoName+"/"+first.ID] = first
		st.OpencodeSessions[repoName+"/"+second.ID] = second
		return nil
	})
	if err != nil {
		t.Fatalf("seed sessions: %v", err)
	}

	_, err = pool.FindOpencodeSession(repoPath, "abc")
	if !errors.Is(err, ErrAmbiguousOpencodeSessionIDPrefix) {
		t.Fatalf("expected ErrAmbiguousOpencodeSessionIDPrefix, got %v", err)
	}
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

	if slug != sanitizeRepoName(repoPath) {
		t.Fatalf("expected slug %q, got %q", sanitizeRepoName(repoPath), slug)
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

	if daemon.Repo != sanitizeRepoName(repoPath) {
		t.Fatalf("expected repo %q, got %q", sanitizeRepoName(repoPath), daemon.Repo)
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

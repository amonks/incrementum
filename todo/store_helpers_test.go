package todo

import "testing"

type noopSnapshotter struct{}

func (noopSnapshotter) Snapshot(workspacePath string) error {
	return nil
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	return &Store{
		repoPath: "test",
		wsPath:   t.TempDir(),
		snapshot: noopSnapshotter{},
		wsRelease: func() error {
			return nil
		},
	}
}

func openTestStore(t *testing.T) (*Store, error) {
	t.Helper()
	return newTestStore(t), nil
}

type snapshotterFunc func(string) error

func (fn snapshotterFunc) Snapshot(workspacePath string) error {
	return fn(workspacePath)
}

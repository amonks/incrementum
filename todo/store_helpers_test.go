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

func (s *Store) getTodoByID(id string) (*Todo, error) {
	todos, err := s.readTodos()
	if err != nil {
		return nil, err
	}

	resolved, err := resolveTodoIDsWithTodos([]string{id}, todos)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, ErrTodoNotFound
	}

	for i := range todos {
		if todos[i].ID == resolved[0] {
			return &todos[i], nil
		}
	}

	return nil, ErrTodoNotFound
}

type snapshotterFunc func(string) error

func (fn snapshotterFunc) Snapshot(workspacePath string) error {
	return fn(workspacePath)
}

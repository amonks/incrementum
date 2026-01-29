package habit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("loads habit without frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `# Clean Up

Look for code cleanup opportunities.

## Guidelines

- Remove dead code
- Simplify logic`
		if err := os.WriteFile(filepath.Join(habitsDir, "cleanup.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Load(dir, "cleanup")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if habit.Name != "cleanup" {
			t.Errorf("Name = %q, want %q", habit.Name, "cleanup")
		}
		if habit.Instructions != content {
			t.Errorf("Instructions mismatch:\ngot:\n%s\nwant:\n%s", habit.Instructions, content)
		}
		if habit.ImplementationModel != "" {
			t.Errorf("ImplementationModel = %q, want empty", habit.ImplementationModel)
		}
		if habit.ReviewModel != "" {
			t.Errorf("ReviewModel = %q, want empty", habit.ReviewModel)
		}
	})

	t.Run("loads habit with frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `---
models:
  implementation: claude-sonnet-4
  review: claude-haiku
---

# Performance

Look for performance improvements.`
		if err := os.WriteFile(filepath.Join(habitsDir, "perf.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Load(dir, "perf")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if habit.Name != "perf" {
			t.Errorf("Name = %q, want %q", habit.Name, "perf")
		}
		expectedInstructions := "# Performance\n\nLook for performance improvements."
		if habit.Instructions != expectedInstructions {
			t.Errorf("Instructions mismatch:\ngot:\n%s\nwant:\n%s", habit.Instructions, expectedInstructions)
		}
		if habit.ImplementationModel != "claude-sonnet-4" {
			t.Errorf("ImplementationModel = %q, want %q", habit.ImplementationModel, "claude-sonnet-4")
		}
		if habit.ReviewModel != "claude-haiku" {
			t.Errorf("ReviewModel = %q, want %q", habit.ReviewModel, "claude-haiku")
		}
	})

	t.Run("loads habit with partial frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		content := `---
models:
  implementation: claude-opus-4
---

# Docs

Update documentation.`
		if err := os.WriteFile(filepath.Join(habitsDir, "docs.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		habit, err := Load(dir, "docs")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if habit.ImplementationModel != "claude-opus-4" {
			t.Errorf("ImplementationModel = %q, want %q", habit.ImplementationModel, "claude-opus-4")
		}
		if habit.ReviewModel != "" {
			t.Errorf("ReviewModel = %q, want empty", habit.ReviewModel)
		}
	})

	t.Run("returns error for missing habit", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir, "nonexistent")
		if err == nil {
			t.Error("expected error for missing habit")
		}
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir, "")
		if err == nil {
			t.Error("expected error for empty name")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns empty for no habits directory", func(t *testing.T) {
		dir := t.TempDir()
		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
	})

	t.Run("returns empty for empty habits directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, HabitsDir), 0755); err != nil {
			t.Fatal(err)
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(names) != 0 {
			t.Errorf("got %d names, want 0", len(names))
		}
	})

	t.Run("returns sorted habit names", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create habits in non-alphabetical order
		for _, name := range []string{"zebra", "alpha", "mid"} {
			if err := os.WriteFile(filepath.Join(habitsDir, name+".md"), []byte("# "+name), 0644); err != nil {
				t.Fatal(err)
			}
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		expected := []string{"alpha", "mid", "zebra"}
		if len(names) != len(expected) {
			t.Fatalf("got %d names, want %d", len(names), len(expected))
		}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
			}
		}
	})

	t.Run("ignores non-md files", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(habitsDir, "valid.md"), []byte("# Valid"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(habitsDir, "ignored.txt"), []byte("ignored"), 0644); err != nil {
			t.Fatal(err)
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(names) != 1 || names[0] != "valid" {
			t.Errorf("got names %v, want [valid]", names)
		}
	})

	t.Run("ignores directories", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(habitsDir, "valid.md"), []byte("# Valid"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(habitsDir, "subdir.md"), 0755); err != nil {
			t.Fatal(err)
		}

		names, err := List(dir)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(names) != 1 || names[0] != "valid" {
			t.Errorf("got names %v, want [valid]", names)
		}
	})
}

func TestFirst(t *testing.T) {
	t.Run("returns nil for no habits", func(t *testing.T) {
		dir := t.TempDir()
		habit, err := First(dir)
		if err != nil {
			t.Fatalf("First failed: %v", err)
		}
		if habit != nil {
			t.Error("expected nil habit for empty directory")
		}
	})

	t.Run("returns first habit alphabetically", func(t *testing.T) {
		dir := t.TempDir()
		habitsDir := filepath.Join(dir, HabitsDir)
		if err := os.MkdirAll(habitsDir, 0755); err != nil {
			t.Fatal(err)
		}

		for _, name := range []string{"zebra", "alpha", "mid"} {
			if err := os.WriteFile(filepath.Join(habitsDir, name+".md"), []byte("# "+name), 0644); err != nil {
				t.Fatal(err)
			}
		}

		habit, err := First(dir)
		if err != nil {
			t.Fatalf("First failed: %v", err)
		}

		if habit.Name != "alpha" {
			t.Errorf("Name = %q, want %q", habit.Name, "alpha")
		}
	})
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		wantImpl   string
		wantReview string
	}{
		{
			name:       "empty",
			data:       "",
			wantImpl:   "",
			wantReview: "",
		},
		{
			name: "full models section",
			data: `models:
  implementation: claude-sonnet-4
  review: claude-haiku`,
			wantImpl:   "claude-sonnet-4",
			wantReview: "claude-haiku",
		},
		{
			name: "only implementation",
			data: `models:
  implementation: opus`,
			wantImpl:   "opus",
			wantReview: "",
		},
		{
			name: "only review",
			data: `models:
  review: haiku`,
			wantImpl:   "",
			wantReview: "haiku",
		},
		{
			name: "other fields ignored",
			data: `title: Test
models:
  implementation: sonnet
  review: haiku
other: value`,
			wantImpl:   "sonnet",
			wantReview: "haiku",
		},
		{
			name: "no models section",
			data: `title: Test
description: Something`,
			wantImpl:   "",
			wantReview: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotImpl, gotReview := parseFrontmatter(tt.data)
			if gotImpl != tt.wantImpl {
				t.Errorf("implementation = %q, want %q", gotImpl, tt.wantImpl)
			}
			if gotReview != tt.wantReview {
				t.Errorf("review = %q, want %q", gotReview, tt.wantReview)
			}
		})
	}
}

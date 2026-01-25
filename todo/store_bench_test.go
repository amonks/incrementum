package todo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkReadJSONLFromReader1K(b *testing.B) {
	benchmarkReadJSONLFromReader(b, 1000)
}

func BenchmarkReadJSONLFromReader10K(b *testing.B) {
	benchmarkReadJSONLFromReader(b, 10000)
}

func BenchmarkWriteJSONL1K(b *testing.B) {
	benchmarkWriteJSONL(b, 1000)
}

func BenchmarkWriteJSONL10K(b *testing.B) {
	benchmarkWriteJSONL(b, 10000)
}

func benchmarkReadJSONLFromReader(b *testing.B, count int) {
	todos := make([]Todo, count)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		title := fmt.Sprintf("Todo %d", i)
		todos[i] = Todo{
			ID:          GenerateID(title, now.Add(time.Duration(i)*time.Second)),
			Title:       title,
			Description: "Benchmark payload for todo store.",
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for i := range todos {
		if err := encoder.Encode(todos[i]); err != nil {
			b.Fatalf("encode todo %d: %v", i, err)
		}
	}
	data := buffer.Bytes()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		if _, err := readJSONLFromReader[Todo](reader); err != nil {
			b.Fatalf("read todos: %v", err)
		}
	}
}

func benchmarkWriteJSONL(b *testing.B, count int) {
	todos := make([]Todo, count)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		title := fmt.Sprintf("Todo %d", i)
		todos[i] = Todo{
			ID:          GenerateID(title, now.Add(time.Duration(i)*time.Second)),
			Title:       title,
			Description: "Benchmark payload for todo store.",
			Status:      StatusOpen,
			Priority:    PriorityMedium,
			Type:        TypeTask,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	path := filepath.Join(b.TempDir(), "todos.jsonl")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeJSONL(path, todos); err != nil {
			b.Fatalf("write todos: %v", err)
		}
	}
}

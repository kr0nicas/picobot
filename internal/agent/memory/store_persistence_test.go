package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryPersistence_ReadWriteLongAndToday(t *testing.T) {
	tmp := t.TempDir()
	s := NewMemoryStoreWithWorkspace(tmp, 10)

	// Write long-term
	if err := s.WriteLongTerm("Long-term fact\n"); err != nil {
		t.Fatalf("WriteLongTerm error: %v", err)
	}
	lt, err := s.ReadLongTerm()
	if err != nil {
		t.Fatalf("ReadLongTerm error: %v", err)
	}
	if lt != "Long-term fact\n" {
		t.Fatalf("unexpected long-term content: %q", lt)
	}

	// Append today
	if err := s.AppendToday("note 1"); err != nil {
		t.Fatalf("AppendToday error: %v", err)
	}
	// ensure file exists
	files, _ := os.ReadDir(filepath.Join(tmp, "memory"))
	if len(files) == 0 {
		t.Fatalf("expected memory file created")
	}

	td, err := s.ReadToday()
	if err != nil {
		t.Fatalf("ReadToday error: %v", err)
	}
	if td == "" {
		t.Fatalf("expected today content, got empty")
	}

	// Get recent memories (1 day)
	rec, err := s.GetRecentMemories(1)
	if err != nil {
		t.Fatalf("GetRecentMemories error: %v", err)
	}
	if rec == "" {
		t.Fatalf("expected recent memory content, got empty")
	}

	// Memory context
	mc, err := s.GetMemoryContext()
	if err != nil {
		t.Fatalf("GetMemoryContext error: %v", err)
	}
	if mc == "" {
		t.Fatalf("expected memory context, got empty")
	}
}

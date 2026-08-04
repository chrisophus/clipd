package history

import (
	"os"
	"path/filepath"
	"testing"
)

func tmpStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "history.json"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestAddDedup(t *testing.T) {
	s := tmpStore(t)
	s.Add("hello")
	s.Add("world")
	s.Add("hello") // duplicate — should move to top, not duplicate

	entries := s.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "hello" {
		t.Errorf("expected 'hello' at top, got %q", entries[0].Text)
	}
}

func TestClear(t *testing.T) {
	s := tmpStore(t)
	s.Add("a")
	s.Add("b")
	s.Clear()
	if len(s.All()) != 0 {
		t.Error("expected empty after Clear")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s1, _ := Open(path)
	s1.Add("persistent")

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	entries := s2.All()
	if len(entries) != 1 || entries[0].Text != "persistent" {
		t.Errorf("unexpected entries after reload: %v", entries)
	}
}

func TestRemove(t *testing.T) {
	s := tmpStore(t)
	s.Add("a")
	s.Add("b")
	s.Add("c") // entries: c, b, a

	s.Remove(1) // remove b
	entries := s.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "c" || entries[1].Text != "a" {
		t.Errorf("unexpected order: %v", entries)
	}
}

func TestEmpty(t *testing.T) {
	s := tmpStore(t)
	if added, _ := s.Add(""); added {
		t.Error("expected empty string to be rejected")
	}
}

func TestAtomicSave(t *testing.T) {
	s := tmpStore(t)
	s.Add("x")
	// Temp file must not linger.
	if _, err := os.Stat(s.path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after save")
	}
}

package history

import (
	"fmt"
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

func TestHistoryLimit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []Option
		limit   int
	}{
		{name: "default", limit: 500},
		{name: "custom", options: []Option{WithMaxEntries(2)}, limit: 2},
		{name: "single", options: []Option{WithMaxEntries(1)}, limit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.json")
			s, err := Open(path, tc.options...)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < tc.limit+2; i++ {
				if added, err := s.Add(fmt.Sprint(i)); err != nil || !added {
					t.Fatalf("Add(%d): added=%v, err=%v", i, added, err)
				}
			}
			check := func(entries []Entry) {
				t.Helper()
				if len(entries) != tc.limit {
					t.Fatalf("got %d entries, want %d", len(entries), tc.limit)
				}
				for i, entry := range entries {
					if want := fmt.Sprint(tc.limit + 1 - i); entry.Text != want {
						t.Errorf("entry %d = %q, want %q", i, entry.Text, want)
					}
				}
			}
			check(s.All())
			reloaded, err := Open(path, tc.options...)
			if err != nil {
				t.Fatal(err)
			}
			check(reloaded.All())
		})
	}
}

func TestInvalidHistoryLimit(t *testing.T) {
	for _, limit := range []int{0, -1} {
		if _, err := Open(filepath.Join(t.TempDir(), "history.json"), WithMaxEntries(limit)); err == nil {
			t.Errorf("Open with limit %d: expected error", limit)
		}
	}
}

func TestLowerHistoryLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"a", "b", "c"} {
		if _, err := s.Add(text); err != nil {
			t.Fatal(err)
		}
	}
	s, err = Open(path, WithMaxEntries(2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("a"); err != nil {
		t.Fatal(err)
	}
	entries := s.All()
	if len(entries) != 2 || entries[0].Text != "a" || entries[1].Text != "c" {
		t.Fatalf("unexpected entries after lowering limit: %v", entries)
	}
}

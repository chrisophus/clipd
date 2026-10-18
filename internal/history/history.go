// Package history manages clipboard history persistence.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultMaxEntries is the history limit used when no option is supplied.
const DefaultMaxEntries = 500

// Option configures a history store.
type Option func(*Store)

// WithMaxEntries sets the maximum number of entries retained on Add.
// The limit must be positive.
func WithMaxEntries(limit int) Option {
	return func(s *Store) { s.maxEntries = limit }
}

// Entry is a single clipboard snapshot.
type Entry struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Store is a thread-safe clipboard history backed by a JSON file.
type Store struct {
	mu         sync.Mutex
	path       string
	entries    []Entry
	maxEntries int
}

// Open loads (or creates) the history file at path.
func Open(path string, options ...Option) (*Store, error) {
	s := &Store{path: path, maxEntries: DefaultMaxEntries}
	for _, option := range options {
		option(s)
	}
	if s.maxEntries <= 0 {
		return nil, fmt.Errorf("history limit must be positive")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_ = s.load() // ignore missing-file errors on first run
	return s, nil
}

// All returns a copy of all entries, newest first.
func (s *Store) All() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Add appends text if it differs from the most-recent entry.
// Returns true when the entry was actually added.
func (s *Store) Add(text string) (bool, error) {
	if text == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Deduplicate: skip if identical to the current top entry.
	if len(s.entries) > 0 && s.entries[0].Text == text {
		return false, nil
	}

	// Remove any older duplicate occurrence anywhere in the list.
	filtered := s.entries[:0]
	for _, e := range s.entries {
		if e.Text != text {
			filtered = append(filtered, e)
		}
	}

	// Prepend (newest first) and cap.
	s.entries = append([]Entry{{Text: text, At: time.Now()}}, filtered...)
	if len(s.entries) > s.maxEntries {
		s.entries = s.entries[:s.maxEntries]
	}

	return true, s.save()
}

// Remove deletes the entry at index i (0-based, newest first).
func (s *Store) Remove(i int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.entries) {
		return nil
	}
	s.entries = append(s.entries[:i], s.entries[i+1:]...)
	return s.save()
}

// Clear wipes all history.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	return s.save()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.entries)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically via temp file.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

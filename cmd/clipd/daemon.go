// Package main is the clipd background daemon.
// It polls the system clipboard and appends new entries to the history store.
// It needs a clipboard to watch, so it only runs where a display server is
// available (macOS, or Linux under X11/Wayland) — on headless machines use
// "clip add" instead.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.design/x/clipboard"

	"github.com/chrisophus/clipd/internal/history"
	"github.com/chrisophus/clipd/internal/version"
)

func defaultHistoryPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.local/share/clipd/history.json"
}

func main() {
	maxEntries := flag.Int("max-entries", history.DefaultMaxEntries, "maximum number of history entries (must be positive)")
	pollInterval := flag.Duration("poll-interval", 500*time.Millisecond, "clipboard poll interval (must be positive)")
	flag.Parse()
	if *maxEntries <= 0 || *pollInterval <= 0 {
		log.Fatal("clipd: max-entries and poll-interval must be positive")
	}

	path := os.Getenv("CLIPD_HISTORY")
	if path == "" {
		path = defaultHistoryPath()
	}

	store, err := history.Open(path, history.WithMaxEntries(*maxEntries))
	if err != nil {
		log.Fatalf("clipd: open history: %v", err)
	}

	if err := clipboard.Init(); err != nil {
		log.Fatalf("clipd: init clipboard: %v", err)
	}

	fmt.Fprintf(os.Stderr, "clipd %s: watching clipboard, history at %s\n", version.Version, path)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			fmt.Fprintln(os.Stderr, "clipd: shutting down")
			return
		case <-ticker.C:
			text := string(clipboard.Read(clipboard.FmtText))
			if text == "" {
				continue
			}
			added, err := store.Add(text)
			if err != nil {
				log.Printf("clipd: save error: %v", err)
			}
			if added {
				preview := text
				if len(preview) > 60 {
					preview = preview[:60] + "…"
				}
				log.Printf("clipd: saved %q", preview)
			}
		}
	}
}

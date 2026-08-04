// Package main is the clipd background daemon.
// It polls the macOS clipboard and appends new entries to the history store.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.design/x/clipboard"

	"github.com/chrisophus/clipd/internal/history"
)

func defaultHistoryPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.local/share/clipd/history.json"
}

func main() {
	path := os.Getenv("CLIPD_HISTORY")
	if path == "" {
		path = defaultHistoryPath()
	}

	store, err := history.Open(path)
	if err != nil {
		log.Fatalf("clipd: open history: %v", err)
	}

	if err := clipboard.Init(); err != nil {
		log.Fatalf("clipd: init clipboard: %v", err)
	}

	fmt.Fprintf(os.Stderr, "clipd: watching clipboard, history at %s\n", path)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(500 * time.Millisecond)
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

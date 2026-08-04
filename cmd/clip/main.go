// Package main is the clip CLI — list, search, copy, and manage clipboard history.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.design/x/clipboard"

	"github.com/chrisophus/clipd/internal/history"
	"github.com/spf13/cobra"
)

func historyPath() string {
	if p := os.Getenv("CLIPD_HISTORY"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return home + "/.local/share/clipd/history.json"
}

func openStore() *history.Store {
	s, err := history.Open(historyPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "clip: open history: %v\n", err)
		os.Exit(1)
	}
	return s
}

func main() {
	root := &cobra.Command{
		Use:   "clip",
		Short: "Manage recent clipboard items",
	}

	// ── list ──────────────────────────────────────────────────────────────────
	var listN int
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recent clipboard entries",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			entries := openStore().All()
			if len(entries) == 0 {
				fmt.Println("(no clipboard history)")
				return nil
			}
			limit := listN
			if limit <= 0 || limit > len(entries) {
				limit = len(entries)
			}
			for i, e := range entries[:limit] {
				fmt.Printf("%3d  %s  %s\n", i, e.At.Format("01-02 15:04"), preview(e.Text, 72))
			}
			return nil
		},
	}
	listCmd.Flags().IntVarP(&listN, "count", "n", 20, "number of entries to show")

	// ── search ────────────────────────────────────────────────────────────────
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search clipboard history (case-insensitive substring)",
		Aliases: []string{"find", "grep"},
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.ToLower(strings.Join(args, " "))
			entries := openStore().All()
			found := 0
			for i, e := range entries {
				if strings.Contains(strings.ToLower(e.Text), q) {
					fmt.Printf("%3d  %s  %s\n", i, e.At.Format("01-02 15:04"), preview(e.Text, 72))
					found++
				}
			}
			if found == 0 {
				fmt.Println("(no matches)")
			}
			return nil
		},
	}

	// ── copy ──────────────────────────────────────────────────────────────────
	copyCmd := &cobra.Command{
		Use:   "copy <index>",
		Short: "Copy entry <index> back to the clipboard",
		Aliases: []string{"cp", "use"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("index must be a number")
			}
			entries := openStore().All()
			if idx < 0 || idx >= len(entries) {
				return fmt.Errorf("index %d out of range (0–%d)", idx, len(entries)-1)
			}
			if err := clipboard.Init(); err != nil {
				return fmt.Errorf("init clipboard: %w", err)
			}
			clipboard.Write(clipboard.FmtText, []byte(entries[idx].Text))
			fmt.Printf("Copied entry %d to clipboard.\n", idx)
			return nil
		},
	}

	// ── show ──────────────────────────────────────────────────────────────────
	showCmd := &cobra.Command{
		Use:   "show <index>",
		Short: "Print the full text of entry <index>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("index must be a number")
			}
			entries := openStore().All()
			if idx < 0 || idx >= len(entries) {
				return fmt.Errorf("index %d out of range (0–%d)", idx, len(entries)-1)
			}
			fmt.Println(entries[idx].Text)
			return nil
		},
	}

	// ── delete ────────────────────────────────────────────────────────────────
	deleteCmd := &cobra.Command{
		Use:   "delete <index>",
		Short: "Delete entry <index> from history",
		Aliases: []string{"rm", "del"},
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("index must be a number")
			}
			store := openStore()
			entries := store.All()
			if idx < 0 || idx >= len(entries) {
				return fmt.Errorf("index %d out of range (0–%d)", idx, len(entries)-1)
			}
			if err := store.Remove(idx); err != nil {
				return err
			}
			fmt.Printf("Deleted entry %d.\n", idx)
			return nil
		},
	}

	// ── clear ─────────────────────────────────────────────────────────────────
	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Wipe all clipboard history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := openStore().Clear(); err != nil {
				return err
			}
			fmt.Println("Clipboard history cleared.")
			return nil
		},
	}

	root.AddCommand(listCmd, searchCmd, copyCmd, showCmd, deleteCmd, clearCmd)

	// Default: show list when called with no subcommand.
	root.RunE = listCmd.RunE

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// preview returns a single-line preview of text, truncated to maxRunes.
func preview(text string, maxRunes int) string {
	// Collapse newlines.
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Trim leading/trailing whitespace.
	text = strings.TrimSpace(text)

	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxRunes]) + "…"
}

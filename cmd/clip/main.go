// Package main is the clip CLI — list, search, copy, and manage clipboard history.
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.design/x/clipboard"

	"github.com/chrisophus/clipd/internal/history"
	"github.com/chrisophus/clipd/internal/version"
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

// useOSC52 reports whether to bypass the system clipboard and write through
// the terminal instead: a Unix box with no display server, i.e. a headless
// server reached over SSH.
func useOSC52() bool {
	switch runtime.GOOS {
	case "darwin", "windows":
		return false
	}
	return os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

// writeOSC52 puts text on the clipboard of the terminal at the far end of the
// tty via the OSC 52 escape sequence. Over SSH that is the clipboard of the
// machine you are sitting at, which is exactly where a staged entry needs to
// be for the next paste.
func writeOSC52(text string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("no display server and no tty for OSC 52: %w", err)
	}
	defer tty.Close()
	_, err = fmt.Fprintf(tty, "\x1b]52;c;%s\x07",
		base64.StdEncoding.EncodeToString([]byte(text)))
	return err
}

// startStageWorker re-execs clip as a detached background process that loads
// entry idx into the clipboard after delay. Detaching matters twice over: it
// lets the caller return before the delay elapses, and on X11/Wayland the
// worker stays resident afterwards as the selection owner (selections die
// with the process that set them).
func startStageWorker(idx int, delay time.Duration) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate executable: %w", err)
	}
	worker := exec.Command(self, "_stage-worker",
		strconv.Itoa(idx),
		delay.String(),
	)
	worker.Stdin = nil
	worker.Stdout = nil
	worker.Stderr = nil
	if err := worker.Start(); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	// Detach — do not wait.
	go worker.Wait() //nolint:errcheck
	return nil
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds the clip command tree. Commands write to cmd.OutOrStdout
// and read from cmd.InOrStdin so tests can capture and feed them.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "clip",
		Short:   "Manage recent clipboard items",
		Version: version.Version,
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "clip %s (commit %s, built %s)\n",
				version.Version, version.Commit, version.Date)
		},
	}

	// ── list ──────────────────────────────────────────────────────────────────
	var listN int
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List recent clipboard entries",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			entries := openStore().All()
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no clipboard history)")
				return nil
			}
			limit := listN
			if limit <= 0 || limit > len(entries) {
				limit = len(entries)
			}
			for i, e := range entries[:limit] {
				fmt.Fprintf(cmd.OutOrStdout(), "%3d  %s  %s\n", i, e.At.Format("01-02 15:04"), preview(e.Text, 72))
			}
			return nil
		},
	}
	listCmd.Flags().IntVarP(&listN, "count", "n", 20, "number of entries to show")

	// ── search ────────────────────────────────────────────────────────────────
	searchCmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search clipboard history (case-insensitive substring)",
		Aliases: []string{"find", "grep"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.ToLower(strings.Join(args, " "))
			entries := openStore().All()
			found := 0
			for i, e := range entries {
				if strings.Contains(strings.ToLower(e.Text), q) {
					fmt.Fprintf(cmd.OutOrStdout(), "%3d  %s  %s\n", i, e.At.Format("01-02 15:04"), preview(e.Text, 72))
					found++
				}
			}
			if found == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no matches)")
			}
			return nil
		},
	}

	// ── copy ──────────────────────────────────────────────────────────────────
	copyCmd := &cobra.Command{
		Use:     "copy <index>",
		Short:   "Copy entry <index> back to the clipboard",
		Aliases: []string{"cp", "use"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("index must be a number")
			}
			entries := openStore().All()
			if idx < 0 || idx >= len(entries) {
				return fmt.Errorf("index %d out of range (0–%d)", idx, len(entries)-1)
			}
			switch {
			case useOSC52():
				// Headless: set the terminal's clipboard through the tty.
				if err := writeOSC52(entries[idx].Text); err != nil {
					return err
				}
			case runtime.GOOS == "darwin" || runtime.GOOS == "windows":
				// The OS clipboard service copies the data; safe to write and exit.
				if err := clipboard.Init(); err != nil {
					return fmt.Errorf("init clipboard: %w", err)
				}
				clipboard.Write(clipboard.FmtText, []byte(entries[idx].Text))
			default:
				// X11/Wayland: the selection dies with its owner, so hand off to
				// a detached worker that writes and stays resident (delay 0).
				if err := startStageWorker(idx, 0); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Copied entry %d to clipboard.\n", idx)
			return nil
		},
	}

	// ── paste ─────────────────────────────────────────────────────────────────
	pasteCmd := &cobra.Command{
		Use:   "paste <index>",
		Short: "Print the full text of entry <index> to stdout",
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
			fmt.Fprintln(cmd.OutOrStdout(), entries[idx].Text)
			return nil
		},
	}

	// ── delete ────────────────────────────────────────────────────────────────
	deleteCmd := &cobra.Command{
		Use:     "delete <index>",
		Short:   "Delete entry <index> from history",
		Aliases: []string{"rm", "del"},
		Args:    cobra.ExactArgs(1),
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
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted entry %d.\n", idx)
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
			fmt.Fprintln(cmd.OutOrStdout(), "Clipboard history cleared.")
			return nil
		},
	}

	// ── add ───────────────────────────────────────────────────────────────────
	addCmd := &cobra.Command{
		Use:   "add [text...]",
		Short: "Add text (or stdin) to history without touching the clipboard",
		Long: `Add stores text directly in the history file. This is how entries get in on
a headless machine, where there is no clipboard for the daemon to watch.

Example:
  clip add 'ssh prod-web-1'
  printf %s "$secret" | clip add`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var text string
			if len(args) > 0 {
				text = strings.Join(args, " ")
			} else {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				text = strings.TrimSuffix(string(data), "\n")
			}
			if text == "" {
				return fmt.Errorf("nothing to add")
			}
			added, err := openStore().Add(text)
			if err != nil {
				return err
			}
			if !added {
				fmt.Fprintln(cmd.OutOrStdout(), "Already at top of history.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Added entry 0.")
			return nil
		},
	}

	// ── stage ─────────────────────────────────────────────────────────────────
	var stageAfter time.Duration
	stageCmd := &cobra.Command{
		Use:   "stage <index>",
		Short: "Load entry <index> into the clipboard after a delay (default 1s)",
		Long: `Stage detaches immediately and loads the entry into the clipboard after the
delay, so you can paste whatever is currently on the clipboard first.

On a headless machine (no display server) the entry is delivered via the
OSC 52 terminal escape instead, which sets the clipboard of the terminal you
are connected from — through SSH.

Example:
  clip stage 1        # load slot 1 after 1s (default)
  clip stage 1 -w 2s  # load slot 1 after 2s`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("index must be a number")
			}
			entries := openStore().All()
			if idx < 0 || idx >= len(entries) {
				return fmt.Errorf("index %d out of range (0–%d)", idx, len(entries)-1)
			}
			return startStageWorker(idx, stageAfter)
		},
	}
	stageCmd.Flags().DurationVarP(&stageAfter, "wait", "w", time.Second, "delay before loading into clipboard")

	// ── _stage-worker (internal, not shown in help) ────────────────────────────
	workerCmd := &cobra.Command{
		Use:    "_stage-worker <index> <duration>",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, _ := strconv.Atoi(args[0])
			delay, err := time.ParseDuration(args[1])
			if err != nil {
				return err
			}
			entries := openStore().All()
			if idx < 0 || idx >= len(entries) {
				return fmt.Errorf("index out of range")
			}
			text := entries[idx].Text
			time.Sleep(delay)
			if useOSC52() {
				// Headless: write through the controlling terminal, which the
				// detached worker still shares with the shell that ran stage.
				return writeOSC52(text)
			}
			if err := clipboard.Init(); err != nil {
				return err
			}
			changed := clipboard.Write(clipboard.FmtText, []byte(text))
			switch runtime.GOOS {
			case "darwin", "windows":
				// The OS clipboard service holds the data; exit immediately.
			default:
				// X11/Wayland selections live only as long as their owner.
				// Stay resident until another application takes the clipboard.
				<-changed
			}
			return nil
		},
	}

	root.AddCommand(listCmd, searchCmd, copyCmd, pasteCmd, addCmd, deleteCmd, clearCmd, stageCmd, workerCmd, versionCmd)

	// Default: show list when called with no subcommand.
	root.RunE = listCmd.RunE

	return root
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

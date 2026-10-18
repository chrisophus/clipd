package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/chrisophus/clipd/internal/history"
)

// row matches one list/search output line for entry idx with the given text.
func row(idx, text string) string {
	return `\s*` + idx + `  \d\d-\d\d \d\d:\d\d  ` + regexp.QuoteMeta(text) + `\n`
}

func TestCommands(t *testing.T) {
	seed := []string{"gamma", "Beta line", "alpha"} // newest first

	tests := []struct {
		name    string
		seed    []string
		args    []string
		stdin   string
		wantOut string // regexp matched against the whole stdout
		wantErr bool
		// wantHistory is the expected history afterwards, newest first.
		// nil means the history must be unchanged from seed.
		wantHistory []string
	}{
		// list
		{name: "list", seed: seed, args: []string{"list"},
			wantOut: "^" + row("0", "gamma") + row("1", "Beta line") + row("2", "alpha") + "$"},
		{name: "list alias ls", seed: seed, args: []string{"ls"},
			wantOut: "^" + row("0", "gamma") + row("1", "Beta line") + row("2", "alpha") + "$"},
		{name: "no subcommand lists", seed: seed, args: []string{},
			wantOut: "^" + row("0", "gamma") + row("1", "Beta line") + row("2", "alpha") + "$"},
		{name: "list count", seed: seed, args: []string{"list", "-n", "1"},
			wantOut: "^" + row("0", "gamma") + "$"},
		{name: "list count over length", seed: seed, args: []string{"ls", "--count", "99"},
			wantOut: "^" + row("0", "gamma") + row("1", "Beta line") + row("2", "alpha") + "$"},
		{name: "list empty", args: []string{"list"},
			wantOut: `^\(no clipboard history\)\n$`},
		{name: "list truncates long entries", seed: []string{strings.Repeat("x", 80)}, args: []string{"list"},
			wantOut: "^" + row("0", strings.Repeat("x", 72)+"…") + "$"},

		// search
		{name: "search case-insensitive", seed: seed, args: []string{"search", "BETA"},
			wantOut: "^" + row("1", "Beta line") + "$"},
		{name: "search alias find", seed: seed, args: []string{"find", "a"},
			wantOut: "^" + row("0", "gamma") + row("1", "Beta line") + row("2", "alpha") + "$"},
		{name: "search alias grep", seed: seed, args: []string{"grep", "alp"},
			wantOut: "^" + row("2", "alpha") + "$"},
		{name: "search joins args", seed: seed, args: []string{"search", "beta", "line"},
			wantOut: "^" + row("1", "Beta line") + "$"},
		{name: "search no matches", seed: seed, args: []string{"search", "zzz"},
			wantOut: `^\(no matches\)\n$`},
		{name: "search requires query", seed: seed, args: []string{"search"},
			wantErr: true},

		// paste
		{name: "paste", seed: []string{"multi\nline"}, args: []string{"paste", "0"},
			wantOut: "^multi\nline\n$"},
		{name: "paste middle", seed: seed, args: []string{"paste", "1"},
			wantOut: "^Beta line\n$"},
		{name: "paste out of range", seed: seed, args: []string{"paste", "3"},
			wantErr: true},
		{name: "paste negative", seed: seed, args: []string{"paste", "-1"},
			wantErr: true},
		{name: "paste non-number", seed: seed, args: []string{"paste", "x"},
			wantErr: true},
		{name: "paste requires index", seed: seed, args: []string{"paste"},
			wantErr: true},

		// add
		{name: "add args", seed: seed, args: []string{"add", "ssh", "prod"},
			wantOut:     "^Added entry 0.\n$",
			wantHistory: []string{"ssh prod", "gamma", "Beta line", "alpha"}},
		{name: "add stdin trims trailing newline", args: []string{"add"}, stdin: "from stdin\n",
			wantOut:     "^Added entry 0.\n$",
			wantHistory: []string{"from stdin"}},
		{name: "add duplicate of top", seed: seed, args: []string{"add", "gamma"},
			wantOut: "^Already at top of history.\n$"},
		{name: "add moves older duplicate to top", seed: seed, args: []string{"add", "alpha"},
			wantOut:     "^Added entry 0.\n$",
			wantHistory: []string{"alpha", "gamma", "Beta line"}},
		{name: "add empty stdin", seed: seed, args: []string{"add"}, stdin: "",
			wantErr: true},

		// delete
		{name: "delete", seed: seed, args: []string{"delete", "1"},
			wantOut:     "^Deleted entry 1.\n$",
			wantHistory: []string{"gamma", "alpha"}},
		{name: "delete alias rm", seed: seed, args: []string{"rm", "0"},
			wantOut:     "^Deleted entry 0.\n$",
			wantHistory: []string{"Beta line", "alpha"}},
		{name: "delete alias del", seed: seed, args: []string{"del", "2"},
			wantOut:     "^Deleted entry 2.\n$",
			wantHistory: []string{"gamma", "Beta line"}},
		{name: "delete out of range", seed: seed, args: []string{"delete", "5"},
			wantErr: true},
		{name: "delete non-number", seed: seed, args: []string{"rm", "one"},
			wantErr: true},

		// clear
		{name: "clear", seed: seed, args: []string{"clear"},
			wantOut:     "^Clipboard history cleared.\n$",
			wantHistory: []string{}},
		{name: "clear empty", args: []string{"clear"},
			wantOut:     "^Clipboard history cleared.\n$",
			wantHistory: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.json")
			t.Setenv("CLIPD_HISTORY", path)

			s, err := history.Open(path)
			if err != nil {
				t.Fatalf("open history: %v", err)
			}
			for _, text := range slices.Backward(tt.seed) {
				if _, err := s.Add(text); err != nil {
					t.Fatalf("seed: %v", err)
				}
			}

			var stdout, stderr bytes.Buffer
			root := newRootCmd()
			root.SetArgs(tt.args)
			root.SetIn(strings.NewReader(tt.stdin))
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			err = root.Execute()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v (stderr: %s)", err, tt.wantErr, stderr.String())
			}
			if !tt.wantErr && !regexp.MustCompile(tt.wantOut).MatchString(stdout.String()) {
				t.Errorf("stdout = %q, want match for %q", stdout.String(), tt.wantOut)
			}

			want := tt.wantHistory
			if want == nil {
				want = tt.seed
			}
			reopened, err := history.Open(path)
			if err != nil {
				t.Fatalf("reopen history: %v", err)
			}
			var got []string
			for _, e := range reopened.All() {
				got = append(got, e.Text)
			}
			if !slices.Equal(got, want) {
				t.Errorf("history = %q, want %q", got, want)
			}
		})
	}
}

# clipd

A lightweight clipboard manager for macOS and Linux (desktop or headless-over-SSH): a background daemon (`clipd`) that captures clipboard history, and a CLI (`clip`) to list, search, and restore past clips.

## Install

Download pre-built binaries for macOS or Linux from the
[releases page](https://github.com/chrisophus/clipd/releases), or install
from source:

```bash
go install github.com/chrisophus/clipd/cmd/clipd@latest
go install github.com/chrisophus/clipd/cmd/clip@latest
```

### Build locally

```bash
make build        # compiles clip and clipd into bin/
make install      # installs into $(GOPATH)/bin
make test         # runs tests
make clean        # removes bin/
```

To cut a new release (pushes a tag, which triggers the GitHub Actions workflow):

```bash
make release VERSION=1.2.3
```

## Usage

### Start the daemon

```bash
clipd &
```

The daemon polls the clipboard every 500 ms and appends new text to
`~/.local/share/clipd/history.json` (up to 500 entries, deduplicated).

Override these defaults with daemon flags:

```bash
clipd -max-entries 1000 -poll-interval 250ms &
```

`-max-entries` defaults to `500`; `-poll-interval` defaults to `500ms` and
accepts Go durations such as `250ms` or `1s`. Both values must be positive.
The history limit is applied when a new entry is added; older entries are
removed first. Run `clipd -h` to see the flags.

To auto-start at login, add `clipd &` to `~/.zprofile`, or create a
`launchd` plist (see below).

### CLI commands

```
clip                          # same as "clip list"
clip list [-n 20]             # show most recent N entries (index, time, preview)
clip search <query>           # case-insensitive substring search
clip paste  <index>           # print full text of entry to stdout
clip add    [text]            # add text (or stdin) to history directly
clip copy   <index>           # copy entry back to the clipboard
clip stage  <index> [-w 1s]   # load entry into the clipboard after a delay
clip delete <index>           # remove entry from history
clip clear                    # wipe all history
```

Aliases: `ls` for `list`; `find`/`grep` for `search`; `cp`/`use` for `copy`;
`rm`/`del` for `delete`.

### Example session

```
$ clip list
  0  08-04 14:32  https://github.com/chrisophus/clipd
  1  08-04 14:30  func (s *Store) Add(text string) (bool, error) {
  2  08-04 14:11  go install github.com/chrisophus/clipd/cmd/clip@latest

$ clip search github
  0  08-04 14:32  https://github.com/chrisophus/clipd

$ clip copy 1          # puts the func signature back on the clipboard
Copied entry 1 to clipboard.

$ clip paste 2
go install github.com/chrisophus/clipd/cmd/clip@latest
```

### Staging the next clip

`clip copy` overwrites the clipboard right now, which is the wrong moment when
you still need to paste what is already there. `clip stage <index>` returns
immediately and writes the entry to the clipboard a moment later (1 s by
default), so the *next* thing you need is waiting by the time you are asked
for it.

A common case: slot 0 holds a long command, slot 1 holds the password that
command is about to prompt for.

```
$ clip list
  0  08-04 14:32  ansible scs001246955-4 -i inventory.yml -m shell -a 'go clean -cache && docke…
  1  08-04 14:31  hunter2
```

Type `clip stage 1 && `, paste slot 0 (still on the clipboard) to complete the
line, and hit enter:

```bash
clip stage 1 && ansible scs001246955-4 -i inventory.yml -m shell \
  -a 'go clean -cache && docker builder prune -af' --become --ask-pass
```

`clip stage` detaches before `ansible` starts, so by the time the prompt
appears the clipboard holds slot 1 and you can paste it straight in:

```
SSH password:      # ⌘V — the staged password
```

Use `-w` when the prompt takes longer to arrive:

```bash
clip stage 1 -w 5s
```

Note that the staged text sits on the clipboard afterwards like any other
clip — follow up with `clip stage 0` (or another `copy`) if you would rather
not leave a password there.

This workflow also works when `clip` runs on a **headless server over SSH** —
the staged entry is delivered to the clipboard of the machine you're sitting
at, via the terminal. See [Headless servers](#headless-servers-ssh) below.

### Configuration

| Env var        | Default                                    | Purpose           |
|----------------|--------------------------------------------|-------------------|
| `CLIPD_HISTORY`| `~/.local/share/clipd/history.json`        | History file path |

### launchd auto-start (macOS)

Create `~/Library/LaunchAgents/com.ccason.clipd.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>             <string>com.ccason.clipd</string>
  <key>ProgramArguments</key>  <array><string>/Users/YOU/bin/clipd</string></array>
  <key>RunAtLoad</key>         <true/>
  <key>KeepAlive</key>         <true/>
  <key>StandardErrorPath</key> <string>/tmp/clipd.log</string>
</dict>
</plist>
```

Then load it:

```bash
launchctl load ~/Library/LaunchAgents/com.ccason.clipd.plist
```

## Linux

Both binaries are pure Go on Linux — no cgo, no C toolchain needed. Behavior
depends on whether a display server is available.

### Headless servers (SSH)

With no display server there is no system clipboard, so `clip copy` and
`clip stage` deliver the entry through the terminal instead, using the
[OSC 52](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands)
escape sequence. That sets the clipboard of the machine your terminal is
running on — through SSH — which is exactly what you want: stage a password
on the server, paste it locally at the prompt.

There is no clipboard for `clipd` to watch, so don't run the daemon; put
entries into history explicitly with `clip add`:

```bash
clip add "ansible scs001246955-4 -i inventory.yml -m shell \
  -a 'go clean -cache && docker builder prune -af' --become --ask-pass"
read -rs pw && printf %s "$pw" | clip add && unset pw   # stays out of shell history
```

From there the [staging workflow](#staging-the-next-clip) is identical to
macOS.

Requirements:

- A terminal emulator that supports OSC 52 writes: iTerm2, kitty, Alacritty,
  WezTerm, foot, Ghostty, Windows Terminal, and recent xterm all do.
  (The stock GNOME Terminal does not.)
- **iTerm2**: enable *Settings → General → Selection → "Applications in
  terminal may access clipboard"* — it is off by default, and without it
  iTerm2 silently ignores OSC 52.
- **tmux**: add `set -g set-clipboard on` to `~/.tmux.conf` on the server.
  The default (`external`) forwards only tmux's own copy-mode yanks and
  drops OSC 52 coming from applications inside panes. Note that with `on`,
  tmux also keeps a copy of the text in its paste-buffer list
  (`tmux delete-buffer` discards it — worth doing after staging a password).
  If it still doesn't forward, your server's terminfo may lack the `Ms`
  capability for your `$TERM`; add
  `set -as terminal-features ',xterm-256color:clipboard'` as well.

### Desktop (X11 / Wayland)

Everything works as on macOS, with one difference under the hood: X11 and
Wayland clipboard selections only live as long as the process that set them,
so `clip copy` and `clip stage` leave a small background `clip` worker
resident as the selection owner. It exits as soon as anything else takes the
clipboard.

To auto-start the daemon, create `~/.config/systemd/user/clipd.service`:

```ini
[Unit]
Description=clipd clipboard history daemon
After=graphical-session.target

[Service]
ExecStart=%h/bin/clipd
Restart=on-failure

[Install]
WantedBy=default.target
```

Then enable it:

```bash
systemctl --user enable --now clipd
```

## File layout

```
cmd/
  clip/   main.go          # CLI
  clipd/  daemon.go        # background daemon
internal/
  history/
    history.go             # Store: thread-safe, atomic saves, dedup
    history_test.go
  version/
    version.go             # build-time version info (injected via ldflags)
.github/
  workflows/
    release.yml            # publishes binaries to GitHub Releases on v* tag
.goreleaser.yaml           # cross-compilation config (linux/darwin, amd64/arm64)
Makefile                   # build / test / install / release targets
```

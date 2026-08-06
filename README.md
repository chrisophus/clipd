# clipd

A lightweight macOS clipboard manager: a background daemon (`clipd`) that captures clipboard history, and a CLI (`clip`) to list, search, and restore past clips.

## Install

```bash
go install github.com/chrisophus/clipd/cmd/clipd@latest
go install github.com/chrisophus/clipd/cmd/clip@latest
```

Or build locally:

```bash
go build -o bin/clipd ./cmd/clipd
go build -o bin/clip  ./cmd/clip
```

## Usage

### Start the daemon

```bash
clipd &
```

The daemon polls the clipboard every 500 ms and appends new text to
`~/.local/share/clipd/history.json` (up to 500 entries, deduplicated).

To auto-start at login, add `clipd &` to `~/.zprofile`, or create a
`launchd` plist (see below).

### CLI commands

```
clip                          # same as "clip list"
clip list [-n 20]             # show most recent N entries (index, time, preview)
clip search <query>           # case-insensitive substring search
clip paste  <index>           # print full text of entry to stdout
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

## File layout

```
cmd/
  clip/   main.go          # CLI
  clipd/  daemon.go        # background daemon
internal/
  history/
    history.go             # Store: thread-safe, atomic saves, dedup
    history_test.go
```

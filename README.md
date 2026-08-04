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
clip show   <index>           # print full text of entry
clip copy   <index>           # copy entry back to the clipboard
clip delete <index>           # remove entry from history
clip clear                    # wipe all history
```

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

$ clip show 2
go install github.com/chrisophus/clipd/cmd/clip@latest
```

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

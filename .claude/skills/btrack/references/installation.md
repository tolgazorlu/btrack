# Installing btrack

The skill drives the `btrack` binary, which must be installed and on `PATH`.

## Install the binary

### macOS and Linux (Homebrew)

```bash
brew install tolgazorlu/btrack/btrack
btrack --version
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex
btrack --version
```

### Go (any platform)

```bash
go install github.com/tolgazorlu/btrack@latest
btrack --version
```

This puts `btrack` in `$GOPATH/bin` (typically `~/go/bin`). Make sure that's on your `PATH`.

### Pre-built binaries

Pick the right archive for your OS/arch from the [Releases page](https://github.com/tolgazorlu/btrack/releases/latest), unpack, and put `btrack` somewhere on `PATH` (e.g. `/usr/local/bin`).

### Build from source

```bash
git clone https://github.com/tolgazorlu/btrack.git
cd btrack
go build -o btrack .
sudo install -m 755 btrack /usr/local/bin/btrack
```

## Install the skill

```bash
btrack skill install          # writes ~/.claude/skills/btrack/
```

Then reopen Claude Code so it picks the skill up. The skill calls `btrack`
through ordinary shell commands, so there is nothing else to register.

## PATH troubleshooting

If Claude reports `btrack: command not found`, the client is launching with a
different `PATH` than your shell.

**Check where it lives:**

```bash
which btrack                  # e.g. /opt/homebrew/bin/btrack
```

**macOS Homebrew on Apple Silicon.** Brew installs to `/opt/homebrew/bin`, which
GUI apps may not see. Symlink it into a directory they do see:

```bash
sudo ln -s /opt/homebrew/bin/btrack /usr/local/bin/btrack
```

**Go installs.** `~/go/bin` is often missing from the GUI `PATH`. Either symlink
as above, or add the directory to your shell profile and relaunch the client
from a terminal.

## Verifying the install

```bash
btrack --version              # binary works
btrack skill install          # writes ~/.claude/skills/btrack/
ls ~/.claude/skills/btrack/   # SKILL.md, README.md, references/
btrack s "test session"       # start
btrack w                      # live status
btrack x -m "works"           # stop
```

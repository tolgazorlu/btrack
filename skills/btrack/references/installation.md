# Installing btrack

The skill drives the `btrack` binary, which must be installed and on `PATH`. This page covers every install path plus PATH-resolution gotchas that show up when MCP launches the binary as a subprocess.

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

## Register the MCP server

### Claude Code

```bash
claude mcp add btrack -- btrack mcp
claude mcp list | grep btrack         # should show "✓ Connected"
```

### Cursor / Claude Desktop / Continue

Add to your client's MCP config (`~/.cursor/mcp.json`, `claude_desktop_config.json`, etc.):

```json
{
  "mcpServers": {
    "btrack": {
      "command": "btrack",
      "args": ["mcp"]
    }
  }
}
```

### Gemini CLI

In `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "btrack": {
      "command": "btrack",
      "args": ["mcp"]
    }
  }
}
```

### HTTP transport (if stdio fails)

If your client's stdio launch can't find `btrack` (sandboxing, PATH, etc.), run the HTTP server yourself and register the URL:

```bash
btrack mcp --http              # 127.0.0.1:8765, path /mcp
```

Then for Claude Code:

```bash
claude mcp add --transport http btrack http://127.0.0.1:8765/mcp
```

## After registering

**Fully quit and reopen the client.** MCP servers load at startup — they will not appear in a session that was already open when you ran `claude mcp add`.

In a fresh session, ask "what am I tracking right now?" — Claude should call `btrack_status` and answer.

## PATH troubleshooting

The most common failure mode is "MCP registered, tools never appear." Almost always a PATH issue: the GUI client launches with a different `PATH` than your shell.

### Symptoms

- `claude mcp list` shows `btrack: ✗ Failed`
- Tool names starting with `mcp__btrack__` never appear in the toolset
- The client logs `exec: "btrack": executable file not found in $PATH`

### Fixes

**1. Use an absolute path.** Find `btrack` and register the absolute path instead:

```bash
which btrack                                            # e.g. /opt/homebrew/bin/btrack
claude mcp remove btrack
claude mcp add btrack -- /opt/homebrew/bin/btrack mcp
```

**2. macOS Homebrew on Apple Silicon.** Brew installs to `/opt/homebrew/bin`, which GUI apps may not see. Either use the absolute path above, or symlink:

```bash
sudo ln -s /opt/homebrew/bin/btrack /usr/local/bin/btrack
```

**3. Switch to HTTP transport.** Bypasses the subprocess PATH entirely (see "HTTP transport" above).

## Verifying the install

After all of the above:

```bash
btrack --version              # binary works
claude mcp list | grep btrack # MCP connected
btrack skill install          # writes ~/.claude/skills/btrack/
ls ~/.claude/skills/btrack/   # SKILL.md, README.md, scripts/, references/
```

Restart Claude Code. In a fresh session, the toolset should include 10 `mcp__btrack__btrack_*` tools and the skill should auto-trigger when you start a coding task.

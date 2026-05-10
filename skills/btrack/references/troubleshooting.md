# Troubleshooting

Common issues with the btrack skill and MCP server, plus how to debug them.

## "The skill never auto-triggers"

**Symptom:** Claude doesn't call `btrack_start` when you begin a coding task. You have to ask manually.

**Causes and fixes:**

1. **Skill not installed.** Check `ls ~/.claude/skills/btrack/SKILL.md`. If missing, run `btrack skill install` or `npx skills add tolgazorlu/btrack`.
2. **Claude Code not restarted.** Skills load at app startup. Fully quit (Cmd+Q on macOS) and reopen.
3. **Skill description doesn't match the prompt.** The skill triggers on phrases like "implement", "fix the bug in", "refactor". Vague prompts ("can you help with this?") may not trigger it. Try: "let's fix bug X" or "implement Y."
4. **Conflicting skill.** If you have another time-tracking skill installed, frontmatter `description` keywords may overlap. Remove the other one or check whose triggers fire first.

## "MCP tools never appear"

**Symptom:** `mcp__btrack__btrack_*` tools missing from the toolset, even after restart.

**Diagnose:**

```bash
claude mcp list | grep btrack
```

Three possible states:

- `btrack: ✓ Connected` → MCP is fine. The issue is the skill (see above).
- `btrack: ✗ Failed` → MCP launch failed. See "PATH issues" below.
- (no output) → MCP not registered. Run `claude mcp add btrack -- btrack mcp`.

## PATH issues

**Symptom:** `claude mcp list` shows `btrack: ✗ Failed` or client logs `exec: "btrack": executable file not found in $PATH`.

The GUI Claude Code launches with a different `PATH` than your shell. Common offenders:

- macOS Apple Silicon (`/opt/homebrew/bin` not in GUI PATH)
- macOS GUI apps using a minimal PATH inherited from launchd
- Windows where `~\go\bin` isn't in user PATH

**Fix 1 — absolute path:**

```bash
which btrack                            # /opt/homebrew/bin/btrack
claude mcp remove btrack
claude mcp add btrack -- /opt/homebrew/bin/btrack mcp
```

**Fix 2 — symlink to a guaranteed location:**

```bash
sudo ln -s /opt/homebrew/bin/btrack /usr/local/bin/btrack
```

**Fix 3 — HTTP transport** (skips subprocess launch entirely):

```bash
btrack mcp --http             # in a long-running terminal
claude mcp remove btrack
claude mcp add --transport http btrack http://127.0.0.1:8765/mcp
```

The HTTP server reads/writes the same SQLite DB as the CLI, so state stays consistent.

## "Sessions are accumulating but never closing"

**Symptom:** `btrack w` shows a session open from hours ago. `btrack shipped` doesn't pair commits because sessions never stopped.

**Cause:** the skill started a session and Claude exited (or the conversation ended) before reaching `btrack_stop`.

**Fix in the moment:**

```bash
btrack x -m "ending forgotten session"
```

**Fix going forward:**

- Set an idle auto-stop: `btrack config idle 15` (auto-stop after 15 min idle).
- Make sure the skill's "Don'ts" rule is being followed — every Claude task that triggered the skill should reach `btrack_stop` before the conversation ends.

## "MCP registered but tools missing in the current session"

**Symptom:** You ran `claude mcp add btrack -- btrack mcp` while a Claude Code session was already open. Tools don't appear.

**Cause:** MCP servers load at app startup, not on `add`. The session that was open when you registered will never see them.

**Fix:** Fully quit and reopen Claude Code, or open a new session.

## "btrack: command not found" but I just installed it

**Symptom:** Brew/scoop installed btrack successfully, but `btrack --version` returns "command not found."

**Cause:** Your shell doesn't see the install location. Open a new terminal tab, or:

- macOS Apple Silicon Brew: ensure `/opt/homebrew/bin` is in PATH. Add to `~/.zshrc`: `export PATH="/opt/homebrew/bin:$PATH"`.
- Windows scoop: ensure `~\scoop\shims` is in PATH (usually automatic; restart terminal if not).
- Go install: ensure `$(go env GOPATH)/bin` is in PATH (typically `~/go/bin`).

## "The skill triggers but the wrong tools fire"

**Symptom:** Claude calls `btrack_start` correctly but then keeps using `btrack_status` instead of `btrack_log_note` for findings, or the wrong tools at wrong times.

**Cause:** The skill's tool guidance is verbose. Some clients (especially older models) don't follow it perfectly.

**Fix:** Be more directive in your prompts. "Track this with btrack and add a note when you find the root cause" works better than letting it infer.

## I want to remove the skill

```bash
rm -rf ~/.claude/skills/btrack
claude mcp remove btrack
```

Restart Claude Code. Your tracked sessions in `~/.config/btrack/btrack.db` are untouched.

## Reporting issues

If none of the above helps, open an issue at https://github.com/tolgazorlu/btrack/issues with:

- `btrack --version`
- `claude --version` (or your client + version)
- OS + arch
- Output of `claude mcp list`
- Output of `which btrack`

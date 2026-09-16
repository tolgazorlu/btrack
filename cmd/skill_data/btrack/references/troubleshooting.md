# Troubleshooting

Common issues with the btrack skill and CLI, plus how to debug them.

## "The skill never auto-triggers"

**Symptom:** Claude doesn't start a session when you begin a coding task. You have to ask manually.

**Causes and fixes:**

1. **Skill not installed.** Check `ls ~/.claude/skills/btrack/SKILL.md`. If missing, run `btrack skill install` or `npx skills add tolgazorlu/btrack`.
2. **Claude Code not restarted.** Skills load at app startup. Fully quit (Cmd+Q on macOS) and reopen.
3. **Skill description doesn't match the prompt.** The skill triggers on phrases like "implement", "fix the bug in", "refactor". Vague prompts ("can you help with this?") may not trigger it. Try: "let's fix bug X" or "implement Y."
4. **Conflicting skill.** If you have another time-tracking skill installed, frontmatter `description` keywords may overlap. Remove the other one or check whose triggers fire first.

## "btrack: command not found" but I just installed it

**Symptom:** Brew/scoop installed btrack successfully, but `btrack --version` returns "command not found."

**Cause:** Your shell doesn't see the install location. Open a new terminal tab, or:

- macOS Apple Silicon Brew: ensure `/opt/homebrew/bin` is in PATH. Add to `~/.zshrc`: `export PATH="/opt/homebrew/bin:$PATH"`.
- Windows scoop: ensure `~\scoop\shims` is in PATH (usually automatic; restart terminal if not).
- Go install: ensure `$(go env GOPATH)/bin` is in PATH (typically `~/go/bin`).

If your shell finds it but Claude Code doesn't, the GUI app launches with a
narrower `PATH`. Symlink the binary somewhere the GUI sees:

```bash
sudo ln -s /opt/homebrew/bin/btrack /usr/local/bin/btrack
```

## "Sessions are accumulating but never closing"

**Symptom:** `btrack w` shows a session open from hours ago.

**Cause:** the skill started a session and Claude exited (or the conversation ended) before stopping it.

**Fix in the moment:**

```bash
btrack x -m "ending forgotten session"
btrack x --at "2h ago"        # or backdate the stop time
```

**Fix going forward:**

- Set an idle auto-stop: `btrack config idle 15` (auto-stop after 15 min with no btrack activity).
- Set a hard cap: `btrack config max-hours 12` (auto-stops and tags `#runaway`).
- Make sure the skill's "Don'ts" rule is followed — every task that triggered the skill should end with `btrack x`.

## "Commands hang or report the daemon isn't running"

**Symptom:** `btrack s` / `btrack w` error out about the socket or daemon.

```bash
btrack daemon status
btrack daemon restart
```

Config changes to `idle` and `max-hours` only take effect after a daemon restart.

## "The skill triggers but does the wrong thing"

**Symptom:** Claude starts a session correctly but then never adds notes, or adds too many.

**Cause:** the skill's guidance is a heuristic, and some models follow it loosely.

**Fix:** Be more directive in your prompts. "Track this with btrack and add a note when you find the root cause" works better than letting it infer.

## I want to remove the skill

```bash
rm -rf ~/.claude/skills/btrack
```

Restart Claude Code. Your tracked sessions in the btrack database are untouched.

## Reporting issues

If none of the above helps, open an issue at https://github.com/tolgazorlu/btrack/issues with:

- `btrack --version`
- `claude --version` (or your client + version)
- OS + arch
- Output of `which btrack`
- Output of `btrack daemon status`

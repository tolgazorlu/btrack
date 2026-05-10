---
name: btrack-tracker
description: Use btrack to time-track coding sessions and tie them to git commits. Trigger this skill whenever the user starts a non-trivial coding task — implementing a feature, fixing a bug, refactoring, or working toward a commit — even if they don't mention btrack. Also trigger when the user mentions btrack, time tracking, sessions, standups, "what did I work on", `btrack shipped`, or asks Claude to log/track work. The skill walks through one-time MCP setup if needed, starts a session at the start of work, drops checkpoint notes during, and stops with a closing message right before the commit so `btrack shipped` lines up sessions with commits.
---

# btrack-tracker

btrack is a CLI time tracker for developers. It exposes a local MCP server with 10 tools (`btrack_start`, `btrack_stop`, `btrack_log_note`, etc.), so Claude Code can drive it directly — no shell calls needed once it's wired up.

Use this skill to:
1. **Ensure btrack MCP is registered** with Claude Code (one-time check per machine).
2. **Track the work you're about to do** — start a session, log notes for discoveries/blockers, stop with a meaningful closing message.
3. **Make `btrack shipped` work** — close the session right before `git commit` so the session window contains the commit.

## When to start a session

Start a session **before doing meaningful work** that will end in code changes. Good triggers:

- "implement X", "fix the bug in Y", "refactor Z", "add tests for W"
- The user gives you a multi-step coding task
- You're about to make changes that will become a commit

**Don't start a session for:**
- One-off questions ("what does this function do?")
- A single typo fix or trivial edit
- Pure exploration / Q&A with no code changes
- Read-only investigations

If unsure, ask: "Want me to track this as a btrack session?" — better to ask than to litter their history with 30-second sessions.

## Step 0 — Make sure the MCP is wired up

Check whether the btrack MCP tools are available in this Claude Code session. Look for tool names starting with `mcp__btrack__` in your toolset.

**If the tools are present** → skip to Step 1.

**If the tools are missing**, the user hasn't registered btrack with Claude Code yet. Run:

```bash
claude mcp list 2>&1 | grep -i btrack
```

If that returns nothing, register it:

```bash
claude mcp add btrack -- btrack mcp
```

Then tell the user: "btrack MCP registered. Restart Claude Code (or this session) so the tools load, then we can start tracking." The tools won't appear in the *current* session — registration takes effect on the next launch. Until then, fall back to the CLI (see the "CLI fallback" section).

If `btrack` itself isn't installed, point them at: `brew install tolgazorlu/btrack/btrack`.

## Step 1 — Start the session

Use `btrack_start` (or `btrack s "..." -p <project>` from the CLI). Pick a task name that describes the *intent*, not the mechanism — what will land in the commit message, roughly.

**Good task names:**
- `"fix JWT clock skew in auth middleware"`
- `"add /mcp status command"`
- `"refactor session repository to use sqlc"`

**Less good:**
- `"work on auth"` (too vague)
- `"edit auth.go"` (describes mechanism, not intent)

btrack auto-captures the git branch and repo, so don't repeat them in the task name. If the repo has a `.btrack` project file, the project flag is picked up automatically; otherwise pass `-p <project-name>`.

Tell the user one line: `→ btrack: tracking "fix JWT clock skew"`. Don't make a ceremony out of it.

## Step 2 — Log notes as you work

Drop a `btrack_log_note` whenever something *worth remembering tomorrow* happens. The bar is: would this help write the standup or debug a regression next week?

**Worth a note:**
- Found the root cause of a bug
- Hit a blocker / made a non-obvious decision
- Discovered something surprising in the codebase
- Finished a meaningful sub-task in a longer session

**Not worth a note:**
- Every file you read
- Every test that passed
- Routine progress

A session with 0–3 notes is normal. A session with 15 notes is noise.

## Step 3 — Stop the session before the commit

Right before you run `git commit`, stop the session with a closing message that mirrors the commit summary. This is what makes `btrack shipped` useful — the session window needs to contain the commit timestamp.

```
btrack_stop  message="fixed JWT clock skew by allowing ±60s drift"
```

Then commit. The order matters:

1. `btrack_stop -m "..."`
2. `git commit -m "..."`

If the user runs many small commits in one logical session, you have two reasonable patterns:
- **One session per commit** — stop, commit, restart with the next task name. Cleaner history, more work.
- **One session per logical chunk** — keep the session running across several commits, stop at the end. Less granular but lower friction.

Default to the second unless the user asks for per-commit tracking.

## Step 4 — Tie commit messages to the session

When you draft the commit message, look at the active or just-stopped session's task name and notes. The closing message + a couple of notes usually compose into a good commit body.

**Example:**

Session task: `"fix JWT clock skew in auth middleware"`
Notes: `"root cause: server clock 45s ahead of token issuer"`, `"added 60s leeway to verifier"`
Closing message: `"fixed by adding ±60s tolerance"`

→ Commit:

```
fix(auth): tolerate ±60s clock skew in JWT verifier

Server clock was 45s ahead of the token issuer; verifier rejected
freshly-issued tokens. Added a 60s leeway window.
```

## Tool reference

| MCP tool | When to use |
|---|---|
| `mcp__btrack__btrack_status` | Check if a session is active before starting another |
| `mcp__btrack__btrack_start` | Start a new session |
| `mcp__btrack__btrack_stop` | Stop the active session, optionally with closing message |
| `mcp__btrack__btrack_switch` | Atomic stop + start when pivoting tasks |
| `mcp__btrack__btrack_resume` | Continue the most recently stopped session (if you stopped to context-switch) |
| `mcp__btrack__btrack_log_note` | Add a checkpoint note to the active session |
| `mcp__btrack__btrack_history` | Look at recent sessions (`today`, `yesterday`, `week`, `last_n:20`) |
| `mcp__btrack__btrack_search` | Full-text search across past sessions |
| `mcp__btrack__btrack_list_projects` | List known projects with cumulative time |
| `mcp__btrack__btrack_get_session` | Pull a single session by ID with all its notes |

Always check `btrack_status` before `btrack_start` — if a session is already running, `start` will refuse. Use `btrack_switch` when the user has changed tasks instead.

## CLI fallback

If MCP tools aren't available (not registered yet, or `btrack mcp` failed to launch), every action has a CLI equivalent. Run via Bash:

| MCP tool | CLI equivalent |
|---|---|
| `btrack_status` | `btrack w` |
| `btrack_start` | `btrack s "task" -p project` |
| `btrack_stop` | `btrack x -m "message"` |
| `btrack_switch` | `btrack sw "new task"` |
| `btrack_resume` | `btrack r` |
| `btrack_log_note` | `btrack n "note"` |
| `btrack_history` | `btrack h` (today), `btrack h -w`, `btrack h -n 20` |
| `btrack_search` | `btrack search "query"` |

The CLI commits state to the same SQLite DB the MCP uses, so it doesn't matter which path you take — the user's history is consistent either way.

## Common patterns

**User says "let's fix the login bug":**

1. Check `btrack_status` — if active session, ask whether to switch.
2. `btrack_start` task=`"fix login bug"`.
3. Investigate. If you find the cause, drop a note.
4. Make the fix. Maybe another note for the key decision.
5. Run tests. If they pass, `btrack_stop` with a closing message.
6. `git commit` with a message informed by the closing message and notes.

**User says "what did I work on yesterday?":**

`btrack_history` window=`yesterday` → render as a short list. No need to start a session.

**User says "draft my standup":**

`btrack_history` window=`yesterday` → group by project, summarize. The CLI has `btrack standup` which does the AI summarization itself; if available, prefer that over reimplementing it.

**User asks for `btrack shipped`:**

Run `btrack shipped` via Bash from inside the repo. It cross-references sessions with `git log` and shows which commits landed during each session. Useful as a sanity check after a long day.

## Don'ts

- **Don't start a session and forget to stop it.** Every Claude Code task that triggered this skill should end with `btrack_stop` (or `btrack_switch` if pivoting). Open sessions accumulate fake hours.
- **Don't log a note for every tool call.** Notes are signal, not telemetry.
- **Don't rename `task` mid-session.** If the work has changed, `btrack_switch` to a new task — preserves history honesty.
- **Don't track meta-work** ("read btrack docs", "set up the MCP") unless the user is specifically billing or reporting on it.

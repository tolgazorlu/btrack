---
name: btrack
description: Use btrack to time-track coding sessions from the CLI. Trigger this skill whenever the user starts a non-trivial coding task — implementing a feature, fixing a bug, refactoring, or working toward a commit — even if they don't mention btrack. Also trigger when the user mentions btrack, time tracking, sessions, "what did I work on", or asks Claude to log/track work. The skill notes the wall-clock time when work begins and records the finished session with `btrack add --from --to` when it ends (clock mode) — or drives a live timer session when the user prefers.
license: MIT
metadata:
  author: tolgazorlu
  version: "0.7.0"
  homepage: "https://btrack.dev"
  repository: "https://github.com/tolgazorlu/btrack"
---

# btrack

btrack is a CLI time tracker for developers. Drive it with plain Bash calls —
`btrack s`, `btrack n`, `btrack x`. Everything is stored locally in SQLite.

Use this skill to:

1. **Track the work you're about to do** — start a session, log notes for
   discoveries and blockers, stop with a meaningful closing message.
2. **Answer questions about past work** — history, stats, search.

## When to start a session

Start a session **before doing meaningful work** that will end in code changes.
Good triggers:

- "implement X", "fix the bug in Y", "refactor Z", "add tests for W"
- The user gives you a multi-step coding task
- You're about to make changes that will become a commit

**Don't start a session for:**

- One-off questions ("what does this function do?")
- A single typo fix or trivial edit
- Pure exploration / Q&A with no code changes
- Read-only investigations

If unsure, ask: "Want me to track this as a btrack session?" — better to ask
than to litter their history with 30-second sessions.

If `btrack` isn't installed, point them at `brew install tolgazorlu/btrack/btrack`
(full install matrix in [references/installation.md](references/installation.md)).

## Two ways to record a session

**Clock mode (preferred for AI assistants).** Don't run a timer at all. When
you begin the work, note the current wall-clock time (`date +%H:%M`). When the
work is done, record the whole session in one call — btrack computes the
duration:

```bash
btrack add "fix JWT clock skew in auth middleware" --from 14:03 --to 15:10 -p myapp -m "fixed by adding ±60s tolerance #bugfix"
```

This has no failure mode of a forgotten running timer, and it costs one
command instead of a start/stop pair. Use `--date 2026-09-16` for past days
and `--for 45m` when you know the duration instead of the end time.

**Timer mode (live sessions).** `btrack s` / `btrack n` / `btrack x` — use it
when the user is driving from the terminal themselves, or explicitly asks for
a live session (`btrack w` shows a running counter).

## Step 1 — Start the session (timer mode)

Check for an active session first; `start` refuses when one is already running.

```bash
btrack w                          # is something already running?
btrack s "fix JWT clock skew in auth middleware" -p myapp
```

Pick a task name that describes the *intent*, not the mechanism — roughly what
will land in the commit message.

**Good task names:**

- `"fix JWT clock skew in auth middleware"`
- `"add CSV export to the history command"`
- `"refactor session repository to use sqlc"`

**Less good:**

- `"work on auth"` (too vague)
- `"edit auth.go"` (describes mechanism, not intent)

btrack auto-captures the git branch and repo, so don't repeat them in the task
name. If the repo has a `.btrack` project file the project is picked up
automatically; otherwise pass `-p <project-name>`.

Tell the user one line: `→ btrack: tracking "fix JWT clock skew"`. Don't make a
ceremony out of it.

## Step 2 — Log notes as you work

```bash
btrack n "root cause: server clock 45s ahead of the token issuer"
```

Drop a note whenever something *worth remembering tomorrow* happens. The bar is:
would this help debug a regression next week?

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

## Step 3 — Stop the session

When the work is done — typically right before the commit — stop with a closing
message that mirrors what actually changed.

```bash
btrack x -m "fixed JWT clock skew by allowing ±60s drift #bugfix"
```

Tags (`#bugfix`, `#feature`, `#refactor`, `#test`, `#docs`) go at the end of the
message.

If the user makes many small commits in one logical session, two patterns work:

- **One session per commit** — stop, commit, restart with the next task name.
  Cleaner history, more friction.
- **One session per logical chunk** — keep it running across several commits,
  stop at the end.

Default to the second unless the user asks for per-commit tracking.

## Step 4 — Tie the commit message to the session

When you draft the commit message, look at the just-stopped session's task name
and notes. The closing message plus a couple of notes usually compose into a
good commit body.

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

## Command reference

| Command | When to use |
|---|---|
| `btrack add "task" --from 14:03 --to 15:10` | Record a finished session in one call (clock mode) |
| `btrack w` | Check whether a session is active before starting another |
| `btrack s "task" -p project` | Start a new session (timer mode) |
| `btrack x -m "message"` | Stop the active session with a closing message |
| `btrack sw "new task"` | Atomic stop + start when pivoting tasks |
| `btrack r` | Resume the most recently stopped session |
| `btrack n "note"` | Add a checkpoint note to the active session |
| `btrack h` | Recent sessions — also `h -w`, `h -m`, `h -n 20`, `h yesterday` |
| `btrack projects` | List known projects with cumulative time |
| `btrack export` | CSV/JSON export — `-p project`, `--days 30`, `--format json` |
| `btrack import hours.tsv -p client` | Backfill hours tracked elsewhere (always `--dry-run` first) |
| `btrack report client --from 21.08` | Client work report (commits + hours) as HTML |

## Common patterns

**User says "let's fix the login bug" (clock mode):**

1. `date +%H:%M` — note the start time (e.g. 14:03).
2. Investigate, fix, test — the actual work.
3. When done: `btrack add "fix login bug" --from 14:03 --to 15:10 -m "root cause: JWT clock skew #bugfix"`.
4. `git commit` with a message informed by what you recorded.

**Same task in timer mode (user drives the terminal):**

1. `btrack w` — if a session is active, ask whether to switch.
2. `btrack s "fix login bug"`.
3. Investigate. If you find the cause, drop a note.
4. Make the fix. Maybe another note for the key decision.
5. Run tests. If they pass, `btrack x -m "..."`.
6. `git commit` with a message informed by the closing message and notes.

**User says "what did I work on yesterday?":**

`btrack h yesterday` → render as a short list. No need to start a session.

**User says "draft my standup":**

`btrack h yesterday` → group by project, summarize in your own words. btrack
stores the raw sessions; the summarizing is your job.

**User asks "how much have I billed on this project?":**

`btrack h -p <project> -m` for the month, or `btrack export` for a CSV they can
hand to an accountant.

## Don'ts

- **Don't start a session and forget to stop it.** Every task that triggered
  this skill should end with `btrack x` (or `btrack sw` if pivoting). Open
  sessions accumulate fake hours.
- **Don't log a note for every tool call.** Notes are signal, not telemetry.
- **Don't rename the task mid-session.** If the work changed, `btrack sw` to a
  new task — it preserves history honesty.
- **Don't track meta-work** ("read btrack docs") unless the user is
  specifically billing or reporting on it.

## Further reading

- [references/installation.md](references/installation.md) — installing the btrack binary, PATH troubleshooting
- [references/troubleshooting.md](references/troubleshooting.md) — common issues (daemon not running, Windows install, missing Brew tap)

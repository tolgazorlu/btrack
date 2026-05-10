# Standup workflow

How to use btrack to write a morning standup from yesterday's tracked sessions instead of trying to remember what you did.

## The two paths

btrack offers two ways to generate a standup:

1. **`btrack standup`** — a dedicated command that uses your configured AI provider to summarize yesterday's sessions into the canonical "Yesterday / Today / Blockers" format. Best when you want a one-shot answer.
2. **The skill + MCP path** — Claude (or Cursor / Gemini) calls `btrack_history` itself, then formats the output however the user asks. Best when the user wants to chat about it ("which session took longest yesterday?").

The skill should prefer path 1 when the user just wants the standup, and path 2 when the user wants to explore.

## Path 1 — `btrack standup`

Run from any directory, no active session needed:

```bash
btrack standup              # yesterday (default)
btrack standup --today      # what you've done so far today
btrack standup --days 3     # last 3 working days, useful after a weekend
```

Requires an AI key configured via `btrack ai setup` (OpenAI, Claude, or Gemini).

Output:

```
Yesterday:
  • Fixed JWT auth bug (2h)
  • Wrote unit tests for auth module (1h)

Today:
  • PR review, deploy to staging

Blockers:
  • None
```

The skill, when asked for a standup, should run this command and pass the output to the user verbatim.

## Path 2 — through MCP

When the user wants to explore (not just generate), use `btrack_history`:

```
btrack_history window="yesterday"
```

Returns sessions grouped by start time with task names, durations, projects, and notes. Then summarize in whatever shape the user asked for:

- "what did I do yesterday?" → bulleted list, project-grouped
- "which session was longest?" → just that one session + duration
- "did I touch the auth module?" → filter by project or grep notes

For multi-day windows, prefer `window="week"` over multiple single-day calls.

## Tying notes into the standup

The reason for dropping `btrack_log_note` during a session is *exactly* this moment. Notes are the bullet points of your standup.

A session with a clear closing message and 1–3 notes produces a good standup line automatically:

```
Session: "fix JWT clock skew in auth middleware"
Notes:
  - root cause: server clock 45s ahead of token issuer
  - added 60s leeway window
Closing: "fixed by adding ±60s tolerance"

→ Standup line: "Fixed JWT clock skew in auth middleware — server clock
  was 45s ahead of issuer, added 60s leeway window."
```

## Common patterns

**User says "draft my standup":**

1. Try `btrack standup` first (CLI does the AI summarization with the user's configured provider).
2. If `btrack standup` errors with "no AI key configured", fall back to `btrack_history window="yesterday"` and summarize manually.
3. Show the result. Don't add commentary unless asked.

**User says "what should I bring up at standup":**

1. `btrack_history window="yesterday"` → look for sessions with notes flagged as blockers, or sessions with much longer durations than usual.
2. Surface those as candidates: "Yesterday's `fix JWT clock skew` session ran 3h and ended with a note about needing a clock-sync RFC — worth bringing up?"

**User says "track my standup as a session":**

This is meta-work. Default to *not* tracking it unless they specifically asked. If they did, use a short `btrack_start` like `"morning standup"`.

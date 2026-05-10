# `btrack shipped` workflow

`btrack shipped` cross-references your tracked sessions with `git log` and shows which commits landed during which session. It only works correctly if sessions are *stopped before commits*, not after. This page explains why and how.

## What `btrack shipped` does

Run from inside a git repo:

```bash
btrack shipped              # commits in today's sessions
btrack shipped -w           # this week
btrack shipped -m           # this month
btrack shipped -p myapp     # filter by project
```

Output:

```
[fix-jwt-clock-skew]    1h 48m
  abc1234  fix(auth): tolerate ±60s clock skew in JWT verifier

[review-pr-42]          24m
  def5678  refactor: extract validateToken
  9012abc  test: cover clock skew edge cases
```

It's useful for:
- Honest invoicing ("here's what I actually shipped this week")
- Standup prep ("here's what landed yesterday")
- Sanity check on session naming ("did my sessions reflect the work?")

## Why session timing matters

`btrack shipped` matches commits to sessions by time window. A commit at `10:42:15` falls into whichever session was active at `10:42:15`.

So this works:

```
10:00  btrack_start  "fix JWT clock skew"
10:30  [code, code, code]
10:42  btrack_stop   "fixed by adding ±60s tolerance"
10:43  git commit -m "fix(auth): tolerate ±60s clock skew"   ← commit time AFTER stop
```

Wait — that's wrong. The commit at 10:43 falls AFTER the session ended at 10:42, so `btrack shipped` won't pair them.

The correct order is the OPPOSITE:

```
10:00  btrack_start  "fix JWT clock skew"
10:30  [code, code, code]
10:42  git commit -m "fix(auth): tolerate ±60s clock skew"   ← commit while session active
10:43  btrack_stop   "fixed by adding ±60s tolerance"
```

Or commit-then-stop in tight succession (the more common pattern, since stop-then-commit risks the user seeing the "Now committing" message and actually doing it later).

**Recommended order in skill flow:**

1. `btrack_log_note` "ready to commit"
2. `git commit -m "..."`
3. `btrack_stop -m "..."`

Sessions ending RIGHT after commits is the cleanest pairing, and it works whether the user does many small commits or one big one per session.

## Per-commit vs per-session-chunk

If a logical task ships as multiple commits, you have two options:

**One session per logical chunk (recommended default):**

```
btrack_start  "refactor session repo to use sqlc"
git commit -m "refactor: introduce sqlc-generated queries"
git commit -m "refactor: replace bbolt session repo"
git commit -m "refactor: drop legacy SessionStore interface"
btrack_stop   "shipped sqlc migration in 3 commits"
```

`btrack shipped` will list all 3 commits under the one session. Cleaner standup line, slightly less granular.

**One session per commit:**

```
btrack_switch "refactor: sqlc generated queries"
git commit -m "..."
btrack_switch "refactor: replace bbolt session repo"
git commit -m "..."
btrack_switch "refactor: drop legacy interface"
git commit -m "..."
btrack_stop   "..."
```

More work in the skill (`switch` between every commit), but every commit gets its own session line. Pick this only when the user explicitly asks for per-commit billing.

## Common edge cases

**"I forgot to stop a session before committing."**

The session is still active when you check `btrack shipped`. `shipped` will pair the commit with that session correctly because the session is open. Just don't forget to stop it eventually.

**"I committed during a different session than the work."**

Likely the user switched tasks, hit "what's the status here" with another commit on a different branch, then went back. `btrack shipped` will show the commit under the wrong session line. There's no fix in btrack — just be aware that the report is honest about *when* the commit happened, not *what* logical work it was part of.

**"git rebase rewrote my commits."**

Commit timestamps survive rebase by default (`git rebase` preserves author date). `btrack shipped` will still pair correctly. If you used `--committer-date-is-author-date` or similar, no problem. If you used `git commit --amend --reset-author` repeatedly, the timestamps will diverge from session windows — `btrack shipped` will become noisy. Avoid amend-spam.

**"Squash merge on GitHub changed my commits."**

Squash merge produces a *new* commit with a *new* timestamp, after your original commits. `btrack shipped` won't pair the squash commit with your session. The original commits in your local branch still pair correctly. This is fine for invoicing if you bill from `btrack shipped` on your local branch, not from `main` after merge.

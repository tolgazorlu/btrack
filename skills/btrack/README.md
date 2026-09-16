# btrack skill

A Claude Code (and other skill-aware client) skill that lets your AI assistant time-track your coding sessions automatically using the [btrack](https://btrack.dev) CLI.

## What it does

When you start a non-trivial coding task, the skill:

1. Starts a btrack session with a meaningful task name.
2. Drops checkpoint notes when something worth remembering happens.
3. Stops the session with a closing message when the work is done.
4. Pulls real session data when you ask "what did I do yesterday?" — no hallucination.

## Prerequisites

- [`btrack`](https://btrack.dev) installed (`brew install tolgazorlu/btrack/btrack` on macOS/Linux; full matrix in [references/installation.md](references/installation.md))
- [Claude Code](https://claude.com/code) (or another skill-aware client)

## Install

**Via skills.sh:**

```bash
npx skills add tolgazorlu/btrack
```

**Via the btrack binary directly** (works without Node):

```bash
btrack skill install
```

Both write `~/.claude/skills/btrack/` so Claude Code picks it up at next launch.

The skill drives btrack through ordinary shell calls, so there's nothing else to
register — just reopen Claude Code once after installing.

## How it feels in use

```
You:  let's add a --quiet flag to btrack stop

Claude:  → btrack: tracking "add --quiet flag to btrack stop"
         [reads cmd/stop.go, makes changes, runs tests]
         → btrack: stopping with closing message "added --quiet flag"
         Now committing.
```

Then in the morning:

```
You:  what did I do yesterday?

Claude:  Yesterday (3h 12m total):
         • added --quiet flag to btrack stop (1h 48m)
         • PR review on #42 (1h 24m)
```

## Files in this skill

```
skills/btrack/
├── SKILL.md                    # the skill itself (loaded by Claude)
├── README.md                   # this file (human-readable overview)
├── metadata.json               # skills.sh manifest
└── references/
    ├── installation.md         # btrack binary install
    └── troubleshooting.md      # PATH / daemon / Windows issues
```

## License

MIT — same as btrack itself.

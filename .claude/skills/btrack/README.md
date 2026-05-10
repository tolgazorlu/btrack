# btrack skill

A Claude Code (and other skill-aware client) skill that lets your AI assistant time-track your coding sessions automatically using the [btrack](https://btrack.dev) CLI.

## What it does

When you start a non-trivial coding task, the skill:

1. Starts a btrack session with a meaningful task name.
2. Drops checkpoint notes when something worth remembering happens.
3. Stops the session right before `git commit` so [`btrack shipped`](https://btrack.dev/docs/shipped) lines commits up with sessions.
4. Pulls real session data when you ask "what did I do yesterday?" or want a standup — no hallucination.

## Prerequisites

- [`btrack`](https://btrack.dev) installed (`brew install tolgazorlu/btrack/btrack` on macOS/Linux; full matrix in [references/installation.md](references/installation.md))
- [Claude Code](https://claude.com/code) (or another MCP- and skill-aware client)

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

## One-time setup

After installing the skill, register the MCP server with Claude Code:

```bash
claude mcp add btrack -- btrack mcp
```

Or run the bundled setup script which does both check + register:

```bash
~/.claude/skills/btrack/scripts/setup.sh
```

Then **fully quit and reopen Claude Code** so it loads the skill + MCP together.

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
├── scripts/
│   └── setup.sh                # one-time MCP registration helper
└── references/
    ├── installation.md         # btrack binary install + MCP setup
    ├── standup-workflow.md     # morning standup recipe
    ├── shipped-workflow.md     # btrack shipped + git pairing
    └── troubleshooting.md      # MCP/PATH/Windows issues
```

## License

MIT — same as btrack itself.

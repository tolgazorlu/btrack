# btrack

> A fast, developer-native CLI time tracker with AI chat, Pomodoro, project billing, and GitHub integration.

[![Release](https://img.shields.io/github/v/release/tolgazorlu/btrack)](https://github.com/tolgazorlu/btrack/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](go.mod)

Track your work the way you code — from the terminal, with git context, notes, tags, AI-powered standups, and built-in Pomodoro timer.

---

## Features

- **Git-style workflow** — `btrack s "fix login bug"` · `btrack x -m "fixed JWT #bugfix"`
- **Live status TUI** — progress bar toward your daily target, elapsed time, recent notes
- **History master view** — `btrack h` for day/week/month/year/table with one command
- **Project grouping** — tag sessions with a project, filter history, track time per client
- **Pomodoro timer** — `btrack pomo` runs 25/5 cycles and auto-creates sessions
- **Invoicing** — generate billable invoices from sessions with per-project hourly rates
- **Shell prompt integration** — `btrack prompt` outputs current session for PS1/Starship
- **Idle auto-stop** — daemon auto-stops sessions after N minutes of inactivity
- **AI chat** — `btrack ai` opens an interactive chat with context about your sessions
- **AI standups** — generate a daily standup from your sessions with OpenAI, Claude, or Gemini
- **AI insights** — weekly productivity dashboard with charts and pattern analysis
- **GitHub integration** — connect your account to pull real commits and PRs into standups
- **Tags & search** — `#bugfix`, `#feature` auto-detected; full-text search across all sessions
- **Streak tracking** — 30-day calendar, current and longest working-day streaks
- **Export** — CSV or JSON for invoicing and reporting
- **Local first** — SQLite by default, PostgreSQL supported

---

## Installation

### macOS / Linux

```bash
brew install tolgazorlu/btrack/btrack
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex
```

### Go install

```bash
go install github.com/tolgazorlu/btrack@latest
```

### Manual

Download the latest binary from [Releases](https://github.com/tolgazorlu/btrack/releases/latest), extract, and add to your `PATH`.

---

## Quick Start

```bash
# Start tracking
btrack s "fix login redirect bug"

# Add a checkpoint note while working
btrack n "found the issue in auth middleware"
btrack n "JWT clock skew — need to sync server time"

# Stop and save with a message + tag
btrack x -m "fixed JWT clock skew #bugfix"

# See today's work as a tree
btrack h

# Live status (or just: btrack)
btrack w
```

---

## Daily Workflow

| Command | Alias | Description |
|---------|-------|-------------|
| `btrack start "task"` | `s` | Start a new session |
| `btrack start "task" -p myapp` | `s -p` | Start in a project |
| `btrack note "text"` | `n` | Add a checkpoint note |
| `btrack stop -m "msg"` | `x` | Stop and save |
| `btrack switch "new task"` | `sw` | Stop current + start new |
| `btrack resume` | `r` | Continue last session |
| `btrack break` | — | Pause for a break |

## View Your Work

All history is accessible through `btrack h` (alias: `hist`, `log`, `l`):

| Command | Description |
|---------|-------------|
| `btrack h` | Today as a tree (default) |
| `btrack h yesterday` | Yesterday's tree |
| `btrack h 2026-05-01` | Specific date |
| `btrack h -w` | This week |
| `btrack h -m` | This month (week-by-week) |
| `btrack h -y` | This year (month-by-month) |
| `btrack h -n 20` | Last 20 sessions as a table |
| `btrack h -n 20 -v` | With checkpoint notes |
| `btrack h -l 5` | Last 5 hours |
| `btrack h -n 50 -p myapp` | Filter table by project |
| `btrack w` | Live TUI status |
| `btrack stats` | Today / week / month snapshot |
| `btrack streak` | Working-day streak + 30-day calendar |
| `btrack tag #bugfix` | Filter by tag |
| `btrack search "JWT"` | Full-text search (alias: `f`) |

## Projects & Billing

Group sessions under a project and generate billable invoices.

```bash
# Start a session in a project
btrack s "fix auth bug" -p myapp
btrack s "write API docs" -p client-x

# List all projects with total time
btrack projects

# Generate an invoice for the current month
btrack invoice -p myapp -r 150

# Specific month
btrack invoice -p myapp -r 150 --month 2026-04

# Round to nearest 15 min (billing)
btrack invoice -p myapp -r 150 --round

# Save to file
btrack invoice -p myapp -r 150 --out invoice-may.txt

# Set a default rate in config (no need to pass -r every time)
btrack config project myapp rate 150
btrack invoice -p myapp   # uses rate from config
```

Invoice output:

```
  Project: myapp                        Rate: $150.00/h
  Period:  May 2026
  ──────────────────────────────────────────────────────────────
  Date        Task                                        Hours
  ──────────────────────────────────────────────────────────────
  2026-05-01  fix login redirect bug                      1.50
  2026-05-01  review PR #43                               0.50
  2026-05-02  write auth tests                            2.25
  ──────────────────────────────────────────────────────────────

  Total hours:  4.25
  Total amount: $637.50
```

## Pomodoro Timer

Built-in Pomodoro mode that automatically creates and stops sessions for each focus interval.

```bash
# Default 25/5 min cycles, 4 rounds then long break
btrack pomo "fix login bug"

# Custom intervals
btrack pomo "write tests" --work 45 --break 10

# Custom rounds
btrack pomo "deep work" --work 50 --break 10 --long-break 30 --rounds 3
```

Each focus interval creates a regular btrack session tagged `#pomo`. Sessions are stopped automatically when the interval ends. Press `q` to stop early.

## Shell Prompt Integration

Show your current session in the terminal prompt — outputs nothing when idle.

```bash
# Plain text: "fix login bug · 23m"
btrack prompt

# JSON for Starship
btrack prompt --format starship

# Machine-readable JSON
btrack prompt --format json
```

**Bash / Zsh** — add to `~/.bashrc` or `~/.zshrc`:
```bash
PS1='$(btrack prompt) $ '
```

**Starship** — add to `~/.config/starship.toml`:
```toml
[custom.btrack]
command = "btrack prompt --format starship"
when    = "btrack prompt"
format  = "[$output]($style) "
style   = "blue"
```

**Fish** — add to `~/.config/fish/config.fish`:
```fish
function fish_prompt
    set bt (btrack prompt)
    if test -n "$bt"
        echo -n "[$bt] "
    end
    echo -n "> "
end
```

## AI Features

```bash
# Configure an AI provider (OpenAI, Claude, or Gemini)
btrack ai setup

# Open interactive AI chat — context about your recent sessions
btrack ai

# Standup summary from today's sessions
btrack ai sum

# Last 3 days
btrack ai sum --days 3

# Productivity dashboard with AI analysis
btrack ai ins

# Stats only, no AI key needed
btrack ai ins --no-ai
```

The interactive chat (`btrack ai`) has full context about your recent sessions. Ask anything:
- *"What did I work on today?"*
- *"Write me a standup for this week"*
- *"When am I most productive?"*

## GitHub Integration

```bash
# Connect your GitHub account
btrack github connect

# See today's GitHub activity (commits, PRs, reviews)
btrack github status

# Import today's commits as btrack sessions
btrack github sync
```

Once connected:
- `btrack ai sum` includes your real commits and PRs in the standup
- `btrack ai ins` includes GitHub contribution stats
- `btrack h` shows GitHub activity below your sessions
- `btrack h -w` shows per-day commit/PR counts

## Tags

Tags are added automatically from keywords in stop messages:

| Keyword | Tag |
|---------|-----|
| fix, bug | `#bugfix` |
| feat, add, new | `#feature` |
| refactor, clean | `#refactor` |
| test | `#test` |
| doc, readme | `#docs` |
| ci, pipeline | `#ci` |

Or add them manually: `btrack x -m "done #bugfix #auth"`

## Configuration

```bash
# Show all settings
btrack config

# Set daily work target
btrack config hours 6

# Auto-stop session after 15 min of no btrack activity (0 = disabled)
btrack config idle 15

# Set hourly billing rate for a project
btrack config project myapp rate 150
```

Config file: `~/.config/btrack/config.yaml`

## Export

```bash
# CSV to stdout
btrack export

# JSON to file
btrack export --format json --out sessions.json

# Last 30 days only
btrack export --days 30 --out april.csv
```

## Edit Past Sessions

```bash
# Find session IDs
btrack h -n 20

# Edit task name or message
btrack edit 42 -t "fix JWT expiry bug"
btrack edit 42 -m "fixed clock skew in auth middleware #bugfix"
```

## Project Links

```bash
btrack repo             # show all links
btrack repo star        # open GitHub repository
btrack repo issue       # open a new issue / feedback
btrack repo releases    # see the changelog
```

---

## Building from Source

```bash
git clone https://github.com/tolgazorlu/btrack.git
cd btrack
go build -o btrack .
```

## Running Tests

```bash
go test ./...
```

---

## License

MIT — see [LICENSE](LICENSE)

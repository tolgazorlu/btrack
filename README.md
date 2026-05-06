# btrack

Every time tracker I tried wanted me to open a browser, log in, pick a workspace, and click through a dashboard. I just wanted to type one line and get back to work.

[![Release](https://img.shields.io/github/v/release/tolgazorlu/btrack)](https://github.com/tolgazorlu/btrack/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](go.mod)

---

## Install

**macOS / Linux**
```bash
brew install btrack
```

**Windows**
```powershell
irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex
```

**Go**
```bash
go install github.com/tolgazorlu/btrack@latest
```

Or grab a binary from [Releases](https://github.com/tolgazorlu/btrack/releases/latest).

---

## Quick start

```bash
btrack s "fix login bug"              # start tracking
btrack n "found the issue"            # add a note while working
btrack x -m "fixed JWT clock skew"   # stop and save
btrack h                              # see today's work
```

---

## Commands

### Core

| Command | Alias | What it does |
|---------|-------|--------------|
| `btrack start "task"` | `s` | Start a session |
| `btrack start "task" -p myapp` | `s -p` | Start in a project |
| `btrack note "text"` | `n` | Add a note to the active session |
| `btrack note "text" -i 42` | `n -i` | Add a note to a past session |
| `btrack stop -m "msg"` | `x` | Stop and save |
| `btrack switch "new task"` | `sw` | Stop current, start new |
| `btrack resume` | `r` | Continue last session |
| `btrack break` | — | Start a break |

### History

All history flows through `btrack h`:

```bash
btrack h                  # today (default)
btrack h yesterday
btrack h 2026-05-01
btrack h -w               # this week
btrack h -m               # this month
btrack h -y               # this year
btrack h -n 20            # last 20 sessions as a table
btrack h -n 20 -v         # with notes
btrack h -l 5             # last 5 hours
btrack h -n 50 -p myapp   # filter by project
```

### Other views

```bash
btrack w                  # live status TUI
btrack stats              # today / week / month snapshot
btrack streak             # working-day streak + 30-day calendar
btrack search "JWT"       # full-text search
btrack tag #bugfix        # filter by tag
btrack shipped            # git commits that landed during your sessions
```

---

## Standup

Generate a standup from your tracked sessions with AI. Defaults to yesterday — run it in the morning before your standup meeting.

```bash
btrack standup              # yesterday (default)
btrack standup --today      # today's sessions so far
btrack standup --days 3     # last 3 days
```

Output format:
```
Yesterday:
  • Fixed JWT auth bug (2h)
  • Wrote unit tests for auth module (1h)

Today:
  • PR review, deploy to staging

Blockers:
  • None
```

Requires an AI key: `btrack ai setup`

---

## Projects

Group sessions by project, filter history, generate invoices.

```bash
btrack s "fix auth" -p myapp        # start session in a project
btrack projects                      # list all projects with total time
btrack h -n 50 -p myapp             # history filtered by project
btrack config project myapp rate 150 # set hourly rate
```

### .btrack file

Drop a `.btrack` file in a repo root to set per-project defaults. `btrack s` picks it up automatically.

```bash
btrack init   # interactive wizard — creates .btrack for you
```

```ini
# .btrack
project     = myapp
task_prefix = [myapp]
daily_hours = 6
```

---

## Invoicing

```bash
btrack invoice -p myapp -r 150               # current month, stdout
btrack invoice -p myapp -r 150 --month 2026-04
btrack invoice -p myapp -r 150 --round       # round to 15 min
btrack invoice -p myapp --out invoice.txt    # save to file
```

---

## Editing sessions

```bash
btrack h -n 20                        # find session IDs
btrack edit 42 -t "new task name"
btrack edit 42 --start 09:00 --end 17:30
btrack edit 42 -p myapp -m "done #bugfix"
```

---

## Shell prompt

Show the active session in your terminal prompt. Outputs nothing when idle.

```bash
btrack shell zsh    # print ready-to-paste zsh snippet
btrack shell bash   # print ready-to-paste bash snippet
btrack shell fish   # print ready-to-paste fish snippet
```

**Zsh** — add to `~/.zshrc`:
```zsh
btrack_prompt() { btrack prompt 2>/dev/null; }
RPROMPT='$(btrack_prompt)'
```

**Bash** — add to `~/.bashrc`:
```bash
btrack_prompt() {
  local s=$(btrack prompt 2>/dev/null)
  [ -n "$s" ] && echo " $s"
}
PS1='\u@\h \w$(btrack_prompt) \$ '
```

**Fish** — add to `~/.config/fish/functions/fish_right_prompt.fish`:
```fish
function fish_right_prompt
  btrack prompt 2>/dev/null
end
```

**Starship** — add to `~/.config/starship.toml`:
```toml
[custom.btrack]
command = "btrack prompt --format starship"
when    = "btrack prompt"
format  = "[$output]($style) "
style   = "blue"
```

Result: `fix login bug · 23m` on the right side of your prompt.

---

## AI

```bash
btrack ai setup       # configure OpenAI, Claude, or Gemini
btrack ai             # interactive chat with session context
btrack ai sum         # standup from today's sessions
btrack ai sum --days 3
btrack ai ins         # productivity dashboard
```

---

## Google Calendar

Push sessions to Google Calendar after stopping.

**Setup (once):**
1. [Google Cloud Console](https://console.cloud.google.com) → new project → enable Calendar API
2. Credentials → OAuth 2.0 Client ID → **Desktop app**
3. Copy the client ID and secret

```bash
btrack gcal connect --client-id <id> --client-secret <secret>
btrack gcal auto-sync on   # push automatically after every stop
```

**Other commands:**
```bash
btrack gcal status         # check connection
btrack gcal sync           # push last 7 days
btrack gcal sync --days 30
btrack gcal push 42        # push a specific session
```

---

## Pomodoro

```bash
btrack pomo "write tests"                          # 25/5 default
btrack pomo "deep work" --work 45 --break 10
```

Each interval creates a regular session tagged `#pomo`. Press `q` to stop early.

---

## GitHub

```bash
btrack github connect   # connect your account
btrack github sync      # import today's commits as sessions
```

Once connected, `btrack standup` and `btrack ai sum` include real commits and PRs.

---

## Configuration

```bash
btrack config             # show all settings
btrack config hours 6     # daily hour target
btrack config idle 15     # auto-stop after 15 min idle (0 = off)
```

Config file: `~/.config/btrack/config.yaml`

---

## Export

```bash
btrack export                              # CSV to stdout
btrack export --format json --out data.json
btrack export --days 30 --out may.csv
```

---

## Build from source

```bash
git clone https://github.com/tolgazorlu/btrack.git
cd btrack
go build -o btrack .
```

---

## License

MIT

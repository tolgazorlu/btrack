# btrack

> **A minimalist, developer-centric time tracker built in Go.**

`btrack` is a fast, terminal-based time tracking tool designed specifically for developers. It allows you to seamlessly track time, take notes during sessions, tag your work, and even generate AI-assisted standup summaries—all without leaving your command line.

## Features

- **Blazing Fast CLI:** Start, stop, and note your time instantly from the terminal.
- **Developer First:** Git branch and repo tracking built-in.
- **AI Integrations:** Connects with OpenAI, Claude, or Gemini to generate standup summaries and weekly insights.
- **Powerful Views:** View your day as a tree (`btrack d`), see history (`btrack h`), or get live status (`btrack w`).
- **Flexible Export:** Export your tracking sessions to CSV or JSON formats.
- **Daemon Mode:** Runs in the background to ensure reliable session tracking and fast CLI response times.
- **Local First:** Uses SQLite by default to store all data locally, with support for PostgreSQL.

## Installation

### Using Go Install

```bash
go install github.com/tolgazorlu/btrack@latest
```

### Script (Windows)

```powershell
irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex
```

## Quick Start

```bash
# Start tracking a task
btrack start "fix login bug"
# Alias: btrack s "fix login bug"

# Add a note to the active session
btrack note "found the issue in auth.go"
# Alias: btrack n "found the issue in auth.go"

# Stop tracking and log a final message with tags
btrack stop -m "fixed it #bugfix #auth"
# Alias: btrack x -m "fixed it #bugfix #auth"
```

## Reviewing Your Work

```bash
# See today's sessions as a tree
btrack day

# See yesterday's sessions
btrack day yesterday

# View all past sessions
btrack history

# See live status of the current session
btrack status
```

## AI Setup (Optional)

`btrack` can generate standup summaries and insights using AI.

```bash
# Configure your preferred AI provider (OpenAI / Claude / Gemini)
btrack ai setup

# Generate a standup summary for today
btrack ai sum

# Generate weekly stats + AI analysis
btrack ai ins
```

## Configuration

Settings are stored at `~/.config/btrack/config.yaml`.
You can manage config via the CLI:

```bash
# Set daily work target (default: 8h)
btrack config hours 8

# Show all current settings
btrack config
```

## Security & Privacy

- All your tracking data is stored **locally** in your machine by default (`~/.local/share/btrack` or equivalent).
- API keys for AI integration are stored locally in the configuration file with restricted permissions.
- You have full control over what is exported or sent to the AI providers (only explicitly requested data is sent).

## License

MIT License. See [LICENSE](LICENSE) for more details.

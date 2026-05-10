#!/usr/bin/env bash
# Run once after installing the btrack skill.
# Verifies that btrack is on PATH and registers it with Claude Code's MCP.
#
# Usage:
#   ~/.claude/skills/btrack/scripts/setup.sh
#
# Idempotent — re-running is safe.

set -euo pipefail

if ! command -v btrack >/dev/null 2>&1; then
  echo "btrack is not installed or not on PATH."
  echo
  echo "Install:"
  echo "  macOS / Linux:  brew install tolgazorlu/btrack/btrack"
  echo "  Windows:        irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex"
  echo "  Go:             go install github.com/tolgazorlu/btrack@latest"
  echo
  echo "Releases: https://github.com/tolgazorlu/btrack/releases/latest"
  exit 1
fi

if ! command -v claude >/dev/null 2>&1; then
  echo "Claude Code CLI not found on PATH."
  echo "Install Claude Code: https://claude.com/code"
  echo
  echo "(The btrack skill is also compatible with Cursor, Gemini CLI, and"
  echo "Claude Desktop — see references/installation.md for those clients.)"
  exit 1
fi

if claude mcp list 2>&1 | grep -qE "^btrack:"; then
  echo "btrack MCP already registered with Claude Code."
else
  claude mcp add btrack -- btrack mcp
  echo "btrack MCP registered with Claude Code."
fi

echo
echo "Done."
echo "Fully quit and reopen Claude Code so it loads the skill and MCP together."
echo "Then start a coding task — the skill will begin tracking automatically."

# btrack updater for Windows
# Usage: irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/update.ps1 | iex

# Simply re-runs the installer (idempotent)
irm https://raw.githubusercontent.com/tolgazorlu/btrack/main/install.ps1 | iex

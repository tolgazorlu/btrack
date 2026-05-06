# Contributing to btrack

## Development setup

**Prerequisites:** Go 1.24+

```bash
git clone https://github.com/tolgazorlu/btrack.git
cd btrack
go build -o btrack .
go test ./...
```

## Architecture

| Package | Role |
|---------|------|
| `cmd/` | CLI commands (Cobra) |
| `internal/daemon/` | Background server + Unix socket IPC |
| `internal/db/` | SQLite / PostgreSQL storage |
| `internal/ai/` | OpenAI, Claude, Gemini providers |
| `internal/config/` | Config loading (Viper) |
| `internal/ui/` | Terminal UI components (Lipgloss, Bubbletea) |

## Coding standards

- File permissions: `0600` for sensitive files, `0750` for directories
- Always use `filepath.Clean()` before writing to paths
- Handle all errors explicitly — avoid silent `_ =` unless documented
- Run `go fmt ./...` and `go vet ./...` before committing

## Submitting a pull request

1. Fork the repo and branch from `main`
2. Add tests for new behavior
3. Ensure `go test ./...` passes
4. Open a PR — all PRs require review from the maintainer before merging

## Security

Do not open public issues for security vulnerabilities. Use GitHub's private vulnerability reporting instead.

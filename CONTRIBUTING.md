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

## Adding a skill

The btrack repo follows the [skills.sh](https://skills.sh) convention so that any skill in `skills/<name>/` is installable via `npx skills add tolgazorlu/btrack`. The canonical `btrack` skill is also embedded into the binary, installable via `btrack skill install`.

Quick start:

```bash
mkdir -p skills/<your-skill>/{scripts,references}
$EDITOR skills/<your-skill>/SKILL.md            # required
$EDITOR skills/<your-skill>/metadata.json       # skills.sh manifest
make sync-skill                                  # only refreshes the embedded btrack skill
go test ./cmd/                                   # confirms embedded skill is in sync
```

Full guide with frontmatter format, embedded-vs-doc-only decisions, and the PR checklist: [docs/contributing-skills](https://btrack.dev/docs/contributing-skills) (or see [`docs/content/docs/contributing-skills.mdx`](docs/content/docs/contributing-skills.mdx) in the repo).

## Security

Do not open public issues for security vulnerabilities. Use GitHub's private vulnerability reporting instead.

# Contributing to btrack

First off, thank you for considering contributing to `btrack`! It's people like you that make `btrack` such a great tool for developers.

## Development Setup

1. **Prerequisites:**
   - Go 1.24+ (Recommended 1.26.2+ for security fixes)
   - `make` (optional, for convenience)

2. **Clone the repository:**
   ```bash
   git clone https://github.com/tolgazorlu/btrack.git
   cd btrack
   ```

3. **Build the project:**
   ```bash
   make build
   # Or using go directly:
   go build -o btrack.exe main.go
   ```

4. **Run tests:**
   ```bash
   go test ./...
   ```

## Architecture

- **`cmd/`**: Contains the CLI commands using `cobra`.
- **`internal/daemon/`**: Contains the background server that handles session state and the client that communicates with it via Unix domain sockets.
- **`internal/db/`**: Handles database interactions (SQLite/PostgreSQL).
- **`internal/ai/`**: Manages integrations with OpenAI, Claude, and Gemini APIs.
- **`internal/config/`**: Manages configuration loading using `viper`.
- **`internal/ui/`**: Contains styling and layout components using `lipgloss`.

## Coding Standards

- **Security First:**
  - Always validate paths using `filepath.Clean()`.
  - Use restricted file permissions: `0600` for sensitive files, `0750` for directories.
  - Never trust user input, especially for file paths or shell commands.
- **Error Handling:**
  - Handle all errors explicitly. Do not use `_ =` for calls that return errors unless strictly necessary, and if so, document why.
- **Formatting:**
  - Run `go fmt ./...` before committing.
  - We use `golangci-lint` for linting. Ensure your code passes the linter.

## Submitting a Pull Request

1. Fork the repository and create your branch from `main`.
2. If you've added code that should be tested, add tests.
3. If you've changed APIs, update the documentation.
4. Ensure the test suite passes (`go test ./...`).
5. Run security scans: `gosec ./...` and `govulncheck ./...`.
6. Issue that pull request!

## Vulnerability Reporting

If you discover a security vulnerability within `btrack`, please do not open a public issue. Instead, send an email to the maintainers privately or use GitHub's private vulnerability reporting feature.

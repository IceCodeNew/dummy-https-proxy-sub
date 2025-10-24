# Repository Guidelines

This file documents contributor expectations and quick reference commands for this repository.

## Project Structure & Module Organization
- Root: project metadata and docs (`README.md`, `AGENTS.md`).
- Source: Go server and packages live under `./` (single-module Go project).
- Tests: Go unit tests use `_test.go` files next to corresponding packages.

## Build, Test, and Development Commands
- `GOFUMPT_SPLIT_LONG_LINES=on find . -type f -name '*.go' -print0 | xargs -0 -r -t -- go run mvdan.cc/gofumpt@latest -l -w` — format Go code.
- `go build ./...` — compile all packages.
- `go test ./...` — run unit tests.
- `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` — static checks.
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` - vulnerabilities checks.
- `go run go.uber.org/nilaway/cmd/nilaway@latest ./...` - detect potential nil panics.
- `go run github.com/alexkohler/prealloc@latest ./...` - find slice declarations that could potentially be preallocated.

## Coding Style & Naming Conventions
- Use `GOFUMPT_SPLIT_LONG_LINES=on find . -type f -name '*.go' -print0 | xargs -0 -r -t -- go run mvdan.cc/gofumpt@latest -l -w` for formatting.
- Exported identifiers use `CamelCase`; unexported use `camelCase` or short names in small scopes.
- File names: lowercase with underscores only if helpful, e.g. `server.go`, `match_utils.go`.
- Comments and code must be in English.

## Testing Guidelines
- Tests use Go's `testing` package. Place tests in `*_test.go` files adjacent to code.
- Aim for clear unit tests that cover edge cases (strict matching, case insensitivity, trailing slashes).
- Run tests with `go test ./...` and include short examples when helpful.

## Commit & Pull Request Guidelines
- Commit messages: short title, optional body. Example: `match: strict name matching for input tokens`.
- PRs should include a description, linked issue (if any), and screenshots or example requests/responses when relevant.
- Keep changes focused and split large work into multiple PRs.

## Security & Configuration Tips
- Do not commit secrets or API keys. Use environment variables for configuration.

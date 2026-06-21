# AGENTS.md

## Repo Shape
- This is a Go CLI module (`module github.com/ducng99/heyai`) with the installable command in `cmd/hey` and reusable CLI package in the repository root.
- `main.go` handles CLI flags and config loading; `chat.go` owns the tool-call loop; `openai.go` contains the Chat Completions client and bash tool schema; `guard/` implements command classification; `tool/bash.go` implements bash tool execution.
- The module declares `go 1.26`; CI uses `actions/setup-go` with `go-version-file: go.mod`.

## Commands
- Run all tests: `go test ./...`
- Run one test: `go test -run TestName ./...`
- Build like the README: `go build -o hey ./cmd/hey`
- Build all packages for verification: `go build ./...`
- Format touched Go files with `gofmt -w <files>`; there is no repo-specific linter or formatter config.

## CLI And Config Details
- Config is JSON only; `LoadConfig` reads `$XDG_CONFIG_HOME/heyai/config.json`, falling back to `~/.config/heyai/config.json`.
- `--init` writes a starter config with `0600` permissions and errors if the file already exists.
- Runtime requires `api_key` in config; there is no environment-variable fallback for the API key.
- `base_url` may be either a host root or end in `/v1`; `chatCompletionsURL` normalizes both to `/v1/chat/completions`.

## Bash Tool Gotchas
- The bash guard is a static classifier, not a sandbox; keep tests around any changes to `CheckBash`, tokenization, path checks, or wrapper-command handling.
- `CheckBash` treats the process current working directory as the current-dir boundary. Absolute paths outside it, `..`, `~`, and `workdir` outside it require confirmation rather than automatic denial.
- Empty bash commands are invalid and denied without prompting.
- Only `go test` is classified safe among `go` commands; `go mod tidy` and other mutating or unknown commands require confirmation.
- `RunBash` executes through `bash -lc`, applies the lower of config/tool timeouts when provided, and truncates combined stdout/stderr by `max_output_bytes`.

## File Tool Gotchas
- `Read`, `Edit`, `Write`, and `Patch` share confirmation plumbing for sensitive paths. `.env` and `.env.*` require confirmation before the tool reads or modifies the file.

## Release Notes
- The release workflow only runs on tags matching `v[0-9]*` and builds static artifacts with `CGO_ENABLED=0` for linux/darwin/windows amd64.
- `cliff.toml` filters changelog entries to conventional commits; non-conventional commit messages are excluded from generated release notes.

## Documentation Maintenance
- Always check if `README.md` and `AGENTS.md` need to be updated after making changes to the codebase.

# heyai

`heyai` is a minimal Go CLI for an OpenAI-compatible Chat Completions API. It accepts a prompt, sends it to the configured model, and supports a guarded `bash` tool for read-only and explicitly approved local commands.

## Build

```bash
go build -o heyai .
```

## Configuration

Config is JSON only and is loaded from:

```txt
$XDG_CONFIG_HOME/heyai/config.json
```

If `XDG_CONFIG_HOME` is unset, the fallback is:

```txt
~/.config/heyai/config.json
```

Create a starter config:

```bash
heyai --init
```

Example:

```json
{
  "api_key": "sk-...",
  "base_url": "https://api.openai.com",
  "model": "gpt-4o-mini",
  "max_turns": 8,
  "bash": {
    "timeout_ms": 30000,
    "allow_risky_without_confirm": false,
    "max_output_bytes": 20000
  }
}
```

## Usage

```bash
heyai "list the Go files and summarize the project"
heyai --config-path
heyai --help
```

## Bash Tool Security Model

The bash tool uses a static guard before executing commands. It classifies commands as safe, confirmation-required, or denied.

Safe commands are intended to be read-only, such as `pwd`, `ls`, `cat`, `grep`, `find` without mutation, and `go test ./...`.

Commands that write or mutate files require confirmation by default, such as `rm`, `mv`, `cp`, `mkdir`, `touch`, `go mod tidy`, `npm install`, `find -delete`, and nested destructive commands in `find -exec` or `xargs`.

Commands are denied when they use unsupported shell features, privileged execution, nested shells, or paths outside the current working directory root.

Important: this is a static guard, not a true sandbox. It reduces risk but cannot perfectly contain arbitrary processes. True containment requires OS sandboxing such as containers, bubblewrap, chroot, seccomp, or similar.

## Verification

```bash
go test ./...
go build ./...
./heyai --help
```

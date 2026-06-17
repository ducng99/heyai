# hey

`hey` is a minimal Go CLI for an OpenAI-compatible Chat Completions API. It accepts a prompt, sends it to the configured model, and supports local tools for guarded bash commands, reading files, listing directories, and modifying files.

Assistant responses are rendered as Markdown when stdout is an interactive terminal. Redirected or piped output remains raw Markdown-friendly text.

## Installation

### Prebuilt binaries

Download the latest release for your platform from the
[GitHub Releases](https://github.com/ducng99/heyai/releases) page, extract
the archive, and place the `hey` binary somewhere on your `$PATH`.

### Go install

```bash
go install github.com/ducng99/heyai/cmd/hey@latest
```

### Build from source

```bash
go build -o hey ./cmd/hey
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
hey --init
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
hey "list the Go files and summarize the project"
hey --config-path
hey --help
```

## Bash Tool Security Model

The bash tool uses a static guard before executing commands. It classifies commands as safe, confirmation-required, or invalid.

Safe commands are intended to be read-only, such as `pwd`, `ls`, `cat`, `grep`, `find` without mutation, and `go test ./...`.

Commands that write, mutate files, use elevated privileges, run nested shells, use shell expansion, or reference paths outside the current directory require confirmation by default. Examples include `sudo`, `su`, `rm`, `mv`, `cp`, `mkdir`, `touch`, `go mod tidy`, `npm install`, `find -delete`, `bash -c`, `$(...)`, `/etc/passwd`, `../file`, and nested commands in `find -exec` or `xargs`.

Empty commands are rejected as invalid instead of prompting.

When `allow_risky_without_confirm` is `true`, confirmation-required commands run without prompting. Safe commands always run without prompting.

Important: this is a static guard, not a true sandbox. It reduces risk but cannot perfectly contain arbitrary processes. True containment requires OS sandboxing such as containers, bubblewrap, chroot, seccomp, or similar.

## File Tools

The `Read` tool reads a file's content or lists the direct entries in a directory.

The `Edit` tool replaces exactly one occurrence of a string in a file. If the string is not found or appears more than once, the tool returns an error instructing the assistant to read the file and provide a more exact replacement target.

The `Write` tool replaces an entire file with supplied content.

The `Patch` tool applies a single-file unified diff patch to a file.

When `--readonly` or `-r` is enabled, `Edit`, `Write`, and `Patch` are not advertised to the model.

## Verification

```bash
go test ./...
go build -o hey ./cmd/hey
./hey --help
```

package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type RiskLevel int

const (
	RiskSafe RiskLevel = iota
	RiskNeedsConfirm
	RiskDenied
)

type GuardResult struct {
	Risk   RiskLevel
	Reason string
}

var deniedWords = map[string]bool{
	"sudo": true, "su": true, "eval": true, "source": true, "cd": true,
}

var destructiveWords = map[string]bool{
	"rm": true, "mv": true, "cp": true, "mkdir": true, "touch": true, "rmdir": true,
	"install": true, "tee": true, "chmod": true, "chown": true, "go": true, "npm": true,
}

var safeWords = map[string]bool{
	"pwd": true, "ls": true, "cat": true, "grep": true, "find": true, "go": true, "curl": true,
}

func CheckBash(command, workdir string) (GuardResult, error) {
	if strings.TrimSpace(command) == "" {
		return GuardResult{Risk: RiskDenied, Reason: "empty command"}, nil
	}
	if strings.Contains(command, "\x00") {
		return GuardResult{Risk: RiskNeedsConfirm, Reason: "command contains a null byte"}, nil
	}
	for _, feature := range []string{"$(", "`", "<(", ">("} {
		if strings.Contains(command, feature) {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "command uses shell expansion " + feature + ", which can hide additional commands"}, nil
		}
	}

	root, err := os.Getwd()
	if err != nil {
		return GuardResult{}, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return GuardResult{}, err
	}
	if workdir != "" {
		wd, err := resolvePath(root, workdir)
		if err != nil || !insideRoot(root, wd) {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "workdir is outside the current dir: " + workdir}, nil
		}
	}

	tokens, err := tokenize(command)
	if err != nil {
		return GuardResult{Risk: RiskNeedsConfirm, Reason: err.Error()}, nil
	}
	if len(tokens) == 0 {
		return GuardResult{Risk: RiskDenied, Reason: "empty command"}, nil
	}
	return inspectTokens(tokens, root)
}

func inspectTokens(tokens []string, root string) (GuardResult, error) {
	risk := RiskSafe
	parts := splitCommands(tokens)
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		res := inspectCommand(part, root)
		if res.Risk == RiskDenied {
			return res, nil
		}
		if res.Risk > risk {
			risk = res.Risk
		}
	}
	if risk == RiskNeedsConfirm {
		return GuardResult{Risk: risk, Reason: "one or more commands may write, mutate files, or need elevated trust"}, nil
	}
	return GuardResult{Risk: RiskSafe, Reason: "read-only command"}, nil
}

func inspectCommand(tokens []string, root string) GuardResult {
	cmd := tokens[0]
	if deniedWords[cmd] {
		return GuardResult{Risk: RiskNeedsConfirm, Reason: "command may change shell state or run with elevated privileges: " + cmd}
	}
	if isShellExec(cmd, tokens) {
		return GuardResult{Risk: RiskNeedsConfirm, Reason: "nested interpreter execution with inline code can hide additional commands: " + cmd}
	}
	if res := inspectPathsAndRedirs(tokens, root); res.Risk == RiskDenied {
		return res
	} else if res.Risk == RiskNeedsConfirm {
		return res
	}

	switch cmd {
	case "curl":
		return inspectCurl(tokens)
	case "find":
		return inspectFind(tokens, root)
	case "xargs", "parallel":
		if len(tokens) == 1 {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "wrapper command has no visible child command: " + cmd}
		}
		return inspectCommand(tokens[1:], root)
	case "env", "command", "nice":
		idx := wrapperCommandIndex(cmd, tokens)
		if idx < 0 || idx >= len(tokens) {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "wrapper command has no visible child command: " + cmd}
		}
		return inspectCommand(tokens[idx:], root)
	case "timeout":
		if len(tokens) < 3 {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "timeout wrapper has no visible child command"}
		}
		return inspectCommand(tokens[2:], root)
	}

	if destructiveWords[cmd] {
		if cmd == "go" && len(tokens) > 1 && tokens[1] == "test" {
			return GuardResult{Risk: RiskSafe, Reason: "go test"}
		}
		return GuardResult{Risk: RiskNeedsConfirm, Reason: "command may write, install, delete, or mutate project files: " + cmd}
	}
	if !safeWords[cmd] {
		return GuardResult{Risk: RiskNeedsConfirm, Reason: "command is not in the read-only allowlist: " + cmd}
	}
	return GuardResult{Risk: RiskSafe, Reason: "read-only command"}
}

func inspectPathsAndRedirs(tokens []string, root string) GuardResult {
	for i, t := range tokens {
		if isRedirection(t) {
			if i+1 >= len(tokens) {
				return GuardResult{Risk: RiskNeedsConfirm, Reason: "redirection operator has no target: " + t}
			}
			res := checkPathToken(tokens[i+1], root)
			if res.Risk == RiskDenied {
				return GuardResult{Risk: RiskNeedsConfirm, Reason: "redirection target is outside the current dir: " + tokens[i+1]}
			}
			if res.Risk == RiskNeedsConfirm {
				return GuardResult{Risk: RiskNeedsConfirm, Reason: "redirection target is outside the current dir: " + tokens[i+1]}
			}
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "redirection may read or write file content: " + t + " " + tokens[i+1]}
		}
		if looksPath(t) {
			if res := checkPathToken(t, root); res.Risk != RiskSafe {
				return res
			}
		}
	}
	return GuardResult{Risk: RiskSafe}
}

func checkPathToken(token string, root string) GuardResult {
	if token == "/" || strings.HasPrefix(token, "~/") || token == "~" || hasParentTraversal(token) {
		return GuardResult{Risk: RiskNeedsConfirm, Reason: "path is outside the current dir: " + token}
	}
	if strings.HasPrefix(token, "/") {
		abs := filepath.Clean(token)
		if !insideRoot(root, abs) {
			return GuardResult{Risk: RiskNeedsConfirm, Reason: "path is outside the current dir: " + token}
		}
	}
	return GuardResult{Risk: RiskSafe}
}

func hasParentTraversal(token string) bool {
	for _, part := range strings.Split(filepath.ToSlash(token), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func resolvePath(root, p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Abs(filepath.Clean(p))
}

func insideRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func looksPath(t string) bool {
	return strings.HasPrefix(t, "/") || strings.HasPrefix(t, "./") || strings.HasPrefix(t, "../") || strings.HasPrefix(t, "~/") || strings.Contains(t, "/")
}

func isRedirection(t string) bool {
	return t == ">" || t == ">>" || t == "<" || t == "2>" || t == "2>>" || t == "&>" || t == "&>>"
}

func isShellExec(cmd string, tokens []string) bool {
	if cmd != "bash" && cmd != "sh" && cmd != "zsh" && cmd != "fish" && cmd != "python" && cmd != "perl" && cmd != "ruby" && cmd != "node" {
		return false
	}
	for _, t := range tokens[1:] {
		if t == "-c" || t == "-e" {
			return true
		}
	}
	return false
}

func wrapperCommandIndex(cmd string, tokens []string) int {
	switch cmd {
	case "env":
		for i := 1; i < len(tokens); i++ {
			if strings.Contains(tokens[i], "=") || strings.HasPrefix(tokens[i], "-") {
				continue
			}
			return i
		}
	case "command", "nice":
		for i := 1; i < len(tokens); i++ {
			if strings.HasPrefix(tokens[i], "-") {
				continue
			}
			return i
		}
	}
	return -1
}

func splitCommands(tokens []string) [][]string {
	var parts [][]string
	start := 0
	for i, t := range tokens {
		if t == "&&" || t == "||" || t == ";" || t == "|" {
			parts = append(parts, tokens[start:i])
			start = i + 1
		}
	}
	parts = append(parts, tokens[start:])
	return parts
}

func tokenize(s string) ([]string, error) {
	var tokens []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			flush()
			continue
		}
		if r == ';' || r == '|' || r == '&' || r == '>' || r == '<' {
			flush()
			tokens = appendOperator(tokens, r)
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return mergeFdRedirs(tokens), nil
}

func appendOperator(tokens []string, r rune) []string {
	t := string(r)
	if len(tokens) == 0 {
		return append(tokens, t)
	}
	last := tokens[len(tokens)-1]
	if (last == "&" && (t == "&" || t == ">")) || (last == "|" && t == "|") || (last == ">" && t == ">") || (last == "<" && t == "<") {
		tokens[len(tokens)-1] = last + t
		return tokens
	}
	return append(tokens, t)
}

func mergeFdRedirs(tokens []string) []string {
	var out []string
	for i := 0; i < len(tokens); i++ {
		if _, err := strconv.Atoi(tokens[i]); err == nil && i+1 < len(tokens) && (tokens[i+1] == ">" || tokens[i+1] == ">>") {
			out = append(out, tokens[i]+tokens[i+1])
			i++
			continue
		}
		out = append(out, tokens[i])
	}
	return out
}

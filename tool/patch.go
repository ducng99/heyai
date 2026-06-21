package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type PatchTool struct {
	Options ToolOptions
}

type PatchArgs struct {
	FilePath string `json:"filePath"`
	Patch    string `json:"patch"`
}

type PatchResult struct {
	Path  string `json:"path"`
	Hunks int    `json:"hunks"`
}

type patchHunk struct {
	OldStart int
	OldCount int
	NewLines []patchLine
}

type patchLine struct {
	Kind byte
	Text string
}

func (PatchTool) Definition() Definition {
	return Definition{Name: "Patch", Description: "Apply a single-file unified diff patch to a file.", Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]string{"type": "string"},
			"patch":    map[string]string{"type": "string"},
		},
		"required": []string{"filePath", "patch"},
	}}
}

func (t PatchTool) Run(ctx context.Context, raw json.RawMessage) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var args PatchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.FilePath == "" {
		return nil, errors.New("filePath is required")
	}
	if args.Patch == "" {
		return nil, errors.New("patch is required")
	}
	if isSensitivePath(args.FilePath) && !confirmToolAction(t.Options, "Patch file (contains potential secrets)", "Sensitive file", args.FilePath) {
		return nil, errors.New("not approved")
	}

	b, err := os.ReadFile(args.FilePath)
	if err != nil {
		return nil, err
	}
	hunks, err := parseUnifiedPatch(args.Patch)
	if err != nil {
		return nil, err
	}
	updated, err := applyUnifiedPatch(string(b), hunks)
	if err != nil {
		return nil, fmt.Errorf("patch did not apply to %s: %w", args.FilePath, err)
	}
	if err := os.WriteFile(args.FilePath, []byte(updated), 0644); err != nil {
		return nil, err
	}
	return PatchResult{Path: args.FilePath, Hunks: len(hunks)}, nil
}

func parseUnifiedPatch(patch string) ([]patchHunk, error) {
	lines := splitPatchLines(patch)
	hunks := []patchHunk{}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") {
			continue
		}
		if !strings.HasPrefix(line, "@@ ") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return nil, fmt.Errorf("expected hunk header, got %q", line)
		}
		hunk, err := parseHunkHeader(line)
		if err != nil {
			return nil, err
		}
		i++
		for ; i < len(lines); i++ {
			line = lines[i]
			if strings.HasPrefix(line, "@@ ") {
				i--
				break
			}
			if line == `\ No newline at end of file` {
				continue
			}
			if line == "" {
				return nil, errors.New("empty patch line must start with a space, +, or -")
			}
			kind := line[0]
			if kind != ' ' && kind != '+' && kind != '-' {
				return nil, fmt.Errorf("invalid patch line %q", line)
			}
			hunk.NewLines = append(hunk.NewLines, patchLine{Kind: kind, Text: line[1:]})
		}
		hunks = append(hunks, hunk)
	}
	if len(hunks) == 0 {
		return nil, errors.New("patch must contain at least one unified diff hunk")
	}
	return hunks, nil
}

func parseHunkHeader(header string) (patchHunk, error) {
	parts := strings.Split(header, " ")
	if len(parts) < 3 || parts[0] != "@@" || !strings.HasPrefix(parts[1], "-") || !strings.HasPrefix(parts[2], "+") {
		return patchHunk{}, fmt.Errorf("invalid hunk header %q", header)
	}
	oldStart, oldCount, err := parseRange(parts[1][1:])
	if err != nil {
		return patchHunk{}, fmt.Errorf("invalid old range in hunk header %q: %w", header, err)
	}
	return patchHunk{OldStart: oldStart, OldCount: oldCount}, nil
}

func parseRange(s string) (int, int, error) {
	parts := strings.SplitN(s, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	count := 1
	if len(parts) == 2 {
		count, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
	}
	return start, count, nil
}

func applyUnifiedPatch(content string, hunks []patchHunk) (string, error) {
	lines, trailingNewline := splitContentLines(content)
	var out []string
	pos := 0
	for _, hunk := range hunks {
		start := hunk.OldStart - 1
		if hunk.OldStart == 0 {
			start = 0
		}
		if start < pos || start > len(lines) {
			return "", fmt.Errorf("hunk starts at line %d outside current file content", hunk.OldStart)
		}
		out = append(out, lines[pos:start]...)
		pos = start
		consumed := 0
		for _, patchLine := range hunk.NewLines {
			switch patchLine.Kind {
			case ' ':
				if pos >= len(lines) || lines[pos] != patchLine.Text {
					return "", fmt.Errorf("context mismatch near line %d", pos+1)
				}
				out = append(out, lines[pos])
				pos++
				consumed++
			case '-':
				if pos >= len(lines) || lines[pos] != patchLine.Text {
					return "", fmt.Errorf("removal mismatch near line %d", pos+1)
				}
				pos++
				consumed++
			case '+':
				out = append(out, patchLine.Text)
			}
		}
		if consumed != hunk.OldCount {
			return "", fmt.Errorf("hunk expected to consume %d old lines but consumed %d", hunk.OldCount, consumed)
		}
	}
	out = append(out, lines[pos:]...)
	updated := strings.Join(out, "\n")
	if trailingNewline {
		updated += "\n"
	}
	return updated, nil
}

func splitContentLines(content string) ([]string, bool) {
	if content == "" {
		return nil, false
	}
	trailingNewline := strings.HasSuffix(content, "\n")
	content = strings.TrimSuffix(content, "\n")
	return strings.Split(content, "\n"), trailingNewline
}

func splitPatchLines(patch string) []string {
	patch = strings.ReplaceAll(patch, "\r\n", "\n")
	patch = strings.TrimSuffix(patch, "\n")
	if patch == "" {
		return nil
	}
	return strings.Split(patch, "\n")
}

package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type EditTool struct {
	Options ToolOptions
}

type EditArgs struct {
	FilePath    string `json:"filePath,omitempty"`
	Path        string `json:"path,omitempty"`
	OldString   string `json:"oldString,omitempty"`
	NewString   string `json:"newString,omitempty"`
	Replace     string `json:"replace,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

type EditResult struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
}

func (EditTool) Definition() Definition {
	return Definition{Name: "Edit", Description: "Replace exactly one occurrence of a string in a file.", Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath":    map[string]string{"type": "string"},
			"path":        map[string]string{"type": "string"},
			"oldString":   map[string]string{"type": "string"},
			"newString":   map[string]string{"type": "string"},
			"replace":     map[string]string{"type": "string"},
			"replacement": map[string]string{"type": "string"},
		},
	}}
}

func (t EditTool) Run(ctx context.Context, raw json.RawMessage) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var args EditArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	path := args.FilePath
	if path == "" {
		path = args.Path
	}
	oldString := args.OldString
	if oldString == "" {
		oldString = args.Replace
	}
	newString := args.NewString
	if newString == "" {
		newString = args.Replacement
	}
	if path == "" {
		return nil, errors.New("filePath is required")
	}
	if oldString == "" {
		return nil, errors.New("string to replace is required")
	}
	if isSensitivePath(path) && !confirmToolAction(t.Options, "Edit file (contains potential secrets)", "Sensitive file", path) {
		return nil, errors.New("not approved")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(b)
	count := strings.Count(content, oldString)
	if count == 0 {
		return nil, fmt.Errorf("string to replace was not found in %s; call Read to inspect the current file content and provide an exact oldString", path)
	}
	if count > 1 {
		return nil, fmt.Errorf("string to replace occurs %d times in %s; call Read and provide a larger, unique oldString with more surrounding context", count, path)
	}

	updated := strings.Replace(content, oldString, newString, 1)
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return nil, err
	}
	return EditResult{Path: path, Replacements: 1}, nil
}

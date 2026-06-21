package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
)

type WriteTool struct {
	Options ToolOptions
}

type WriteArgs struct {
	FilePath string `json:"filePath,omitempty"`
	Path     string `json:"path,omitempty"`
	Content  string `json:"content"`
}

type WriteResult struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}

func (WriteTool) Definition() Definition {
	return Definition{Name: "Write", Description: "Replace an entire file with new content.", Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filePath": map[string]string{"type": "string"},
			"path":     map[string]string{"type": "string"},
			"content":  map[string]string{"type": "string"},
		},
		"required": []string{"content"},
	}}
}

func (t WriteTool) Run(ctx context.Context, raw json.RawMessage) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var args WriteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	path := args.FilePath
	if path == "" {
		path = args.Path
	}
	if path == "" {
		return nil, errors.New("filePath is required")
	}
	if isSensitivePath(path) && !confirmToolAction(t.Options, "Write file (contains potential secrets)", "Sensitive file", path) {
		return nil, errors.New("not approved")
	}
	if err := os.WriteFile(path, []byte(args.Content), 0644); err != nil {
		return nil, err
	}
	return WriteResult{Path: path, Bytes: len(args.Content)}, nil
}

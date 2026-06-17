package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
)

type ReadTool struct{}

type ReadArgs struct {
	Path     string `json:"path,omitempty"`
	FilePath string `json:"filePath,omitempty"`
}

type ReadResult struct {
	Path    string   `json:"path"`
	Type    string   `json:"type"`
	Content string   `json:"content,omitempty"`
	Files   []string `json:"files,omitempty"`
}

func (ReadTool) Definition() Definition {
	return Definition{Name: "Read", Description: "Read a file's content or list files in a directory.", Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":     map[string]string{"type": "string"},
			"filePath": map[string]string{"type": "string"},
		},
	}}
}

func (ReadTool) Run(ctx context.Context, raw json.RawMessage) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var args ReadArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	path := args.Path
	if path == "" {
		path = args.FilePath
	}
	if path == "" {
		return nil, errors.New("path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		files := make([]string, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			files = append(files, name)
		}
		sort.Strings(files)
		return ReadResult{Path: path, Type: "directory", Files: files}, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadResult{Path: path, Type: "file", Content: string(b)}, nil
}

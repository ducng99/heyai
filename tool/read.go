package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

type ReadTool struct {
	Options ToolOptions
}

type ReadArgs struct {
	Path     string `json:"path,omitempty"`
	FilePath string `json:"filePath,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
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
			"offset":   map[string]string{"type": "integer", "description": "Optional 1-indexed line number to start reading from."},
			"limit":    map[string]string{"type": "integer", "description": "Optional maximum number of lines to read."},
		},
	}}
}

func (t ReadTool) Run(ctx context.Context, raw json.RawMessage) (any, error) {
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
	if args.Offset < 0 {
		return nil, errors.New("offset must be greater than or equal to 0")
	}
	if args.Limit < 0 {
		return nil, errors.New("limit must be greater than or equal to 0")
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
	if isSensitivePath(path) && !confirmToolAction(t.Options, "Read file (contains potential secrets)", "Sensitive file", path) {
		return nil, errors.New("not approved")
	}

	if args.Offset > 0 || args.Limit > 0 {
		content, err := readFileRange(ctx, path, args.Offset, args.Limit)
		if err != nil {
			return nil, err
		}
		return ReadResult{Path: path, Type: "file", Content: content}, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(b)
	return ReadResult{Path: path, Type: "file", Content: content}, nil
}

func readFileRange(ctx context.Context, path string, offset, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	startLine := 1
	if offset > 0 {
		startLine = offset
	}
	for line := 1; line < startLine; line++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		_, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
	}

	var content strings.Builder
	for linesRead := 0; limit == 0 || linesRead < limit; {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			content.WriteString(line)
			linesRead++
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return content.String(), nil
}

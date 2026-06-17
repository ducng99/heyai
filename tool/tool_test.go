package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadToolReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := (ReadTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	read, ok := result.(ReadResult)
	if !ok {
		t.Fatalf("result type=%T", result)
	}
	if read.Type != "file" || read.Content != "hello" {
		t.Fatalf("result=%#v", read)
	}
}

func TestReadToolListsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a"), 0755); err != nil {
		t.Fatal(err)
	}

	result, err := (ReadTool{}).Run(context.Background(), json.RawMessage(`{"path":`+quote(dir)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	read := result.(ReadResult)
	if strings.Join(read.Files, ",") != "a/,b.txt" {
		t.Fatalf("files=%#v", read.Files)
	}
}

func TestEditToolReplacesUniqueOccurrence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := (EditTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"oldString":"world","newString":"gopher"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello gopher" {
		t.Fatalf("content=%q", string(b))
	}
}

func TestEditToolRejectsMultipleOccurrences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("x x"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := (EditTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"oldString":"x","newString":"y"}`))
	if err == nil || !strings.Contains(err.Error(), "provide a larger, unique oldString") {
		t.Fatalf("err=%v", err)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

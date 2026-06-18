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

func TestReadToolReadsFileLineRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := (ReadTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"offset":2,"limit":2}`))
	if err != nil {
		t.Fatal(err)
	}
	read := result.(ReadResult)
	if read.Content != "two\nthree\n" {
		t.Fatalf("content=%q", read.Content)
	}
}

func TestReadToolRejectsNegativeLineRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := (ReadTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"offset":-1}`))
	if err == nil || !strings.Contains(err.Error(), "offset") {
		t.Fatalf("err=%v", err)
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

func TestWriteToolReplacesEntireFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := (WriteTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"content":"new content"}`))
	if err != nil {
		t.Fatal(err)
	}
	write := result.(WriteResult)
	if write.Bytes != len("new content") {
		t.Fatalf("bytes=%d", write.Bytes)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new content" {
		t.Fatalf("content=%q", string(b))
	}
}

func TestPatchToolAppliesUnifiedDiff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	patch := "--- a/note.txt\n+++ b/note.txt\n@@ -1,3 +1,4 @@\n one\n-two\n+2\n three\n+four\n"

	result, err := (PatchTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"patch":`+quote(patch)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	patched := result.(PatchResult)
	if patched.Hunks != 1 {
		t.Fatalf("hunks=%d", patched.Hunks)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "one\n2\nthree\nfour\n" {
		t.Fatalf("content=%q", string(b))
	}
}

func TestPatchToolRejectsMismatchedContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	patch := "@@ -1,2 +1,2 @@\n nope\n-two\n+2\n"

	_, err := (PatchTool{}).Run(context.Background(), json.RawMessage(`{"filePath":`+quote(path)+`,"patch":`+quote(patch)+`}`))
	if err == nil || !strings.Contains(err.Error(), "context mismatch") {
		t.Fatalf("err=%v", err)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

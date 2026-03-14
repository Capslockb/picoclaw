package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHostPreviewToolUsesFileEntryByDefault(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "index.html")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotRoot, gotEntry, gotSlug string
	tool := NewHostPreviewTool(workspace, true, func(root, entry, slug string) (*HostedPreview, error) {
		gotRoot, gotEntry, gotSlug = root, entry, slug
		return &HostedPreview{Root: root, Entry: entry, LocalURL: "http://127.0.0.1:3002/preview/test/index.html"}, nil
	})

	result := tool.Execute(context.Background(), map[string]any{"path": file, "slug": "test"})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if gotRoot != workspace {
		t.Fatalf("root mismatch: got %q want %q", gotRoot, workspace)
	}
	if gotEntry != "index.html" {
		t.Fatalf("entry mismatch: got %q", gotEntry)
	}
	if gotSlug != "test" {
		t.Fatalf("slug mismatch: got %q", gotSlug)
	}
}

func TestHostPreviewToolUsesIndexForDirectory(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "site")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotEntry string
	tool := NewHostPreviewTool(workspace, true, func(root, entry, slug string) (*HostedPreview, error) {
		gotEntry = entry
		return &HostedPreview{Root: root, Entry: entry, LocalURL: "http://127.0.0.1:3002/preview/test/"}, nil
	})

	result := tool.Execute(context.Background(), map[string]any{"path": project})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if gotEntry != "index.html" {
		t.Fatalf("entry mismatch: got %q", gotEntry)
	}
}

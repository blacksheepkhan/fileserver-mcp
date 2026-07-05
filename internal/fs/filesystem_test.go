package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blacksheepkhan/fileserver-mcp/internal/security"
)

func TestNewLocalFileSystemRejectsEmptyRoot(t *testing.T) {
	t.Parallel()

	_, err := NewLocalFileSystem("")

	if !errors.Is(err, security.ErrEmptyRoot) {
		t.Fatalf("expected ErrEmptyRoot, got %v", err)
	}
}

func TestLocalFileSystemListReturnsFilesAndDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeTestFile(t, filepath.Join(root, "file.txt"), "hello")
	mkdir(t, filepath.Join(root, "subdir"))

	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %#v", len(entries), entries)
	}

	fileEntry := findEntry(t, entries, "file.txt")
	if fileEntry.IsDir {
		t.Fatal("expected file.txt to be a file")
	}

	if fileEntry.Size != int64(len("hello")) {
		t.Fatalf("expected file size %d, got %d", len("hello"), fileEntry.Size)
	}

	dirEntry := findEntry(t, entries, "subdir")
	if !dirEntry.IsDir {
		t.Fatal("expected subdir to be a directory")
	}
}

func TestLocalFileSystemListEmptyDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf("expected empty directory, got %d entries", len(entries))
	}
}

func TestLocalFileSystemListNestedDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nestedDir := filepath.Join(root, "alpha", "beta")

	mkdir(t, nestedDir)
	writeTestFile(t, filepath.Join(nestedDir, "nested.txt"), "nested-content")

	filesystem := mustNewLocalFileSystem(t, root)

	entries, err := filesystem.List(filepath.Join("alpha", "beta"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := findEntry(t, entries, "nested.txt")
	if entry.IsDir {
		t.Fatal("expected nested.txt to be a file")
	}

	if entry.Size != int64(len("nested-content")) {
		t.Fatalf("expected size %d, got %d", len("nested-content"), entry.Size)
	}
}

func TestLocalFileSystemListRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List("..")

	if !errors.Is(err, security.ErrPathTraversal) {
		t.Fatalf("expected ErrPathTraversal, got %v", err)
	}
}

func TestLocalFileSystemListRejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List(filepath.Join(root, "file.txt"))

	if !errors.Is(err, security.ErrAbsolutePath) {
		t.Fatalf("expected ErrAbsolutePath, got %v", err)
	}
}

func TestLocalFileSystemListReturnsErrorForMissingDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filesystem := mustNewLocalFileSystem(t, root)

	_, err := filesystem.List("missing")

	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func mustNewLocalFileSystem(t *testing.T, root string) *LocalFileSystem {
	t.Helper()

	filesystem, err := NewLocalFileSystem(root)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	return filesystem
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
}

func findEntry(t *testing.T, entries []Entry, name string) Entry {
	t.Helper()

	for _, entry := range entries {
		if entry.Name == name {
			return entry
		}
	}

	t.Fatalf("entry %q not found in %#v", name, entries)

	return Entry{}
}

package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const smallFileContent = "FlashGate benchmark text.\n"

type corpus struct {
	root string
}

func createCorpus() (_ corpus, err error) {
	root, err := os.MkdirTemp("", "flashgate-benchmark-")
	if err != nil {
		return corpus{}, fmt.Errorf("create benchmark corpus: %w", err)
	}
	created := false
	defer func() {
		if !created {
			_ = os.RemoveAll(root)
		}
	}()
	root, err = filepath.Abs(root)
	if err != nil {
		return corpus{}, fmt.Errorf("normalize benchmark corpus path: %w", err)
	}
	root = filepath.Clean(root)

	directories := []string{"small-dir", "large-dir", "path-checks", "read-files"}
	for _, relative := range directories {
		if err := os.Mkdir(filepath.Join(root, relative), 0o700); err != nil {
			return corpus{}, fmt.Errorf("create corpus directory: %w", err)
		}
	}

	files := map[string][]byte{
		"existing.txt":    []byte(smallFileContent),
		"small.txt":       []byte(smallFileContent),
		"text-64kib.txt":  []byte(strings.Repeat("x", 64*1024)),
		"small-dir/a.txt": []byte("a"),
		"small-dir/b.txt": []byte("bb"),
		"small-dir/c.txt": []byte("ccc"),
	}
	for relative, content := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), content, 0o600); err != nil {
			return corpus{}, fmt.Errorf("write corpus file: %w", err)
		}
	}

	for index := 0; index < 500; index++ {
		name := fmt.Sprintf("entry-%04d.txt", index)
		content := []byte(strings.Repeat("x", index%17))
		if err := os.WriteFile(filepath.Join(root, "large-dir", name), content, 0o600); err != nil {
			return corpus{}, fmt.Errorf("write large directory fixture: %w", err)
		}
	}

	for index := 0; index < 10; index++ {
		name := fmt.Sprintf("entry-%02d.txt", index)
		if err := os.WriteFile(filepath.Join(root, "path-checks", name), []byte{byte('0' + index)}, 0o600); err != nil {
			return corpus{}, fmt.Errorf("write path-check fixture: %w", err)
		}
		if err := os.WriteFile(filepath.Join(root, "read-files", name), []byte(smallFileContent), 0o600); err != nil {
			return corpus{}, fmt.Errorf("write read fixture: %w", err)
		}
	}

	created = true
	return corpus{root: root}, nil
}

func (c corpus) remove() error {
	return os.RemoveAll(c.root)
}

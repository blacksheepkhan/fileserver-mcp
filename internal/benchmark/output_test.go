package benchmark

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteResultFileCreatesAndReplacesRegularFile(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	output := filepath.Join(root, "build", "benchmark-current.json")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := WriteResultFile(output, protected, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := WriteResultFile(output, protected, []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second\n" {
		t.Fatalf("output=%q, want second result", data)
	}
}

func TestWriteResultFileRejectsProtectedParentAlias(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	alias := filepath.Join(root, "alias")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(protected, alias); err != nil {
		skipUnsupportedSymlink(t, err)
	}

	err := WriteResultFile(filepath.Join(alias, "benchmark-current.json"), protected, []byte("blocked\n"))
	if err == nil || !strings.Contains(err.Error(), "protected baseline directory") {
		t.Fatalf("protected parent alias error=%v", err)
	}
}

func TestWriteResultFileRejectsFinalAndBrokenSymlinks(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	outputDirectory := filepath.Join(root, "build")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(protected, "baseline.windows-amd64.json")
	baselineData := []byte("baseline\n")
	if err := os.WriteFile(baseline, baselineData, 0o600); err != nil {
		t.Fatal(err)
	}
	baselineHash := sha256.Sum256(baselineData)

	for _, tc := range []struct {
		name   string
		target string
	}{
		{name: "baseline target", target: baseline},
		{name: "broken target", target: filepath.Join(root, "missing.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(outputDirectory, strings.ReplaceAll(tc.name, " ", "-")+".json")
			if err := os.Symlink(tc.target, output); err != nil {
				skipUnsupportedSymlink(t, err)
			}
			err := WriteResultFile(output, protected, []byte("blocked\n"))
			if err == nil || !strings.Contains(err.Error(), "must not be a symbolic link") {
				t.Fatalf("final symlink error=%v", err)
			}
		})
	}

	currentBaseline, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(currentBaseline) != baselineHash {
		t.Fatal("protected baseline changed through a final symlink")
	}
}

func TestWriteResultFileReplacesBaselineHardLinkWithoutWritingThroughIt(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	outputDirectory := filepath.Join(root, "build")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(protected, "baseline.windows-amd64.json")
	baselineData := []byte("baseline\n")
	if err := os.WriteFile(baseline, baselineData, 0o600); err != nil {
		t.Fatal(err)
	}
	baselineHash := sha256.Sum256(baselineData)
	output := filepath.Join(outputDirectory, "benchmark-current.json")
	if err := os.Link(baseline, output); err != nil {
		t.Skipf("hard links are not supported by the current filesystem: %v", err)
	}

	if err := WriteResultFile(output, protected, []byte("diagnostic\n")); err != nil {
		t.Fatal(err)
	}
	currentBaseline, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(currentBaseline) != baselineHash {
		t.Fatal("protected baseline changed through a hard-link alias")
	}
	outputData, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(outputData, []byte("diagnostic\n")) {
		t.Fatalf("output=%q, want diagnostic result", outputData)
	}
}

func TestWriteResultFileReplacesLateSymlinkWithoutFollowingIt(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	outputDirectory := filepath.Join(root, "build")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(protected, "baseline.linux-amd64.json")
	baselineData := []byte("baseline\n")
	if err := os.WriteFile(baseline, baselineData, 0o600); err != nil {
		t.Fatal(err)
	}
	baselineHash := sha256.Sum256(baselineData)
	output := filepath.Join(outputDirectory, "benchmark-current.json")
	if err := os.WriteFile(output, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := writeResultFile(output, protected, []byte("new\n"), func() error {
		if err := os.Remove(output); err != nil {
			return err
		}
		return os.Symlink(baseline, output)
	})
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symbolic links are not supported by the current configuration: %v", err)
		}
		t.Fatal(err)
	}
	currentBaseline, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(currentBaseline) != baselineHash {
		t.Fatal("protected baseline changed during late target exchange")
	}
	outputData, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(outputData, []byte("new\n")) {
		t.Fatalf("output=%q, want committed result", outputData)
	}
}

func TestWriteResultFileFailsClosedWhenParentChangesAfterValidation(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	outputDirectory := filepath.Join(root, "build")
	boundDirectory := filepath.Join(root, "build-bound")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(protected, "baseline.linux-amd64.json")
	baselineData := []byte("baseline\n")
	if err := os.WriteFile(baseline, baselineData, 0o600); err != nil {
		t.Fatal(err)
	}
	baselineHash := sha256.Sum256(baselineData)
	output := filepath.Join(outputDirectory, "benchmark-current.json")

	err := writeResultFile(output, protected, []byte("blocked\n"), func() error {
		if err := os.Rename(outputDirectory, boundDirectory); err != nil {
			return err
		}
		return os.Symlink(protected, outputDirectory)
	})
	if err != nil && (errors.Is(err, os.ErrPermission) || (runtime.GOOS == "windows" && strings.Contains(err.Error(), "being used by another process"))) {
		t.Skipf("parent exchange is not supported by the current configuration: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "parent changed after validation") {
		t.Fatalf("parent exchange error=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(boundDirectory, "benchmark-current.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("bound output was not removed after parent exchange: %v", statErr)
	}
	currentBaseline, readErr := os.ReadFile(baseline)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if sha256.Sum256(currentBaseline) != baselineHash {
		t.Fatal("protected baseline changed during late parent exchange")
	}
}

func TestWriteResultFileFailsClosedWhenProtectedDirectoryChangesAfterValidation(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "benchmarks")
	boundProtected := filepath.Join(root, "benchmarks-bound")
	outputDirectory := filepath.Join(root, "build")
	if err := os.Mkdir(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(protected, "baseline.linux-amd64.json")
	baselineData := []byte("baseline\n")
	if err := os.WriteFile(baseline, baselineData, 0o600); err != nil {
		t.Fatal(err)
	}
	baselineHash := sha256.Sum256(baselineData)
	output := filepath.Join(outputDirectory, "benchmark-current.json")

	err := writeResultFile(output, protected, []byte("blocked\n"), func() error {
		if err := os.Rename(protected, boundProtected); err != nil {
			return err
		}
		return os.Symlink(outputDirectory, protected)
	})
	if err != nil && (errors.Is(err, os.ErrPermission) || (runtime.GOOS == "windows" && strings.Contains(err.Error(), "being used by another process"))) {
		t.Skipf("protected-directory exchange is not supported by the current configuration: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "protected baseline directory changed after validation") {
		t.Fatalf("protected-directory exchange error=%v", err)
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output was not removed after protected-directory exchange: %v", statErr)
	}
	currentBaseline, readErr := os.ReadFile(filepath.Join(boundProtected, filepath.Base(baseline)))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if sha256.Sum256(currentBaseline) != baselineHash {
		t.Fatal("protected baseline changed during late protected-directory exchange")
	}
}

func skipUnsupportedSymlink(t *testing.T, err error) {
	t.Helper()
	if runtime.GOOS == "windows" && errors.Is(err, os.ErrPermission) {
		t.Skipf("symbolic links are not supported by the current Windows configuration: %v", err)
	}
	t.Fatal(err)
}

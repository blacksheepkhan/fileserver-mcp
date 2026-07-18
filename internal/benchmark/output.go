package benchmark

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// WriteResultFile writes one benchmark result without following the final
// destination entry. The output parent is bound through os.Root for the entire
// operation, and the protected baseline directory is rejected by file identity.
func WriteResultFile(outputPath string, protectedBaselineDirectory string, data []byte) error {
	return writeResultFile(outputPath, protectedBaselineDirectory, data, nil)
}

func writeResultFile(outputPath string, protectedBaselineDirectory string, data []byte, beforeRename func() error) (returnErr error) {
	if outputPath == "" {
		return fmt.Errorf("benchmark output path is required")
	}
	if protectedBaselineDirectory == "" {
		return fmt.Errorf("protected baseline directory is required")
	}

	outputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("normalize benchmark output path: %w", err)
	}
	outputPath = filepath.Clean(outputPath)
	outputParent := filepath.Dir(outputPath)
	outputName := filepath.Base(outputPath)
	if outputName == "." || outputName == string(filepath.Separator) {
		return fmt.Errorf("benchmark output path must name a file")
	}

	protectedBaselineDirectory, err = filepath.Abs(protectedBaselineDirectory)
	if err != nil {
		return fmt.Errorf("normalize protected baseline directory: %w", err)
	}
	protectedBaselineDirectory = filepath.Clean(protectedBaselineDirectory)

	if err := os.MkdirAll(outputParent, 0o755); err != nil {
		return fmt.Errorf("create benchmark output directory: %w", err)
	}

	protectedRoot, err := os.OpenRoot(protectedBaselineDirectory)
	if err != nil {
		return fmt.Errorf("open protected baseline directory: %w", err)
	}
	defer func() {
		if closeErr := protectedRoot.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close protected baseline directory: %w", closeErr)
		}
	}()

	outputRoot, err := os.OpenRoot(outputParent)
	if err != nil {
		return fmt.Errorf("open benchmark output directory: %w", err)
	}
	defer func() {
		if closeErr := outputRoot.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close benchmark output directory: %w", closeErr)
		}
	}()

	protectedInfo, err := protectedRoot.Stat(".")
	if err != nil {
		return fmt.Errorf("inspect protected baseline directory: %w", err)
	}
	outputParentInfo, err := outputRoot.Stat(".")
	if err != nil {
		return fmt.Errorf("inspect benchmark output directory: %w", err)
	}
	if err := validateBoundDirectories(outputParent, outputParentInfo, protectedBaselineDirectory, protectedInfo); err != nil {
		return err
	}

	if err := validateResultTarget(outputRoot, outputName); err != nil {
		return err
	}

	temporaryName, err := uniqueTemporaryName(outputName)
	if err != nil {
		return err
	}
	temporaryFile, err := outputRoot.OpenFile(temporaryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create benchmark output candidate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = outputRoot.Remove(temporaryName)
		}
		if closeErr := temporaryFile.Close(); closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close benchmark output candidate: %w", closeErr)
		}
	}()

	written, err := temporaryFile.Write(data)
	if err != nil {
		return fmt.Errorf("write benchmark output candidate: %w", err)
	}
	if written != len(data) {
		return fmt.Errorf("write benchmark output candidate: short write")
	}
	if err := temporaryFile.Sync(); err != nil {
		return fmt.Errorf("sync benchmark output candidate: %w", err)
	}

	if err := validateResultTarget(outputRoot, outputName); err != nil {
		return err
	}
	if err := validateBoundDirectories(outputParent, outputParentInfo, protectedBaselineDirectory, protectedInfo); err != nil {
		return err
	}
	if beforeRename != nil {
		if err := beforeRename(); err != nil {
			return fmt.Errorf("benchmark output test hook: %w", err)
		}
	}
	if err := outputRoot.Rename(temporaryName, outputName); err != nil {
		return fmt.Errorf("publish benchmark output candidate: %w", err)
	}

	writtenInfo, err := temporaryFile.Stat()
	if err != nil {
		return fmt.Errorf("inspect written benchmark output: %w", err)
	}
	publishedInfo, err := outputRoot.Lstat(outputName)
	if err != nil {
		return fmt.Errorf("inspect published benchmark output: %w", err)
	}
	if !publishedInfo.Mode().IsRegular() || !os.SameFile(writtenInfo, publishedInfo) {
		return fmt.Errorf("published benchmark output identity changed during commit")
	}

	if err := validateBoundDirectories(outputParent, outputParentInfo, protectedBaselineDirectory, protectedInfo); err != nil {
		_ = outputRoot.Remove(outputName)
		return err
	}

	committed = true
	return nil
}

func validateBoundDirectories(outputParent string, outputParentInfo os.FileInfo, protectedBaselineDirectory string, protectedInfo os.FileInfo) error {
	currentOutputParentInfo, err := os.Stat(outputParent)
	if err != nil || !os.SameFile(outputParentInfo, currentOutputParentInfo) {
		return fmt.Errorf("benchmark output parent changed after validation")
	}
	currentProtectedInfo, err := os.Stat(protectedBaselineDirectory)
	if err != nil || !os.SameFile(protectedInfo, currentProtectedInfo) {
		return fmt.Errorf("protected baseline directory changed after validation")
	}
	if os.SameFile(currentOutputParentInfo, currentProtectedInfo) {
		return fmt.Errorf("benchmark output directory resolves to the protected baseline directory")
	}
	return nil
}

func validateResultTarget(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect benchmark output target: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("benchmark output target must not be a symbolic link or reparse point")
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("benchmark output target must be a regular file")
	}
	return nil
}

func uniqueTemporaryName(outputName string) (string, error) {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate benchmark output candidate name: %w", err)
	}
	return "." + outputName + ".tmp-" + hex.EncodeToString(suffix[:]), nil
}

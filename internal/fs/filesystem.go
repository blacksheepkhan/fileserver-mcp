package fs

import (
	"errors"
	"io"
	"os"

	"github.com/blacksheepkhan/fileserver-mcp/internal/security"
)

var (
	// ErrFileTooLarge is returned when a file exceeds the configured read limit.
	ErrFileTooLarge = errors.New("file exceeds maximum allowed size")

	// ErrPathIsDirectory is returned when a file operation receives a directory path.
	ErrPathIsDirectory = errors.New("path is a directory")

	// ErrPathIsNotDirectory is returned when a directory operation receives a non-directory path.
	ErrPathIsNotDirectory = errors.New("path is not a directory")

	// ErrFileExists is returned when writing a file that already exists without overwrite permission.
	ErrFileExists = errors.New("file already exists")

	// ErrDirectoryNotEmpty is returned when deleting a non-empty directory without recursive deletion.
	ErrDirectoryNotEmpty = errors.New("directory is not empty")

	// ErrCopyDirectoryUnsupported is returned when attempting to copy a directory.
	ErrCopyDirectoryUnsupported = errors.New("copying directories is not supported")
)

// Entry represents a filesystem directory entry.
type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// Metadata represents filesystem metadata.
type Metadata struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// FileSystem defines filesystem operations used by MCP tools.
type FileSystem interface {
	List(path string) ([]Entry, error)
	Read(path string, maxBytes int64) ([]byte, error)
	Stat(path string) (Metadata, error)
	Exists(path string) (bool, error)
	Write(path string, content []byte, overwrite bool) error
	Mkdir(path string) error
	Delete(path string, recursive bool) error
	Move(source string, target string, overwrite bool) error
	Copy(source string, target string, overwrite bool) error
	Rename(source string, target string, overwrite bool) error
}

// LocalFileSystem implements FileSystem using the local operating system.
type LocalFileSystem struct {
	guard *security.PathGuard
}

// NewLocalFileSystem creates a new local filesystem.
func NewLocalFileSystem(root string) (*LocalFileSystem, error) {
	guard, err := security.NewPathGuard(root)
	if err != nil {
		return nil, err
	}

	return &LocalFileSystem{
		guard: guard,
	}, nil
}

// List lists directory entries.
func (f *LocalFileSystem) List(path string) ([]Entry, error) {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(safePath.String())
	if err != nil {
		return nil, err
	}

	result := make([]Entry, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		info, err := dirEntry.Info()
		if err != nil {
			return nil, err
		}

		result = append(result, Entry{
			Name:  dirEntry.Name(),
			IsDir: dirEntry.IsDir(),
			Size:  info.Size(),
		})
	}

	return result, nil
}

// Read reads a file up to maxBytes bytes.
func (f *LocalFileSystem) Read(path string, maxBytes int64) ([]byte, error) {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(safePath.String())
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, ErrPathIsDirectory
	}

	if maxBytes <= 0 {
		return nil, ErrFileTooLarge
	}

	if info.Size() > maxBytes {
		return nil, ErrFileTooLarge
	}

	return os.ReadFile(safePath.String())
}

// Stat returns filesystem metadata.
func (f *LocalFileSystem) Stat(path string) (Metadata, error) {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return Metadata{}, err
	}

	info, err := os.Stat(safePath.String())
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		Name:  info.Name(),
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}, nil
}

// Exists checks whether a path exists.
func (f *LocalFileSystem) Exists(path string) (bool, error) {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(safePath.String())
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

// Write writes a file. Existing files are only overwritten when overwrite is true.
func (f *LocalFileSystem) Write(path string, content []byte, overwrite bool) error {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(safePath.String())
	if err == nil {
		if info.IsDir() {
			return ErrPathIsDirectory
		}

		if !overwrite {
			return ErrFileExists
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(safePath.String(), flags, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}

		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	return err
}

// Mkdir creates a directory and any missing parent directories.
func (f *LocalFileSystem) Mkdir(path string) error {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(safePath.String(), 0o700)
}

// Delete deletes a file or directory.
func (f *LocalFileSystem) Delete(path string, recursive bool) error {
	safePath, err := f.guard.Resolve(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(safePath.String())
	if err != nil {
		return err
	}

	if info.IsDir() {
		if recursive {
			return os.RemoveAll(safePath.String())
		}

		entries, err := os.ReadDir(safePath.String())
		if err != nil {
			return err
		}

		if len(entries) > 0 {
			return ErrDirectoryNotEmpty
		}
	}

	return os.Remove(safePath.String())
}

// Move moves a file or directory. Existing targets are only overwritten when overwrite is true.
func (f *LocalFileSystem) Move(source string, target string, overwrite bool) error {
	sourcePath, err := f.guard.Resolve(source)
	if err != nil {
		return err
	}

	targetPath, err := f.guard.Resolve(target)
	if err != nil {
		return err
	}

	if err := ensureTargetPolicy(targetPath.String(), overwrite); err != nil {
		return err
	}

	if overwrite {
		if err := removeExistingTarget(targetPath.String()); err != nil {
			return err
		}
	}

	return os.Rename(sourcePath.String(), targetPath.String())
}

// Copy copies a file. Directory copy is intentionally unsupported.
func (f *LocalFileSystem) Copy(source string, target string, overwrite bool) error {
	sourcePath, err := f.guard.Resolve(source)
	if err != nil {
		return err
	}

	targetPath, err := f.guard.Resolve(target)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(sourcePath.String())
	if err != nil {
		return err
	}

	if sourceInfo.IsDir() {
		return ErrCopyDirectoryUnsupported
	}

	if err := ensureTargetPolicy(targetPath.String(), overwrite); err != nil {
		return err
	}

	sourceFile, err := os.Open(sourcePath.String())
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	targetFile, err := os.OpenFile(targetPath.String(), flags, sourceInfo.Mode().Perm())
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrFileExists
		}

		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	return err
}

// Rename renames a file or directory. It is a semantic alias for Move.
func (f *LocalFileSystem) Rename(source string, target string, overwrite bool) error {
	return f.Move(source, target, overwrite)
}

func ensureTargetPolicy(targetPath string, overwrite bool) error {
	info, err := os.Stat(targetPath)
	if err == nil {
		if !overwrite {
			return ErrFileExists
		}

		if info.IsDir() {
			return ErrPathIsDirectory
		}

		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func removeExistingTarget(targetPath string) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	if info.IsDir() {
		return ErrPathIsDirectory
	}

	return os.Remove(targetPath)
}

package security

import (
	"errors"
	"path/filepath"
	"strings"
)

// SafePath represents a filesystem path that has passed validation.
type SafePath struct {
	path string
}

// String returns the safe absolute path.
func (p SafePath) String() string {
	return p.path
}

// Base returns the last path element.
func (p SafePath) Base() string {
	return filepath.Base(p.path)
}

// Dir returns the directory component.
func (p SafePath) Dir() string {
	return filepath.Dir(p.path)
}

// PathGuard validates and resolves user-provided paths.
type PathGuard struct {
	root string
}

// NewPathGuard creates a new path guard.
func NewPathGuard(root string) *PathGuard {
	return &PathGuard{
		root: filepath.Clean(root),
	}
}

// Resolve validates and resolves a user path against the configured root.
func (g *PathGuard) Resolve(userPath string) (SafePath, error) {
	if strings.TrimSpace(userPath) == "" {
		userPath = "."
	}

	cleanedUserPath := filepath.Clean(userPath)

	if cleanedUserPath == ".." || strings.HasPrefix(cleanedUserPath, ".."+string(filepath.Separator)) {
		return SafePath{}, errors.New("path traversal detected")
	}

	if filepath.IsAbs(cleanedUserPath) {
		return SafePath{}, errors.New("absolute paths are not allowed")
	}

	rootAbs, err := filepath.Abs(g.root)
	if err != nil {
		return SafePath{}, err
	}

	resolvedAbs, err := filepath.Abs(filepath.Join(rootAbs, cleanedUserPath))
	if err != nil {
		return SafePath{}, err
	}

	relative, err := filepath.Rel(rootAbs, resolvedAbs)
	if err != nil {
		return SafePath{}, err
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return SafePath{}, errors.New("access outside root denied")
	}

	return SafePath{
		path: resolvedAbs,
	}, nil
}

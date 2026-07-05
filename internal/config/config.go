package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

const (
	envRootPath         = "MCP_ROOT"
	envReadOnly         = "MCP_READ_ONLY"
	envMaxFileSize      = "MCP_MAX_FILE_SIZE"
	envAllowHiddenFiles = "MCP_ALLOW_HIDDEN_FILES"
	envAllowUNCPaths    = "MCP_ALLOW_UNC_PATHS"
	envFollowSymlinks   = "MCP_FOLLOW_SYMLINKS"
	envServerDebug      = "MCP_DEBUG"

	defaultRootPath    = "."
	defaultMaxFileSize = int64(10 * 1024 * 1024) // 10 MiB
	defaultServerName  = "fileserver-mcp"
	defaultVersion     = "0.1.0-dev"
)

// Config contains the complete application configuration.
type Config struct {
	filesystem FilesystemConfig
	security   SecurityConfig
	server     ServerConfig
}

// FilesystemConfig contains filesystem-related configuration.
type FilesystemConfig struct {
	rootPath    string
	readOnly    bool
	maxFileSize int64
}

// SecurityConfig contains security-related configuration.
type SecurityConfig struct {
	allowHiddenFiles bool
	allowUNCPaths    bool
	followSymlinks   bool
}

// ServerConfig contains server-related configuration.
type ServerConfig struct {
	name    string
	version string
	debug   bool
}

// DefaultConfig returns the default application configuration.
func DefaultConfig() Config {
	return Config{
		filesystem: FilesystemConfig{
			rootPath:    defaultRootPath,
			readOnly:    false,
			maxFileSize: defaultMaxFileSize,
		},
		security: SecurityConfig{
			allowHiddenFiles: false,
			allowUNCPaths:    false,
			followSymlinks:   false,
		},
		server: ServerConfig{
			name:    defaultServerName,
			version: defaultVersion,
			debug:   false,
		},
	}
}

// LoadFromEnvironment loads configuration from environment variables.
func LoadFromEnvironment() (Config, error) {
	cfg := DefaultConfig()

	if value := os.Getenv(envRootPath); value != "" {
		cfg.filesystem.rootPath = value
	}

	if value := os.Getenv(envReadOnly); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_READ_ONLY value")
		}
		cfg.filesystem.readOnly = parsed
	}

	if value := os.Getenv(envMaxFileSize); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return Config{}, errors.New("invalid MCP_MAX_FILE_SIZE value")
		}
		cfg.filesystem.maxFileSize = parsed
	}

	if value := os.Getenv(envAllowHiddenFiles); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_ALLOW_HIDDEN_FILES value")
		}
		cfg.security.allowHiddenFiles = parsed
	}

	if value := os.Getenv(envAllowUNCPaths); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_ALLOW_UNC_PATHS value")
		}
		cfg.security.allowUNCPaths = parsed
	}

	if value := os.Getenv(envFollowSymlinks); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_FOLLOW_SYMLINKS value")
		}
		cfg.security.followSymlinks = parsed
	}

	if value := os.Getenv(envServerDebug); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("invalid MCP_DEBUG value")
		}
		cfg.server.debug = parsed
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate validates the complete configuration.
func (c Config) Validate() error {
	if c.filesystem.rootPath == "" {
		return errors.New("filesystem root path must not be empty")
	}

	if c.filesystem.maxFileSize <= 0 {
		return errors.New("maximum file size must be greater than zero")
	}

	if filepath.IsAbs(c.filesystem.rootPath) {
		return nil
	}

	cleaned := filepath.Clean(c.filesystem.rootPath)
	if cleaned == "." {
		return nil
	}

	return nil
}

// Filesystem returns the filesystem configuration.
func (c Config) Filesystem() FilesystemConfig {
	return c.filesystem
}

// Security returns the security configuration.
func (c Config) Security() SecurityConfig {
	return c.security
}

// Server returns the server configuration.
func (c Config) Server() ServerConfig {
	return c.server
}

// RootPath returns the configured filesystem root path.
func (c FilesystemConfig) RootPath() string {
	return c.rootPath
}

// ReadOnly returns whether filesystem writes are disabled.
func (c FilesystemConfig) ReadOnly() bool {
	return c.readOnly
}

// MaxFileSize returns the maximum allowed file size in bytes.
func (c FilesystemConfig) MaxFileSize() int64 {
	return c.maxFileSize
}

// AllowHiddenFiles returns whether hidden files may be accessed.
func (c SecurityConfig) AllowHiddenFiles() bool {
	return c.allowHiddenFiles
}

// AllowUNCPaths returns whether UNC paths may be used on Windows.
func (c SecurityConfig) AllowUNCPaths() bool {
	return c.allowUNCPaths
}

// FollowSymlinks returns whether symbolic links may be followed.
func (c SecurityConfig) FollowSymlinks() bool {
	return c.followSymlinks
}

// Name returns the server name.
func (c ServerConfig) Name() string {
	return c.name
}

// Version returns the server version.
func (c ServerConfig) Version() string {
	return c.version
}

// Debug returns whether debug mode is enabled.
func (c ServerConfig) Debug() bool {
	return c.debug
}

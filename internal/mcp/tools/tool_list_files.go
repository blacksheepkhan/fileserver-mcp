package tools

import (
	"context"
	"encoding/json"

	"github.com/blacksheepkhan/fileserver-mcp/internal/fs"
	"github.com/blacksheepkhan/fileserver-mcp/internal/protocol"
)

// ListFilesTool lists files and directories.
type ListFilesTool struct {
	filesystem fs.FileSystem
}

// NewListFilesTool creates a new list_files tool.
func NewListFilesTool(filesystem fs.FileSystem) *ListFilesTool {
	return &ListFilesTool{
		filesystem: filesystem,
	}
}

// Name returns the tool name.
func (t *ListFilesTool) Name() string {
	return "list_files"
}

// Description returns the tool description.
func (t *ListFilesTool) Description() string {
	return "Lists files and directories in a given path."
}

// InputSchema returns the JSON schema for the tool input.
func (t *ListFilesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type": "string",
			},
		},
		"required": []string{"path"},
	}
}

// Execute executes the list_files tool.
func (t *ListFilesTool) Execute(_ context.Context, arguments json.RawMessage) (any, *protocol.Error) {
	var input struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(arguments, &input); err != nil {
		return nil, &protocol.Error{
			Code:    protocol.ErrInvalidParams,
			Message: "invalid arguments",
		}
	}

	entries, err := t.filesystem.List(input.Path)
	if err != nil {
		return nil, &protocol.Error{
			Code:    protocol.ErrInternalError,
			Message: err.Error(),
		}
	}

	return map[string]any{
		"path":  input.Path,
		"items": entries,
	}, nil
}

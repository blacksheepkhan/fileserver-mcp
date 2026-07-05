package tools

import (
	"encoding/json"

	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/handlers"
	"github.com/blacksheepkhan/fileserver-mcp/internal/protocol"
)

// CallHandler handles the MCP tools/call method.
type CallHandler struct {
	registry *Registry
}

// NewCallHandler creates a new tools/call handler.
func NewCallHandler(registry *Registry) *CallHandler {
	return &CallHandler{
		registry: registry,
	}
}

// Method returns the MCP method name.
func (h *CallHandler) Method() string {
	return "tools/call"
}

// Handle handles the tools/call request.
func (h *CallHandler) Handle(ctx handlers.Context, params json.RawMessage) (any, *protocol.Error) {
	var input struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(params, &input); err != nil {
		return nil, &protocol.Error{
			Code:    protocol.ErrInvalidParams,
			Message: "invalid params",
		}
	}

	tool, ok := h.registry.Get(input.Name)
	if !ok {
		return nil, &protocol.Error{
			Code:    protocol.ErrMethodNotFound,
			Message: "tool not found: " + input.Name,
		}
	}

	return tool.Execute(ctx.Context, input.Arguments)
}

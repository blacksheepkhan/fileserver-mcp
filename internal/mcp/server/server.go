package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/handlers"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/router"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/transport"
	"github.com/blacksheepkhan/fileserver-mcp/internal/protocol"
)

// Server is the MCP server runtime.
type Server struct {
	transport *transport.Transport
	router    *router.Router
}

// New creates a new MCP server.
func New(in io.Reader, out io.Writer, router *router.Router) *Server {
	return &Server{
		transport: transport.New(in, out),
		router:    router,
	}
}

// Run starts the MCP request loop.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		message, err := s.transport.ReadMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}

		var request protocol.Request
		if err := json.Unmarshal(message, &request); err != nil {
			if writeErr := s.writeError(nil, protocol.ErrParseError, "invalid json"); writeErr != nil {
				return writeErr
			}
			continue
		}

		result, rpcErr := s.router.Dispatch(
			request.Method,
			handlers.Context{Context: ctx},
			request.Params,
		)

		if rpcErr != nil {
			if writeErr := s.writeError(request.ID, rpcErr.Code, rpcErr.Message); writeErr != nil {
				return writeErr
			}
			continue
		}

		response := protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      request.ID,
			Result:  result,
		}

		if err := s.transport.WriteMessage(response); err != nil {
			return err
		}
	}
}

func (s *Server) writeError(id json.RawMessage, code int, message string) error {
	response := protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      id,
		Error: &protocol.Error{
			Code:    code,
			Message: message,
		},
	}

	return s.transport.WriteMessage(response)
}

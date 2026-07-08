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

		request, validationErr := validateRequestMessage(message)
		if validationErr != nil {
			if writeErr := s.writeError(validationErr.id, validationErr.code, validationErr.message); writeErr != nil {
				return writeErr
			}
			continue
		}

		if request.notification {
			continue
		}

		response := s.handleRequest(ctx, request)
		if err := s.transport.WriteMessage(response); err != nil {
			return err
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, request validatedRequest) (response protocol.Response) {
	defer func() {
		if recovered := recover(); recovered != nil {
			response = protocol.Response{
				JSONRPC: protocol.JSONRPCVersion,
				ID:      request.id,
				Error: &protocol.Error{
					Code:    protocol.ErrInternalError,
					Message: "internal error",
				},
			}
		}
	}()

	result, rpcErr := s.router.Dispatch(
		request.method,
		handlers.Context{Context: ctx},
		request.params,
	)

	if rpcErr != nil {
		return protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      request.id,
			Error: &protocol.Error{
				Code:    rpcErr.Code,
				Message: rpcErr.Message,
			},
		}
	}

	return protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      request.id,
		Result:  result,
	}
}

func (s *Server) writeError(id json.RawMessage, code int, message string) error {
	if len(id) == 0 {
		id = nullID()
	}

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

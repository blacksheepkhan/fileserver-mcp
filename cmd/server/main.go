package main

import (
	"context"
	"fmt"
	"os"

	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/initialize"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/router"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/server"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/tools"
)

const (
	serverName    = "fileserver-mcp"
	serverVersion = "0.1.0-dev"
)

func main() {
	if err := run(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fileserver-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	toolRegistry := tools.NewRegistry()

	mcpRouter := router.New()
	mcpRouter.Register(initialize.NewHandler(serverName, serverVersion))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry))

	mcpServer := server.New(os.Stdin, os.Stdout, mcpRouter)

	return mcpServer.Run(ctx)
}

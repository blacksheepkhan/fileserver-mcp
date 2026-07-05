package main

import (
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/initialize"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/router"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/tools"
)

func createRouter(serverName string, serverVersion string, toolRegistry *tools.Registry) *router.Router {
	mcpRouter := router.New()
	mcpRouter.Register(initialize.NewHandler(serverName, serverVersion))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry))

	return mcpRouter
}

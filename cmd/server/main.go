package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/blacksheepkhan/fileserver-mcp/internal/config"
	"github.com/blacksheepkhan/fileserver-mcp/internal/fs"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/initialize"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/router"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/server"
	"github.com/blacksheepkhan/fileserver-mcp/internal/mcp/tools"
	"github.com/blacksheepkhan/fileserver-mcp/internal/version"
)

var errInvalidCLIArguments = errors.New("invalid CLI arguments")

func main() {
	exitCode, err := runCLI(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fileserver-mcp: %v\n", err)

		if errors.Is(err, errInvalidCLIArguments) {
			os.Exit(2)
		}

		os.Exit(exitCode)
	}

	os.Exit(exitCode)
}

func runCLI(args []string) (int, error) {
	switch len(args) {
	case 0:
		if err := run(context.Background()); err != nil {
			return 1, err
		}

		return 0, nil
	case 1:
		switch args[0] {
		case "--version":
			fmt.Println(version.Get().String())
			return 0, nil
		case "--help", "-h":
			fmt.Print(helpText())
			return 0, nil
		default:
			return 2, fmt.Errorf("%w: unknown argument: %s\nUse --help for usage.", errInvalidCLIArguments, args[0])
		}
	default:
		return 2, fmt.Errorf("%w: too many arguments\nUse --help for usage.", errInvalidCLIArguments)
	}
}

func helpText() string {
	return `fileserver-mcp

Usage:
  fileserver-mcp
  fileserver-mcp --version
  fileserver-mcp --help

Environment:
  MCP_ROOT    Root directory exposed to MCP clients
`
}

func run(ctx context.Context) error {
	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		return err
	}

	filesystem, err := fs.NewLocalFileSystem(cfg.Filesystem().RootPath())
	if err != nil {
		return err
	}

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewListFilesTool(filesystem))
	toolRegistry.Register(tools.NewReadFileTool(filesystem, cfg.Filesystem().MaxFileSize()))
	toolRegistry.Register(tools.NewStatPathTool(filesystem))
	toolRegistry.Register(tools.NewExistsPathTool(filesystem))
	toolRegistry.Register(tools.NewWriteFileTool(filesystem))
	toolRegistry.Register(tools.NewMkdirTool(filesystem))
	toolRegistry.Register(tools.NewDeletePathTool(filesystem))
	toolRegistry.Register(tools.NewMovePathTool(filesystem))
	toolRegistry.Register(tools.NewCopyPathTool(filesystem))
	toolRegistry.Register(tools.NewRenamePathTool(filesystem))

	mcpRouter := router.New()
	mcpRouter.Register(initialize.NewHandler(cfg.Server().Name(), cfg.Server().Version()))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry))

	mcpServer := server.New(os.Stdin, os.Stdout, mcpRouter)

	return mcpServer.Run(ctx)
}

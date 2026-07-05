# fileserver-mcp

A high-performance, cross-platform Model Context Protocol (MCP) server written in Go that provides secure, low-latency filesystem operations for AI assistants.

`fileserver-mcp` is designed as a modern replacement for existing filesystem-oriented MCP servers such as Desktop Commander and Filesystem MCP. Its primary goals are maximum performance, minimal resource consumption, predictable behavior and enterprise-grade reliability.

The server communicates exclusively over **STDIO**, making it directly compatible with Claude Desktop, Codex CLI and any MCP-compatible client.

---

# Goals

- Extremely fast filesystem operations
- Low memory and CPU usage
- Single self-contained binary
- No runtime dependencies
- Cross-platform (Windows and Linux)
- Enterprise-ready architecture
- Secure-by-default implementation
- Comprehensive automated test suite
- Clean, extensible codebase

---

# Planned Features

## File Operations

- Read files
- Write files
- Create files
- Delete files
- Copy files
- Move files
- Rename files
- Append to files
- Retrieve file metadata

## Directory Operations

- List directories
- Create directories
- Delete directories
- Recursive directory traversal
- Directory statistics

## Search

- Fast filename search
- Recursive search
- Content search
- Configurable filters
- Ignore patterns

## Security

- Configurable allowed root directories
- Path traversal protection
- Symbolic link validation
- Permission verification
- Optional read-only mode

## Performance

- Optimized buffered I/O
- Streaming for large files
- Efficient directory traversal
- Low allocation design
- Concurrent request handling where appropriate

## MCP Features

- Full Model Context Protocol compatibility
- STDIO transport
- JSON-RPC 2.0
- Tool discovery
- Structured error responses
- Progress notifications (planned)

---

# Project Structure

```
cmd/
    server/             Application entry point

internal/
    config/             Configuration
    fs/                 Filesystem implementation
    mcp/                MCP protocol implementation
    security/           Security layer
    search/             Search engine
    logging/            Logging

pkg/
    protocol/           Shared protocol definitions

tests/
    integration/
    unit/

docs/
    architecture/
    protocol/
```

---

# Building

## Requirements

- Go 1.26 or newer

Clone the repository:

```bash
git clone https://github.com/blacksheepkhan/fileserver-mcp.git
cd fileserver-mcp
```

Build:

```bash
go build ./cmd/server
```

---

# Usage

This project is an MCP server and communicates over STDIO.

Example Claude Desktop configuration will be provided after the first stable release.

---

# Development

The project follows modern Go best practices.

- `go test ./...`
- `go fmt ./...`
- `go vet ./...`

Every feature should include:

- Unit tests
- Integration tests
- Documentation
- Error handling
- Cross-platform compatibility

---

# Roadmap

- [ ] MCP protocol implementation
- [ ] Filesystem abstraction
- [ ] Read file
- [ ] Write file
- [ ] Directory listing
- [ ] File search
- [ ] Security layer
- [ ] Logging
- [ ] Configuration file
- [ ] Integration tests
- [ ] Performance benchmarks
- [ ] GitHub Actions CI
- [ ] First stable release (v1.0.0)

---

# License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0).

See the LICENSE file for details.
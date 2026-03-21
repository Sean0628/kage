package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/shoito/kage/internal/config"
)

// NewServer creates a new MCP server with all kage coordination tools registered.
func NewServer(cfg *config.Config) *server.MCPServer {
	s := server.NewMCPServer(
		"kage",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	registerTools(s, cfg)
	return s
}

// Serve starts the MCP server on stdio.
func Serve(cfg *config.Config) error {
	s := NewServer(cfg)
	return server.ServeStdio(s)
}

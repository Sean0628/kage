package cmd

import (
	"fmt"

	kageMcp "github.com/Sean0628/kage/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for agent coordination",
	Long:  "Starts an MCP server over stdio that exposes kage coordination tools to Claude Code.",
	RunE:  runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	return kageMcp.Serve(cfg)
}

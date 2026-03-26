package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Sean0628/kage/internal/config"
	"github.com/Sean0628/kage/internal/tmux"
	"github.com/spf13/cobra"
)

var cfgPath string

var rootCmd = &cobra.Command{
	Use:   "kage",
	Short: "kage (影) — manage multiple AI coding agent worktree sessions via tmux",
	RunE:  runRoot,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path (default: ~/.config/kage/config.yaml)")
}

func loadConfig() (*config.Config, error) {
	path := cfgPath
	if path == "" {
		path = config.DefaultConfigPath()
	}
	return config.Load(path)
}

func runRoot(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	insideTmux := tmux.InsideTmux()
	hasSession := tmux.HasSession()

	switch {
	case !insideTmux && !hasSession:
		// Create session, send `kage dash` to window 0, then attach
		if err := tmux.NewSession(cfg.EffectiveWorkspace()); err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		// Register Ctrl+b K keybinding to jump back to dashboard
		registerDashboardKeybinding()
		// Send the dash command to the first window
		if err := tmux.SendKeys(tmux.SessionName+":dashboard", "kage dash"); err != nil {
			return fmt.Errorf("sending dash command: %w", err)
		}
		// Split dashboard and launch coordinator Claude Code (if enabled)
		if cfg.Coordinator {
			setupCoordinatorPane(cfg)
		}
		// Attach (replaces this process)
		return tmux.AttachSession()

	case !insideTmux && hasSession:
		// Just attach
		return tmux.AttachSession()

	case insideTmux && !hasSession:
		// Create session and switch to it
		if err := tmux.NewSession(cfg.EffectiveWorkspace()); err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		registerDashboardKeybinding()
		if err := tmux.SendKeys(tmux.SessionName+":dashboard", "kage dash"); err != nil {
			return fmt.Errorf("sending dash command: %w", err)
		}
		// Split dashboard and launch coordinator Claude Code (if enabled)
		if cfg.Coordinator {
			setupCoordinatorPane(cfg)
		}
		return tmux.SwitchClient()

	case insideTmux && hasSession:
		// Switch to kage session and select dashboard window
		if err := tmux.SwitchClient(); err != nil {
			return fmt.Errorf("switching to kage session: %w", err)
		}
		return tmux.SelectWindow("dashboard")
	}
	return nil
}

func registerDashboardKeybinding() {
	// Bind Ctrl+b K to jump to the kage dashboard window
	tmux.RunSilent("bind-key", "-T", "prefix", "K",
		"switch-client", "-t", tmux.SessionName, "\\;",
		"select-window", "-t", tmux.SessionName+":dashboard")
}

// setupCoordinatorPane splits the dashboard window and launches the coordinator
// agent in the right pane. Supports claude (default) and codex with automatic
// MCP wiring. Other agents are launched as-is.
func setupCoordinatorPane(cfg *config.Config) {
	agentCmd := cfg.CoordinatorCmd
	if agentCmd == "" {
		agentCmd = "claude"
	}

	coordinatorCmd, err := buildCoordinatorCmd(agentCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kage: failed to set up coordinator: %v\n", err)
		return
	}

	// Split dashboard window: left = TUI, right = coordinator
	// Give the dashboard most of the window so long branch/worktree names remain readable.
	// Use SplitWindowWithCmd to run the agent directly, avoiding the race condition
	// where SendKeys fires before the shell in the new pane is ready.
	// Detached=true keeps focus on the dashboard (pane 0).
	target := tmux.SessionName + ":dashboard"
	if err := tmux.SplitWindowWithCmd(target, true, "35", cfg.EffectiveWorkspace(), coordinatorCmd, true); err != nil {
		fmt.Fprintf(os.Stderr, "kage: failed to split dashboard for coordinator: %v\n", err)
		return
	}
}

// buildCoordinatorCmd returns the shell command to launch the coordinator agent
// with MCP wired appropriately for the agent type.
func buildCoordinatorCmd(agentCmd string) (string, error) {
	kageBin := resolveKageBin()

	switch agentCmd {
	case "claude":
		mcpConfigPath, err := ensureClaudeMCPConfig(kageBin)
		if err != nil {
			return "", fmt.Errorf("writing claude MCP config: %w", err)
		}
		return fmt.Sprintf("claude --mcp-config %s", mcpConfigPath), nil

	case "codex":
		// Register kage MCP server with codex, then launch codex
		if err := registerCodexMCP(kageBin); err != nil {
			return "", fmt.Errorf("registering codex MCP server: %w", err)
		}
		return "codex", nil

	default:
		// Custom command — launch as-is
		return agentCmd, nil
	}
}

// resolveKageBin returns the absolute path to the kage binary.
func resolveKageBin() string {
	kageBin, err := exec.LookPath("kage")
	if err != nil {
		return "kage"
	}
	return kageBin
}

// registerCodexMCP registers the kage MCP server with codex via `codex mcp add`.
// It removes any existing registration first to ensure a clean state.
func registerCodexMCP(kageBin string) error {
	// Remove existing registration (ignore errors if it doesn't exist)
	exec.Command("codex", "mcp", "remove", "kage").Run()

	// Register kage MCP server
	cmd := exec.Command("codex", "mcp", "add", "kage", "--", kageBin, "mcp")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// mcpConfig is the structure for Claude Code's MCP configuration file.
type mcpConfig struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// ensureClaudeMCPConfig writes the kage MCP server config to ~/.config/kage/mcp.json
// and returns the path. This file format is specific to Claude Code's --mcp-config flag.
func ensureClaudeMCPConfig(kageBin string) (string, error) {
	cfg := mcpConfig{
		MCPServers: map[string]mcpServerEntry{
			"kage": {
				Command: kageBin,
				Args:    []string{"mcp"},
			},
		},
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(home, ".config", "kage")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, "mcp.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return "", err
	}

	return configPath, nil
}

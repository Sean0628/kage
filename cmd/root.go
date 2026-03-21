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
			setupCoordinatorPane(cfg.EffectiveWorkspace())
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
			setupCoordinatorPane(cfg.EffectiveWorkspace())
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

// setupCoordinatorPane splits the dashboard window and launches Claude Code
// with the kage MCP server configured in the right pane.
func setupCoordinatorPane(workDir string) {
	mcpConfigPath, err := ensureMCPConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kage: failed to write MCP config: %v\n", err)
		return
	}

	// Split dashboard window: left = TUI, right = coordinator
	// Use SplitWindowWithCmd to run claude directly, avoiding the race condition
	// where SendKeys fires before the shell in the new pane is ready.
	target := tmux.SessionName + ":dashboard"
	claudeCmd := fmt.Sprintf("claude --mcp-config %s", mcpConfigPath)
	if err := tmux.SplitWindowWithCmd(target, false, "50", workDir, claudeCmd); err != nil {
		fmt.Fprintf(os.Stderr, "kage: failed to split dashboard for coordinator: %v\n", err)
		return
	}

	// Select the left pane (TUI) as active
	tmux.RunSilent("select-pane", "-t", target+".0")
}

// mcpConfig is the structure for Claude Code's MCP configuration file.
type mcpConfig struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// ensureMCPConfig writes the kage MCP server config to ~/.config/kage/mcp.json
// and returns the path.
func ensureMCPConfig() (string, error) {
	kageBin, err := exec.LookPath("kage")
	if err != nil {
		// Fall back to just "kage" if not found in PATH
		kageBin = "kage"
	}

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

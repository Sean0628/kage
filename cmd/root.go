package cmd

import (
	"fmt"
	"os"

	"github.com/shoito/kage/internal/config"
	"github.com/shoito/kage/internal/tmux"
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
	// Ensure config is loadable
	if _, err := loadConfig(); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	insideTmux := tmux.InsideTmux()
	hasSession := tmux.HasSession()

	switch {
	case !insideTmux && !hasSession:
		// Create session, send `kage dash` to window 0, then attach
		if err := tmux.NewSession(); err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		// Register Ctrl+b K keybinding to jump back to dashboard
		registerDashboardKeybinding()
		// Send the dash command to the first window
		if err := tmux.SendKeys(tmux.SessionName+":dashboard", "kage dash"); err != nil {
			return fmt.Errorf("sending dash command: %w", err)
		}
		// Attach (replaces this process)
		return tmux.AttachSession()

	case !insideTmux && hasSession:
		// Just attach
		return tmux.AttachSession()

	case insideTmux && !hasSession:
		// Create session and switch to it
		if err := tmux.NewSession(); err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		registerDashboardKeybinding()
		if err := tmux.SendKeys(tmux.SessionName+":dashboard", "kage dash"); err != nil {
			return fmt.Errorf("sending dash command: %w", err)
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

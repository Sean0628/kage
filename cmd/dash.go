package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Sean0628/kage/internal/state"
	"github.com/Sean0628/kage/internal/tui"
	"github.com/spf13/cobra"
)

var dashCmd = &cobra.Command{
	Use:   "dash",
	Short: "Launch the kage dashboard TUI",
	RunE:  runDash,
}

func init() {
	rootCmd.AddCommand(dashCmd)
}

func runDash(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	st, err := state.Load(state.DefaultStatePath())
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	model := tui.New(cfg, st)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
	return nil
}

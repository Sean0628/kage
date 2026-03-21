package tui

import "strings"

// renderGuide returns the help guide content.
func renderGuide() string {
	var b strings.Builder

	b.WriteString(guideHeaderStyle.Render("  kage 影 — Quick Guide"))
	b.WriteString("\n\n")

	b.WriteString(guideHeaderStyle.Render("  NAVIGATION"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    ↑/k, ↓/j    Move cursor up/down"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    Enter        Jump to selected feature (opens tmux window)"))
	b.WriteString("\n\n")

	b.WriteString(guideHeaderStyle.Render("  ACTIONS"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    n            Create new feature branch + worktree"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    a            Attach an existing remote branch"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    d            Delete feature worktree"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    r            Refresh project state"))
	b.WriteString("\n\n")

	b.WriteString(guideHeaderStyle.Render("  GENERAL"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    h            Show this guide"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    q / Ctrl+C   Quit dashboard"))
	b.WriteString("\n\n")

	b.WriteString(guideHeaderStyle.Render("  WORKFLOW"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    • kage manages tmux sessions with multiple worktrees"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    • Each feature gets its own worktree + tmux window"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    • Pane layout is defined in ~/.config/kage/config.yaml"))
	b.WriteString("\n")
	b.WriteString(normalStyle.Render("    • Use Ctrl+b K to return to the dashboard from any window"))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("  Press any key to close"))
	b.WriteString("\n")

	return b.String()
}

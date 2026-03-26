package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	projectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	liveMarker = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			SetString("●")

	inactiveMarker = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			SetString("○")

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	columnHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("244"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Align(lipgloss.Right)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	guideHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	statusIdleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	statusWaitingInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("81"))

	statusWaitingPermissionStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("203"))
)

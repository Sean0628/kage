package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shoito/kage/internal/config"
	"github.com/shoito/kage/internal/project"
	"github.com/shoito/kage/internal/worktree"
)

// Mode represents the current TUI mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeNewBranch
	ModeConfirmDelete
	ModeAttachBranch
	ModeHelp
)

// Model is the bubbletea model for the dashboard.
type Model struct {
	cfg    *config.Config
	states []project.ProjectState
	items  []listItem
	cursor int
	mode   Mode
	err    string

	// New branch input
	textInput    textinput.Model
	newProjectID int // which project the new branch is for

	// Delete confirmation
	deleteTarget *listItem

	// Attach branch picker
	attachProjectID int
	attachBranches  []string // filtered list
	attachAll       []string // all available branches
	attachCursor    int
	attachInput     textinput.Model
	attachLoading   bool

	width  int
	height int
}

type tickMsg time.Time
type refreshMsg []project.ProjectState
type fetchBranchesMsg struct {
	branches []string
	err      error
}

// New creates a new dashboard model.
func New(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "feature/branch-name"
	ti.CharLimit = 100
	ti.Width = 40

	ai := textinput.New()
	ai.Placeholder = "type to filter..."
	ai.CharLimit = 100
	ai.Width = 40

	m := Model{
		cfg:         cfg,
		textInput:   ti,
		attachInput: ai,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(refresh(m.cfg), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(refresh(m.cfg), tick())

	case refreshMsg:
		m.states = []project.ProjectState(msg)
		m.items = flattenItems(m.states)
		// Clamp cursor to a selectable (non-header) item
		if m.cursor >= len(m.items) {
			m.cursor = max(0, len(m.items)-1)
		}
		if m.cursor < len(m.items) && m.items[m.cursor].isHeader {
			m.cursor = m.nextSelectable()
		}
		return m, nil

	case fetchBranchesMsg:
		m.attachLoading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.mode = ModeNormal
			return m, nil
		}
		m.attachAll = msg.branches
		m.attachBranches = msg.branches
		m.attachCursor = 0
		return m, nil

	case tea.KeyMsg:
		m.err = ""
		switch m.mode {
		case ModeNewBranch:
			return m.updateNewBranch(msg)
		case ModeConfirmDelete:
			return m.updateConfirmDelete(msg)
		case ModeAttachBranch:
			return m.updateAttachBranch(msg)
		case ModeHelp:
			m.mode = ModeNormal
			return m, nil
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "q" || msg.String() == "ctrl+c":
		return m, tea.Quit

	case msg.String() == "up" || msg.String() == "k":
		m.cursor = m.prevSelectable()

	case msg.String() == "down" || msg.String() == "j":
		m.cursor = m.nextSelectable()

	case msg.String() == "enter":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if !item.isHeader && item.feature != nil {
				ps := m.states[item.projectIdx]
				// LaunchFeature handles both existing windows (with pane count
				// validation) and new window creation with layout setup.
				err := project.LaunchFeature(m.cfg, ps.Config, item.feature.Branch, false)
				if err != nil {
					m.err = err.Error()
				}
			}
		}

	case msg.String() == "n":
		if len(m.items) == 0 {
			return m, nil
		}
		// Find the project for the current cursor position
		item := m.items[m.cursor]
		m.newProjectID = item.projectIdx
		m.mode = ModeNewBranch
		m.textInput.Reset()
		m.textInput.Focus()
		return m, textinput.Blink

	case msg.String() == "d":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if !item.isHeader && item.feature != nil && !item.feature.IsMain {
				m.mode = ModeConfirmDelete
				m.deleteTarget = &item
			}
		}

	case msg.String() == "a":
		if len(m.items) == 0 {
			return m, nil
		}
		item := m.items[m.cursor]
		m.attachProjectID = item.projectIdx
		m.mode = ModeAttachBranch
		m.attachLoading = true
		m.attachInput.Reset()
		m.attachInput.Focus()
		m.attachBranches = nil
		m.attachAll = nil
		m.attachCursor = 0
		ps := m.states[m.attachProjectID]
		return m, tea.Batch(textinput.Blink, fetchBranches(ps.Config.Path, ps.Features))

	case msg.String() == "r":
		return m, refresh(m.cfg)

	case msg.String() == "h":
		m.mode = ModeHelp
	}

	return m, nil
}

func (m Model) updateNewBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		branch := m.textInput.Value()
		if branch == "" {
			m.mode = ModeNormal
			return m, nil
		}
		ps := m.states[m.newProjectID]
		err := project.LaunchFeature(m.cfg, ps.Config, branch, true)
		if err != nil {
			m.err = err.Error()
		}
		m.mode = ModeNormal
		return m, refresh(m.cfg)

	case "esc", "ctrl+c":
		m.mode = ModeNormal
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if m.deleteTarget != nil && m.deleteTarget.feature != nil {
			ps := m.states[m.deleteTarget.projectIdx]
			err := project.DeleteFeature(ps.Config, m.deleteTarget.feature.Branch, false)
			if err != nil {
				// Try force delete
				err = project.DeleteFeature(ps.Config, m.deleteTarget.feature.Branch, true)
				if err != nil {
					m.err = err.Error()
				}
			}
		}
		m.mode = ModeNormal
		m.deleteTarget = nil
		return m, refresh(m.cfg)

	default:
		m.mode = ModeNormal
		m.deleteTarget = nil
		return m, nil
	}
}

func (m Model) updateAttachBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if len(m.attachBranches) > 0 && m.attachCursor < len(m.attachBranches) {
			branch := m.attachBranches[m.attachCursor]
			ps := m.states[m.attachProjectID]
			err := project.LaunchFeature(m.cfg, ps.Config, branch, false)
			if err != nil {
				m.err = err.Error()
			}
			m.mode = ModeNormal
			return m, refresh(m.cfg)
		}
		m.mode = ModeNormal
		return m, nil

	case "esc", "ctrl+c":
		m.mode = ModeNormal
		return m, nil

	case "up", "ctrl+p":
		if m.attachCursor > 0 {
			m.attachCursor--
		}
		return m, nil

	case "down", "ctrl+n":
		if m.attachCursor < len(m.attachBranches)-1 {
			m.attachCursor++
		}
		return m, nil

	default:
		var cmd tea.Cmd
		m.attachInput, cmd = m.attachInput.Update(msg)
		// Filter branches by input
		filter := strings.ToLower(m.attachInput.Value())
		if filter == "" {
			m.attachBranches = m.attachAll
		} else {
			var filtered []string
			for _, b := range m.attachAll {
				if strings.Contains(strings.ToLower(b), filter) {
					filtered = append(filtered, b)
				}
			}
			m.attachBranches = filtered
		}
		m.attachCursor = 0
		return m, cmd
	}
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("kage 影"))
	b.WriteString("\n")

	if len(m.items) == 0 {
		b.WriteString(dimStyle.Render("  No projects configured. Edit ~/.config/kage/config.yaml"))
		b.WriteString("\n")
	}

	for i, item := range m.items {
		selected := i == m.cursor
		b.WriteString(renderItem(item, selected))
		b.WriteString("\n")
	}

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  Error: " + m.err))
		b.WriteString("\n")
	}

	switch m.mode {
	case ModeNewBranch:
		projName := ""
		if m.newProjectID < len(m.states) {
			projName = m.states[m.newProjectID].Config.Name
		}
		b.WriteString("\n")
		b.WriteString(promptStyle.Render(fmt.Sprintf("  New branch for %s: ", projName)))
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
	case ModeConfirmDelete:
		if m.deleteTarget != nil && m.deleteTarget.feature != nil {
			b.WriteString("\n")
			b.WriteString(promptStyle.Render(
				fmt.Sprintf("  Delete worktree for '%s'? (y/n) ", m.deleteTarget.feature.Branch),
			))
			b.WriteString("\n")
		}
	case ModeAttachBranch:
		projName := ""
		if m.attachProjectID < len(m.states) {
			projName = m.states[m.attachProjectID].Config.Name
		}
		b.WriteString("\n")
		if m.attachLoading {
			b.WriteString(promptStyle.Render(fmt.Sprintf("  Fetching branches for %s...", projName)))
			b.WriteString("\n")
		} else {
			b.WriteString(promptStyle.Render(fmt.Sprintf("  Attach branch for %s: ", projName)))
			b.WriteString(m.attachInput.View())
			b.WriteString("\n")
			if len(m.attachBranches) == 0 {
				b.WriteString(dimStyle.Render("    No branches available"))
				b.WriteString("\n")
			} else {
				// Show up to 10 branches around the cursor
				start := m.attachCursor - 5
				if start < 0 {
					start = 0
				}
				end := start + 10
				if end > len(m.attachBranches) {
					end = len(m.attachBranches)
					start = end - 10
					if start < 0 {
						start = 0
					}
				}
				for i := start; i < end; i++ {
					branch := m.attachBranches[i]
					if i == m.attachCursor {
						b.WriteString(fmt.Sprintf("  > %s", selectedStyle.Render(branch)))
					} else {
						b.WriteString(fmt.Sprintf("    %s", normalStyle.Render(branch)))
					}
					b.WriteString("\n")
				}
				if len(m.attachBranches) > 10 {
					b.WriteString(dimStyle.Render(fmt.Sprintf("    (%d branches total)", len(m.attachBranches))))
					b.WriteString("\n")
				}
			}
			b.WriteString(helpStyle.Render("  [↑/↓] navigate  [Enter] select  [Esc] cancel"))
			b.WriteString("\n")
		}
	case ModeHelp:
		b.WriteString("\n")
		b.WriteString(renderGuide())
	default:
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  [Enter] jump  [n] new  [a] attach  [d] delete  [r] refresh  [h] help  [q] quit"))
		b.WriteString("\n")
	}

	return b.String()
}

// nextSelectable returns the index of the next non-header item after the
// current cursor, or the current cursor if none exists.
func (m Model) nextSelectable() int {
	for i := m.cursor + 1; i < len(m.items); i++ {
		if !m.items[i].isHeader {
			return i
		}
	}
	return m.cursor
}

// prevSelectable returns the index of the previous non-header item before the
// current cursor, or the current cursor if none exists.
func (m Model) prevSelectable() int {
	for i := m.cursor - 1; i >= 0; i-- {
		if !m.items[i].isHeader {
			return i
		}
	}
	return m.cursor
}

func refresh(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		states := project.LoadAll(cfg)
		return refreshMsg(states)
	}
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchBranches(repoPath string, features []project.Feature) tea.Cmd {
	return func() tea.Msg {
		// Fetch from all remotes
		if err := worktree.Fetch(repoPath); err != nil {
			return fetchBranchesMsg{err: err}
		}

		branches, err := worktree.ListAvailableBranches(repoPath)
		if err != nil {
			return fetchBranchesMsg{err: err}
		}

		// Filter out branches that already have worktrees
		wtSet := make(map[string]bool)
		for _, f := range features {
			wtSet[f.Branch] = true
		}
		var available []string
		for _, b := range branches {
			if !wtSet[b] {
				available = append(available, b)
			}
		}

		return fetchBranchesMsg{branches: available}
	}
}

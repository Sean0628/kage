package project

import (
	"fmt"

	"github.com/Sean0628/kage/internal/config"
	"github.com/Sean0628/kage/internal/state"
	"github.com/Sean0628/kage/internal/tmux"
	"github.com/Sean0628/kage/internal/worktree"
)

// FeatureStatus represents the state of a feature branch.
type FeatureStatus int

const (
	StatusInactive FeatureStatus = iota
	StatusLive
)

// Feature represents a worktree/branch with its tmux state.
type Feature struct {
	Branch      string
	WorkDir     string
	IsMain      bool
	Status      FeatureStatus
	WindowName  string
	Panes       []PaneStatus
	Description string
}

// PaneStatus represents a pane's configured command and running process.
type PaneStatus struct {
	ConfigCmd      string // from config layout, e.g. "claude", "codex", "npm run dev"
	CurrentProcess string // from tmux pane_current_command
	AgentName      string
	IsAgent        bool
	Status         AgentStatus
}

// ProjectState is the merged view of a project.
type ProjectState struct {
	Config   config.Project
	Features []Feature
}

// LoadAll builds the full state for all configured projects.
func LoadAll(cfg *config.Config, st *state.State) []ProjectState {
	var states []ProjectState
	for _, proj := range cfg.Projects {
		state := LoadProject(cfg, proj, st)
		states = append(states, state)
	}
	return states
}

// LoadProject builds the state for a single project.
func LoadProject(cfg *config.Config, proj config.Project, st *state.State) ProjectState {
	ps := ProjectState{Config: proj}

	// Get worktrees
	wts, err := worktree.List(proj.Path)
	if err != nil {
		return ps
	}

	// Get tmux windows
	windows, _ := tmux.ListWindows()
	windowMap := make(map[string]tmux.WindowInfo)
	for _, w := range windows {
		windowMap[w.Name] = w
	}

	layoutNode := proj.EffectiveLayout(cfg.Defaults)

	for _, wt := range wts {
		feature := Feature{
			Branch:  wt.Branch,
			WorkDir: wt.Path,
			IsMain:  wt.IsMain,
			Status:  StatusInactive,
		}
		if st != nil {
			key := state.DescriptionKey(proj.Name, wt.Branch)
			feature.Description = st.GetDescription(key)
		}

		// Check if there's a tmux window for this branch
		windowName := featureWindowName(proj.Name, wt.Branch)
		if w, ok := windowMap[windowName]; ok {
			feature.Status = StatusLive
			feature.WindowName = windowName
			feature.Panes = loadPaneStatus(
				fmt.Sprintf("%s:%s", tmux.SessionName, w.Index),
				layoutNode,
			)
		}

		ps.Features = append(ps.Features, feature)
	}

	return ps
}

// featureWindowName generates the tmux window name for a feature.
func featureWindowName(projectName, branch string) string {
	return fmt.Sprintf("%s/%s", projectName, branch)
}

func loadPaneStatus(windowTarget string, layout *config.LayoutNode) []PaneStatus {
	tmuxPanes, err := tmux.ListPanes(windowTarget)
	if err != nil {
		return nil
	}

	leaves := layout.Leaves()

	var panes []PaneStatus
	for i, tp := range tmuxPanes {
		ps := PaneStatus{
			CurrentProcess: tp.CurrentCommand,
		}
		if i < len(leaves) {
			ps.ConfigCmd = leaves[i].Cmd
		}
		ps.IsAgent = IsAgentPane(ps.ConfigCmd, ps.CurrentProcess)
		if ps.IsAgent {
			ps.AgentName = AgentDisplayName(ps.ConfigCmd, ps.CurrentProcess)
			output, err := tmux.CapturePane(fmt.Sprintf("%s.%d", windowTarget, tp.Index), 8)
			if err == nil {
				ps.Status = DetectAgentStatus(ps.ConfigCmd, ps.CurrentProcess, output)
			} else {
				ps.Status = DetectAgentStatus(ps.ConfigCmd, ps.CurrentProcess, "")
			}
		}
		panes = append(panes, ps)
	}
	return panes
}

// LaunchFeature creates a worktree (if needed), tmux window, and sets up the pane layout.
func LaunchFeature(cfg *config.Config, proj config.Project, branch string, createNew bool) error {
	// Check if worktree already exists
	wts, err := worktree.List(proj.Path)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	var workDir string
	found := false
	for _, wt := range wts {
		if wt.Branch == branch {
			workDir = wt.Path
			found = true
			break
		}
	}

	if !found {
		if createNew {
			workDir, err = worktree.AddNewBranch(proj.Path, branch)
		} else {
			workDir, err = worktree.Add(proj.Path, branch)
		}
		if err != nil {
			return fmt.Errorf("creating worktree: %w", err)
		}
	}

	windowName := featureWindowName(proj.Name, branch)

	// Check if window already exists with the correct pane count
	layoutNode := proj.EffectiveLayout(cfg.Defaults)
	expectedPanes := len(layoutNode.Leaves())
	windows, _ := tmux.ListWindows()
	for _, w := range windows {
		if w.Name == windowName {
			// Verify the window has the expected number of panes
			windowTarget := fmt.Sprintf("%s:%s", tmux.SessionName, w.Index)
			panes, _ := tmux.ListPanes(windowTarget)
			if len(panes) >= expectedPanes {
				return tmux.SelectWindow(w.Index)
			}
			// Pane count mismatch — kill the stale window and recreate below
			tmux.KillWindow(windowTarget)
			break
		}
	}

	// Find the first leaf command so we can run it directly in the new window,
	// avoiding the race condition where SendKeys fires before the shell is ready.
	firstCmd := firstLeafCmd(layoutNode)
	if firstCmd != "" {
		if err := tmux.NewWindowWithCmd(windowName, workDir, true, firstCmd); err != nil {
			return fmt.Errorf("creating window: %w", err)
		}
	} else {
		if err := tmux.NewWindow(windowName, workDir, true); err != nil {
			return fmt.Errorf("creating window: %w", err)
		}
	}

	// Find the window's numeric index for reliable targeting
	// (window names containing "/" confuse tmux target parsing with .pane suffix)
	windows, _ = tmux.ListWindows()
	var windowIndex string
	for _, w := range windows {
		if w.Name == windowName {
			windowIndex = w.Index
			break
		}
	}
	if windowIndex == "" {
		return fmt.Errorf("window %s not found after creation", windowName)
	}

	windowTarget := fmt.Sprintf("%s:%s", tmux.SessionName, windowIndex)

	// Query the initial pane
	panes, err := tmux.ListPanes(windowTarget)
	if err != nil || len(panes) == 0 {
		tmux.KillWindow(windowTarget)
		return fmt.Errorf("no panes in new window")
	}
	firstPane := fmt.Sprintf("%s.%d", windowTarget, panes[0].Index)

	// Setup pane layout recursively; first leaf command already handled by NewWindowWithCmd
	if err := tmux.SetupLayoutTree(windowTarget, layoutNode, firstPane, workDir, firstCmd != ""); err != nil {
		tmux.KillWindow(windowTarget)
		return fmt.Errorf("setting up layout: %w", err)
	}

	// Select the first pane
	tmux.RunSilent("select-pane", "-t", firstPane)

	// Switch to the fully set up window
	return tmux.SelectWindow(windowIndex)
}

// DeleteFeature removes a feature's tmux window and worktree.
func DeleteFeature(proj config.Project, branch string, force bool) error {
	windowName := featureWindowName(proj.Name, branch)

	// Kill tmux window if it exists
	windows, _ := tmux.ListWindows()
	for _, w := range windows {
		if w.Name == windowName {
			tmux.KillWindow(fmt.Sprintf("%s:%s", tmux.SessionName, w.Index))
			break
		}
	}

	// Remove worktree
	wts, err := worktree.List(proj.Path)
	if err != nil {
		return err
	}
	for _, wt := range wts {
		if wt.Branch == branch && !wt.IsMain {
			return worktree.Remove(proj.Path, wt.Path, force)
		}
	}
	return nil
}

// firstLeafCmd returns the command of the first leaf node in a layout tree,
// or "" if the first leaf is a "shell" or has no command.
func firstLeafCmd(node *config.LayoutNode) string {
	if node == nil {
		return ""
	}
	if node.IsLeaf() {
		if node.Cmd != "" && node.Cmd != "shell" {
			return node.Cmd
		}
		return ""
	}
	if len(node.Panes) == 0 {
		return ""
	}
	return firstLeafCmd(node.Panes[0])
}

// FeatureWindowName is exported for use by TUI.
func FeatureWindowName(projectName, branch string) string {
	return featureWindowName(projectName, branch)
}

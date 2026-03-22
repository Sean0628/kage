package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Sean0628/kage/internal/config"
)

// WindowInfo represents a tmux window.
type WindowInfo struct {
	Index string
	Name  string
}

// PaneInfo represents a tmux pane.
type PaneInfo struct {
	Index          int
	CurrentCommand string
}

// NewWindow creates a new window in the kage session.
// If detached is true, the window is created without switching to it.
func NewWindow(name string, startDir string, detached bool) error {
	args := []string{"new-window", "-t", SessionName + ":", "-n", name}
	if detached {
		args = append(args, "-d")
	}
	if startDir != "" {
		args = append(args, "-c", startDir)
	}
	return RunSilent(args...)
}

// NewWindowWithCmd creates a new window and runs a command directly in it.
// Unlike NewWindow + SendKeys, this avoids the race condition where keystrokes
// arrive before the shell is ready.
func NewWindowWithCmd(name string, startDir string, detached bool, cmd string) error {
	args := []string{"new-window", "-t", SessionName + ":", "-n", name}
	if detached {
		args = append(args, "-d")
	}
	if startDir != "" {
		args = append(args, "-c", startDir)
	}
	args = append(args, cmd)
	return RunSilent(args...)
}

// SplitWindow splits a pane. If horizontal is true, splits top/bottom (-v flag in tmux).
// size is a percentage string like "20%".
func SplitWindow(target string, horizontal bool, size string, startDir string) error {
	flag := "-h" // side by side
	if horizontal {
		flag = "-v" // top/bottom
	}
	args := []string{"split-window", flag, "-t", target}
	if size != "" {
		args = append(args, "-p", strings.TrimSuffix(size, "%"))
	}
	if startDir != "" {
		args = append(args, "-c", startDir)
	}
	return RunSilent(args...)
}

// SplitWindowWithCmd splits a pane and runs a command directly in the new pane.
// Unlike SplitWindow + SendKeys, this avoids the race condition where keystrokes
// arrive before the shell is ready. The pane is set to remain-on-exit so it
// stays open if the command exits.
// If detached is true, the original pane retains focus.
func SplitWindowWithCmd(target string, horizontal bool, size string, startDir string, cmd string, detached bool) error {
	flag := "-h" // side by side
	if horizontal {
		flag = "-v" // top/bottom
	}
	args := []string{"split-window", flag, "-t", target}
	if detached {
		args = append(args, "-d")
	}
	if size != "" {
		args = append(args, "-p", strings.TrimSuffix(size, "%"))
	}
	if startDir != "" {
		args = append(args, "-c", startDir)
	}
	args = append(args, cmd)
	return RunSilent(args...)
}

// SendKeys sends keystrokes to a tmux pane.
func SendKeys(target string, keys string) error {
	return RunSilent("send-keys", "-t", target, keys, "Enter")
}

// SendKeysLiteral sends literal text to a tmux pane (no special key interpretation).
func SendKeysLiteral(target string, text string) error {
	return RunSilent("send-keys", "-t", target, "-l", text)
}

// CapturePane captures visible content from a tmux pane.
// If lines > 0, only the last N lines are captured.
func CapturePane(target string, lines int) (string, error) {
	args := []string{"capture-pane", "-t", target, "-p"}
	if lines > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", lines))
	}
	return Run(args...)
}

// ListWindows returns all windows in the kage session.
func ListWindows() ([]WindowInfo, error) {
	out, err := Run("list-windows", "-t", SessionName, "-F", "#{window_index}|#{window_name}")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var windows []WindowInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			windows = append(windows, WindowInfo{Index: parts[0], Name: parts[1]})
		}
	}
	return windows, nil
}

// ListPanes returns all panes in a specific window.
func ListPanes(windowTarget string) ([]PaneInfo, error) {
	out, err := Run("list-panes", "-t", windowTarget, "-F", "#{pane_index}|#{pane_current_command}")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var panes []PaneInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			idx, _ := strconv.Atoi(parts[0])
			panes = append(panes, PaneInfo{Index: idx, CurrentCommand: parts[1]})
		}
	}
	return panes, nil
}

// KillWindow destroys a window.
func KillWindow(target string) error {
	return RunSilent("kill-window", "-t", target)
}

// CalcRelativeSplitSizes computes the tmux split percentages needed to achieve
// the desired absolute layout. tmux splits are relative to the remaining space.
// For example, [60%, 20%, 20%] → the first pane gets 60%. The remaining 40%
// is split: 20/40 = 50% for the second, leaving 20/20 = 100% (no split needed).
// Returns the percentage for each split operation (first pane is the initial window,
// so we return n-1 values for n panes).
func CalcRelativeSplitSizes(panes []int) []int {
	if len(panes) <= 1 {
		return nil
	}

	total := 0
	for _, p := range panes {
		total += p
	}

	var splits []int
	remaining := total
	for i := 0; i < len(panes)-1; i++ {
		remaining -= panes[i]
		// The split percentage is relative to the current pane being split.
		// We want `remaining` out of `panes[i] + remaining`.
		pct := (remaining * 100) / (panes[i] + remaining)
		splits = append(splits, pct)
	}
	return splits
}

// ParseSizePercent extracts the integer from a percentage string like "60%".
func ParseSizePercent(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	n, _ := strconv.Atoi(s)
	return n
}

// SetupLayoutTree creates panes according to a recursive layout tree.
// windowTarget is like "kage:1", paneTarget is like "kage:1.0".
// If initialCmdHandled is true, the first leaf pane's command was already
// started (e.g. via NewWindowWithCmd) and should not be sent again.
func SetupLayoutTree(windowTarget string, node *config.LayoutNode, paneTarget string, workDir string, initialCmdHandled bool) error {
	if node == nil {
		return nil
	}

	// Leaf node: send the command unless already handled
	if node.IsLeaf() {
		if initialCmdHandled {
			return nil
		}
		if node.Cmd != "" && node.Cmd != "shell" {
			return SendKeys(paneTarget, node.Cmd)
		}
		return nil
	}

	// Branch node: split the pane and recurse
	if len(node.Panes) == 0 {
		return nil
	}

	horizontal := node.Split == "horizontal" || node.Split == ""

	sizes := make([]int, len(node.Panes))
	for i, p := range node.Panes {
		sizes[i] = ParseSizePercent(p.Size)
	}
	splits := CalcRelativeSplitSizes(sizes)

	// Track pane targets for each child
	paneTargets := make([]string, len(node.Panes))
	paneTargets[0] = paneTarget

	// Track which children already have their command running
	cmdHandled := make([]bool, len(node.Panes))
	cmdHandled[0] = initialCmdHandled

	currentPane := paneTarget
	for i, splitPct := range splits {
		child := node.Panes[i+1]
		sizeStr := fmt.Sprintf("%d", splitPct)

		// For leaf children with commands, use SplitWindowWithCmd to avoid
		// the race condition where SendKeys fires before the shell is ready.
		if child.IsLeaf() && child.Cmd != "" && child.Cmd != "shell" {
			if err := SplitWindowWithCmd(currentPane, horizontal, sizeStr, workDir, child.Cmd, false); err != nil {
				return fmt.Errorf("splitting for child %d: %w", i+1, err)
			}
			cmdHandled[i+1] = true
		} else {
			if err := SplitWindow(currentPane, horizontal, sizeStr, workDir); err != nil {
				return fmt.Errorf("splitting for child %d: %w", i+1, err)
			}
		}

		updatedPanes, err := ListPanes(windowTarget)
		if err != nil {
			return fmt.Errorf("listing panes after split %d: %w", i+1, err)
		}
		newPane := updatedPanes[len(updatedPanes)-1]
		newTarget := fmt.Sprintf("%s.%d", windowTarget, newPane.Index)
		paneTargets[i+1] = newTarget
		currentPane = newTarget
	}

	// Recurse into each child
	for i, child := range node.Panes {
		if child.IsLeaf() {
			if !cmdHandled[i] && child.Cmd != "" && child.Cmd != "shell" {
				if err := SendKeys(paneTargets[i], child.Cmd); err != nil {
					return err
				}
			}
			continue
		}
		if err := SetupLayoutTree(windowTarget, child, paneTargets[i], workDir, cmdHandled[i]); err != nil {
			return err
		}
	}
	return nil
}

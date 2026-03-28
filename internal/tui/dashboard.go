package tui

import (
	"fmt"
	"strings"

	"github.com/Sean0628/kage/internal/project"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	defaultBranchColumnWidth = 28
	defaultStatusColumnWidth = 48
	minBranchColumnWidth     = 20
	minStatusColumnWidth     = 24
	minPaneColumnWidth       = 12
	rowFixedWidth            = 11
)

type dashboardLayout struct {
	branchWidth int
	statusWidth int
	paneWidth   int
}

// listItem is a flattened item in the dashboard list.
type listItem struct {
	isHeader    bool
	projectIdx  int
	featureIdx  int
	projectName string
	feature     *project.Feature
	liveCount   int // only for headers
}

// flattenItems converts project states into a flat list of items for the TUI.
func flattenItems(states []project.ProjectState) []listItem {
	var items []listItem
	for pi, ps := range states {
		liveCount := 0
		for _, f := range ps.Features {
			if f.Status == project.StatusLive {
				liveCount++
			}
		}
		items = append(items, listItem{
			isHeader:    true,
			projectIdx:  pi,
			projectName: ps.Config.Name,
			liveCount:   liveCount,
		})
		for fi := range ps.Features {
			items = append(items, listItem{
				isHeader:   false,
				projectIdx: pi,
				featureIdx: fi,
				feature:    &states[pi].Features[fi],
			})
		}
	}
	return items
}

// renderItem renders a single list item.
func renderItem(item listItem, selected bool, layout dashboardLayout) string {
	if item.isHeader {
		return renderHeader(item, selected)
	}
	return renderFeature(item, selected, layout)
}

func renderHeader(item listItem, _ bool) string {
	name := projectStyle.Render(item.projectName)
	suffix := ""
	if item.liveCount > 0 {
		suffix = dimStyle.Render(fmt.Sprintf("  (%d live)", item.liveCount))
	}
	return "  " + name + suffix
}

func renderFeature(item listItem, selected bool, layout dashboardLayout) string {
	f := item.feature
	var marker string
	if f.Status == project.StatusLive {
		marker = liveMarker.String()
	} else {
		marker = inactiveMarker.String()
	}

	branch := f.Branch
	if selected {
		branch = selectedStyle.Render(branch)
	} else {
		branch = normalStyle.Render(branch)
	}
	branchInfo := branch
	if f.ID > 0 {
		branchInfo = dimStyle.Render(fmt.Sprintf("#%d ", f.ID)) + branchInfo
	}
	if f.Description != "" {
		branchInfo += dimStyle.Render("  " + f.Description)
	}

	statusInfo := renderStatusColumn(f)
	paneInfo := renderPaneColumn(f)

	cursor := "  "
	if selected {
		cursor = "> "
	}
	return fmt.Sprintf(
		"%s    %s %s %s %s",
		cursor,
		marker,
		renderColumnCell(branchInfo, layout.branchWidth),
		renderColumnCell(statusInfo, layout.statusWidth),
		renderColumnCell(paneInfo, layout.paneWidth),
	)
}

func formatPanes(panes []project.PaneStatus) string {
	var names []string
	for _, p := range panes {
		name := p.CurrentProcess
		if name == "" {
			name = p.ConfigCmd
		}
		names = append(names, name)
	}
	return "[" + strings.Join(names, ", ") + "]"
}

func renderColumnHeaders(layout dashboardLayout) string {
	branch := renderColumnCell(columnHeaderStyle.Render("Branch"), layout.branchWidth)
	status := renderColumnCell(columnHeaderStyle.Render("Status"), layout.statusWidth)
	panes := renderColumnCell(columnHeaderStyle.Render("Panes"), layout.paneWidth)
	return fmt.Sprintf("      %s %s %s", branch, status, panes)
}

func renderStatusColumn(f *project.Feature) string {
	if f.Status != project.StatusLive {
		return dimStyle.Render("—")
	}

	var parts []string
	for _, p := range f.Panes {
		if !p.IsAgent {
			continue
		}
		parts = append(parts, renderAgentStatusToken(p))
	}
	if len(parts) == 0 {
		return dimStyle.Render("—")
	}
	return strings.Join(parts, dimStyle.Render(", "))
}

func renderAgentStatusToken(p project.PaneStatus) string {
	label := fmt.Sprintf("%s:%s", p.AgentName, p.Status.Label())
	switch p.Status {
	case project.AgentStatusIdle:
		return statusIdleStyle.Render(label)
	case project.AgentStatusRunning:
		return statusRunningStyle.Render(label)
	case project.AgentStatusWaitingInput:
		return statusWaitingInputStyle.Render(label)
	case project.AgentStatusWaitingPermission:
		return statusWaitingPermissionStyle.Render(label)
	default:
		return dimStyle.Render(label)
	}
}

func renderPaneColumn(f *project.Feature) string {
	if f.Status == project.StatusLive && len(f.Panes) > 0 {
		return dimStyle.Render(formatPanes(f.Panes))
	}
	return dimStyle.Render("—")
}

func computeDashboardLayout(totalWidth int) dashboardLayout {
	layout := dashboardLayout{
		branchWidth: defaultBranchColumnWidth,
		statusWidth: defaultStatusColumnWidth,
		paneWidth:   minPaneColumnWidth,
	}

	if totalWidth <= 0 {
		return layout
	}

	available := totalWidth - rowFixedWidth
	minTotal := minBranchColumnWidth + minStatusColumnWidth + minPaneColumnWidth
	if available <= minTotal {
		return dashboardLayout{
			branchWidth: minBranchColumnWidth,
			statusWidth: minStatusColumnWidth,
			paneWidth:   minPaneColumnWidth,
		}
	}

	branchWidth := max(minBranchColumnWidth, available*35/100)
	statusWidth := max(minStatusColumnWidth, available*30/100)
	paneWidth := available - branchWidth - statusWidth

	if paneWidth < minPaneColumnWidth {
		deficit := minPaneColumnWidth - paneWidth
		reducibleStatus := statusWidth - minStatusColumnWidth
		reduceStatus := min(deficit, reducibleStatus)
		statusWidth -= reduceStatus
		deficit -= reduceStatus
		if deficit > 0 {
			reducibleBranch := branchWidth - minBranchColumnWidth
			reduceBranch := min(deficit, reducibleBranch)
			branchWidth -= reduceBranch
			deficit -= reduceBranch
		}
		paneWidth = minPaneColumnWidth + max(0, -deficit)
	}

	return dashboardLayout{
		branchWidth: branchWidth,
		statusWidth: statusWidth,
		paneWidth:   available - branchWidth - statusWidth,
	}
}

func renderColumnCell(content string, width int) string {
	content = ansi.Truncate(content, width, "…")
	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Inline(true).
		Render(content)
}

package tui

import (
	"fmt"
	"strings"

	"github.com/shoito/kage/internal/project"
)

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
func renderItem(item listItem, selected bool) string {
	if item.isHeader {
		return renderHeader(item, selected)
	}
	return renderFeature(item, selected)
}

func renderHeader(item listItem, _ bool) string {
	name := projectStyle.Render(item.projectName)
	suffix := ""
	if item.liveCount > 0 {
		suffix = dimStyle.Render(fmt.Sprintf("  (%d live)", item.liveCount))
	}
	return "  " + name + suffix
}

func renderFeature(item listItem, selected bool) string {
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

	paneInfo := ""
	if f.Status == project.StatusLive && len(f.Panes) > 0 {
		paneInfo = dimStyle.Render("  " + formatPanes(f.Panes))
	} else if f.Status == project.StatusInactive {
		paneInfo = dimStyle.Render("  —")
	}

	cursor := "  "
	if selected {
		cursor = "> "
	}
	return fmt.Sprintf("%s    %s %s%s", cursor, marker, branch, paneInfo)
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

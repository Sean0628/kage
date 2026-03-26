package tui

import (
	"strings"
	"testing"

	"github.com/Sean0628/kage/internal/project"
)

func TestRenderFeatureShowsAgentStatuses(t *testing.T) {
	layout := computeDashboardLayout(160)
	item := listItem{
		feature: &project.Feature{
			Branch: "feature/auth",
			Status: project.StatusLive,
			Panes: []project.PaneStatus{
				{
					ConfigCmd:      "claude",
					CurrentProcess: "claude",
					AgentName:      "claude",
					IsAgent:        true,
					Status:         project.AgentStatusRunning,
				},
				{
					ConfigCmd:      "codex",
					CurrentProcess: "codex",
					AgentName:      "codex",
					IsAgent:        true,
					Status:         project.AgentStatusWaitingPermission,
				},
				{
					ConfigCmd:      "shell",
					CurrentProcess: "zsh",
				},
			},
		},
	}

	got := renderFeature(item, false, layout)
	for _, want := range []string{"feature/auth", "claude:running", "codex:waiting permission", "[claude, codex, zsh]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderFeature() missing %q in %q", want, got)
		}
	}
}

func TestRenderFeatureInactiveStaysClean(t *testing.T) {
	layout := computeDashboardLayout(120)
	item := listItem{
		feature: &project.Feature{
			Branch: "feature/inactive",
			Status: project.StatusInactive,
		},
	}

	got := renderFeature(item, false, layout)
	if !strings.Contains(got, "feature/inactive") {
		t.Fatalf("renderFeature() missing branch in %q", got)
	}
	if strings.Count(got, "—") < 2 {
		t.Fatalf("renderFeature() should show placeholders for inactive row: %q", got)
	}
}

func TestRenderColumnHeaders(t *testing.T) {
	got := renderColumnHeaders(computeDashboardLayout(120))
	for _, want := range []string{"Branch", "Status", "Panes"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderColumnHeaders() missing %q in %q", want, got)
		}
	}
}

func TestRenderFeatureTruncatesLongColumns(t *testing.T) {
	layout := computeDashboardLayout(90)
	item := listItem{
		feature: &project.Feature{
			Branch: "feature/very-long-branch-name-that-should-not-wrap",
			Status: project.StatusLive,
			Panes: []project.PaneStatus{
				{
					ConfigCmd:      "codex",
					CurrentProcess: "codex",
					AgentName:      "codex",
					IsAgent:        true,
					Status:         project.AgentStatusWaitingPermission,
				},
				{
					ConfigCmd:      "shell",
					CurrentProcess: "zsh",
				},
			},
		},
	}

	got := renderFeature(item, false, layout)
	if strings.Contains(got, "\n") {
		t.Fatalf("renderFeature() should stay on one line, got %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("renderFeature() should truncate long columns, got %q", got)
	}
}

func TestRenderFeatureShowsDescriptionInBranchColumn(t *testing.T) {
	layout := computeDashboardLayout(140)
	item := listItem{
		feature: &project.Feature{
			Branch:      "feature/desc",
			Description: "keep this visible",
			Status:      project.StatusInactive,
		},
	}

	got := renderFeature(item, false, layout)
	for _, want := range []string{"feature/desc", "keep this visible"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderFeature() missing %q in %q", want, got)
		}
	}
}

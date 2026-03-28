package project

import (
	"testing"

	"github.com/Sean0628/kage/internal/config"
)

func TestFeatureWindowName(t *testing.T) {
	tests := []struct {
		projectName string
		branch      string
		expected    string
	}{
		{"kage", "main", "kage/main"},
		{"kage", "feature/auth", "kage/feature/auth"},
		{"other-repo", "fix/login", "other-repo/fix/login"},
	}

	for _, tt := range tests {
		got := FeatureWindowName(tt.projectName, tt.branch)
		if got != tt.expected {
			t.Errorf("FeatureWindowName(%q, %q) = %q, want %q",
				tt.projectName, tt.branch, got, tt.expected)
		}
	}
}

func TestAssignFeatureIDsAcrossProjects(t *testing.T) {
	states := []ProjectState{
		{
			Config: config.Project{Name: "kage"},
			Features: []Feature{
				{Branch: "main"},
				{Branch: "feature/auth"},
			},
		},
		{
			Config: config.Project{Name: "project-b"},
			Features: []Feature{
				{Branch: "main"},
			},
		},
	}

	assignFeatureIDs(states)

	got := []int{
		states[0].Features[0].ID,
		states[0].Features[1].ID,
		states[1].Features[0].ID,
	}
	want := []int{1, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("feature id %d = %d, want %d", i, got[i], want[i])
		}
	}
}

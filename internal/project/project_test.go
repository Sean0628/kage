package project

import (
	"testing"
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

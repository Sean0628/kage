package worktree

import (
	"testing"
)

func TestParsePorcelain(t *testing.T) {
	input := `worktree /Users/shoito/work/kage
HEAD abc123def456
branch refs/heads/main

worktree /Users/shoito/work/kage-feature-auth
HEAD def456abc123
branch refs/heads/feature/auth

worktree /Users/shoito/work/kage-fix-typo
HEAD 789abc123def
branch refs/heads/fix/typo

`
	wts := ParsePorcelain(input)

	if len(wts) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(wts))
	}

	// First worktree is main
	if !wts[0].IsMain {
		t.Error("expected first worktree to be main")
	}
	if wts[0].Branch != "main" {
		t.Errorf("expected branch 'main', got %q", wts[0].Branch)
	}
	if wts[0].Path != "/Users/shoito/work/kage" {
		t.Errorf("unexpected path: %s", wts[0].Path)
	}

	// Second worktree
	if wts[1].IsMain {
		t.Error("second worktree should not be main")
	}
	if wts[1].Branch != "feature/auth" {
		t.Errorf("expected branch 'feature/auth', got %q", wts[1].Branch)
	}

	// Third worktree
	if wts[2].Branch != "fix/typo" {
		t.Errorf("expected branch 'fix/typo', got %q", wts[2].Branch)
	}
}

func TestParsePorcelainBare(t *testing.T) {
	input := `worktree /Users/shoito/work/kage.git
bare

`
	wts := ParsePorcelain(input)
	if len(wts) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(wts))
	}
	if !wts[0].Bare {
		t.Error("expected bare worktree")
	}
}

func TestParsePorcelainEmpty(t *testing.T) {
	wts := ParsePorcelain("")
	if len(wts) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(wts))
	}
}

func TestWorktreePath(t *testing.T) {
	tests := []struct {
		repoPath string
		branch   string
		expected string
	}{
		{"/Users/shoito/work/kage", "feature/auth", "/Users/shoito/work/kage-feature-auth"},
		{"/Users/shoito/work/kage", "main", "/Users/shoito/work/kage-main"},
		{"/Users/shoito/work/kage", "fix/login/bug", "/Users/shoito/work/kage-fix-login-bug"},
	}

	for _, tt := range tests {
		got := WorktreePath(tt.repoPath, tt.branch)
		if got != tt.expected {
			t.Errorf("WorktreePath(%q, %q) = %q, want %q", tt.repoPath, tt.branch, got, tt.expected)
		}
	}
}

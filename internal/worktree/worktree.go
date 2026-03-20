package worktree

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Info represents a single git worktree entry.
type Info struct {
	Path   string
	Branch string
	IsMain bool
	Bare   bool
}

// List returns all worktrees for the repo at repoPath.
func List(repoPath string) ([]Info, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git worktree list: %s: %w", stderr.String(), err)
	}
	return ParsePorcelain(stdout.String()), nil
}

// ParsePorcelain parses the output of `git worktree list --porcelain`.
func ParsePorcelain(output string) []Info {
	var worktrees []Info
	var current Info
	isFirst := true

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				current.IsMain = isFirst
				worktrees = append(worktrees, current)
				isFirst = false
			}
			current = Info{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			// refs/heads/main → main
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Bare = true
		}
	}
	if current.Path != "" {
		current.IsMain = isFirst
		worktrees = append(worktrees, current)
	}
	return worktrees
}

// WorktreePath computes the path for a new worktree.
// Convention: <repo-parent>/<repo-name>-<branch>
func WorktreePath(repoPath string, branch string) string {
	parent := filepath.Dir(repoPath)
	repoName := filepath.Base(repoPath)
	// Replace slashes in branch name with dashes
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(parent, repoName+"-"+safeBranch)
}

// Fetch runs git fetch --all --prune to update remote-tracking branches.
func Fetch(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "--all", "--prune")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch: %s: %w", stderr.String(), err)
	}
	return nil
}

// ListRemoteBranches returns remote-tracking branch names (e.g. "origin/feature-x").
func ListRemoteBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "-r", "--format=%(refname:short)")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git branch -r: %s: %w", stderr.String(), err)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" && !strings.HasSuffix(line, "/HEAD") {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ListAvailableBranches returns all branch names (local + remote-only, deduplicated).
// Remote-only branches are returned without the remote prefix (e.g. "feature-x" not "origin/feature-x").
func ListAvailableBranches(repoPath string) ([]string, error) {
	local, err := ListBranches(repoPath)
	if err != nil {
		return nil, err
	}
	localSet := make(map[string]bool)
	for _, b := range local {
		localSet[b] = true
	}

	remote, err := ListRemoteBranches(repoPath)
	if err != nil {
		return nil, err
	}

	result := append([]string{}, local...)
	for _, r := range remote {
		// Strip first remote prefix (e.g. "origin/feature-x" → "feature-x")
		parts := strings.SplitN(r, "/", 2)
		if len(parts) < 2 {
			continue
		}
		name := parts[1]
		if !localSet[name] {
			result = append(result, name)
			localSet[name] = true
		}
	}
	return result, nil
}

// Add creates a new worktree at the computed path for the given branch.
// Uses --guess-remote so that remote-tracking branches are automatically
// checked out as new local tracking branches.
func Add(repoPath string, branch string) (string, error) {
	wtPath := WorktreePath(repoPath, branch)
	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", "--guess-remote", wtPath, branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", stderr.String(), err)
	}
	return wtPath, nil
}

// AddNewBranch creates a new worktree with a new branch.
func AddNewBranch(repoPath string, branch string) (string, error) {
	wtPath := WorktreePath(repoPath, branch)
	cmd := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branch, wtPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add -b: %s: %w", stderr.String(), err)
	}
	return wtPath, nil
}

// Remove removes a worktree. If force is true, uses --force.
func Remove(repoPath string, wtPath string, force bool) error {
	args := []string{"-C", repoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wtPath)
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", stderr.String(), err)
	}
	return nil
}

// ListBranches returns local branch names for the repo.
func ListBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git branch: %s: %w", stderr.String(), err)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

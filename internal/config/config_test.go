package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	leaves := cfg.Defaults.Layout.Root.Leaves()
	if len(leaves) != 3 {
		t.Errorf("expected 3 default layout leaves, got %d", len(leaves))
	}
	if cfg.Defaults.Layout.Root.Split != "horizontal" {
		t.Errorf("expected horizontal split, got %s", cfg.Defaults.Layout.Root.Split)
	}
	if cfg.Defaults.Split != "horizontal" {
		t.Errorf("expected horizontal split, got %s", cfg.Defaults.Split)
	}
	if len(cfg.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(cfg.Projects))
	}

	// File should have been created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `
defaults:
  layout:
    - { cmd: "claude", size: "50%" }
    - { cmd: "shell", size: "50%" }
  split: vertical

projects:
  - path: /tmp/myrepo
    name: myrepo
  - path: /tmp/other
    name: other
    layout:
      - { cmd: "claude", size: "70%" }
      - { cmd: "shell", size: "30%" }
`
	os.WriteFile(path, []byte(yaml), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	leaves := cfg.Defaults.Layout.Root.Leaves()
	if len(leaves) != 2 {
		t.Errorf("expected 2 layout leaves, got %d", len(leaves))
	}
	if cfg.Defaults.Split != "vertical" {
		t.Errorf("expected vertical, got %s", cfg.Defaults.Split)
	}
	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}
	// Project without split should inherit default
	if cfg.Projects[0].Split != "vertical" {
		t.Errorf("expected project to inherit vertical split, got %s", cfg.Projects[0].Split)
	}
	// Project with layout should have it parsed
	otherLeaves := cfg.Projects[1].Layout.Root.Leaves()
	if len(otherLeaves) != 2 {
		t.Errorf("expected 2 leaves for other project, got %d", len(otherLeaves))
	}
	if otherLeaves[0].Cmd != "claude" {
		t.Errorf("expected first pane cmd 'claude', got %s", otherLeaves[0].Cmd)
	}
}

func TestLoadNestedLayout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `
defaults:
  layout:
    split: vertical
    panes:
      - split: horizontal
        size: "60%"
        panes:
          - { cmd: "claude", size: "50%" }
          - { cmd: "codex", size: "50%" }
      - { cmd: "shell", size: "40%" }
  split: vertical

projects: []
`
	os.WriteFile(path, []byte(yaml), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	root := cfg.Defaults.Layout.Root
	if root.Split != "vertical" {
		t.Errorf("expected root split vertical, got %s", root.Split)
	}
	if len(root.Panes) != 2 {
		t.Fatalf("expected 2 top-level children, got %d", len(root.Panes))
	}

	// First child is a branch
	top := root.Panes[0]
	if top.IsLeaf() {
		t.Error("expected first child to be a branch")
	}
	if top.Split != "horizontal" {
		t.Errorf("expected horizontal split, got %s", top.Split)
	}
	if len(top.Panes) != 2 {
		t.Fatalf("expected 2 sub-panes, got %d", len(top.Panes))
	}
	if top.Panes[0].Cmd != "claude" {
		t.Errorf("expected claude, got %s", top.Panes[0].Cmd)
	}
	if top.Panes[1].Cmd != "codex" {
		t.Errorf("expected codex, got %s", top.Panes[1].Cmd)
	}

	// Second child is a leaf
	bottom := root.Panes[1]
	if !bottom.IsLeaf() {
		t.Error("expected second child to be a leaf")
	}
	if bottom.Cmd != "shell" {
		t.Errorf("expected shell, got %s", bottom.Cmd)
	}

	// Total leaves
	leaves := root.Leaves()
	if len(leaves) != 3 {
		t.Errorf("expected 3 leaves, got %d", len(leaves))
	}
}

func TestEffectiveLayout(t *testing.T) {
	defaults := Defaults{
		Layout: &LayoutSpec{Root: DefaultLayout},
		Split:  DefaultSplit,
	}

	// Project with no custom layout should use defaults
	p := Project{Path: "/tmp/repo", Name: "repo"}
	layout := p.EffectiveLayout(defaults)
	if len(layout.Leaves()) != 3 {
		t.Errorf("expected 3 leaves from defaults, got %d", len(layout.Leaves()))
	}

	// Project with custom layout should use its own
	p2 := Project{
		Path: "/tmp/repo2",
		Name: "repo2",
		Layout: &LayoutSpec{Root: &LayoutNode{
			Split: "horizontal",
			Panes: []*LayoutNode{{Cmd: "claude", Size: "100%"}},
		}},
	}
	layout2 := p2.EffectiveLayout(defaults)
	if len(layout2.Leaves()) != 1 {
		t.Errorf("expected 1 leaf from project layout, got %d", len(layout2.Leaves()))
	}
}

func TestLayoutNodeLeaves(t *testing.T) {
	// Single leaf
	leaf := &LayoutNode{Cmd: "claude", Size: "100%"}
	if len(leaf.Leaves()) != 1 {
		t.Errorf("expected 1 leaf, got %d", len(leaf.Leaves()))
	}

	// Nested tree
	tree := &LayoutNode{
		Split: "vertical",
		Panes: []*LayoutNode{
			{
				Split: "horizontal",
				Panes: []*LayoutNode{
					{Cmd: "a", Size: "50%"},
					{Cmd: "b", Size: "50%"},
				},
			},
			{Cmd: "c", Size: "100%"},
		},
	}
	leaves := tree.Leaves()
	if len(leaves) != 3 {
		t.Fatalf("expected 3 leaves, got %d", len(leaves))
	}
	if leaves[0].Cmd != "a" || leaves[1].Cmd != "b" || leaves[2].Cmd != "c" {
		t.Errorf("unexpected leaf order: %s, %s, %s", leaves[0].Cmd, leaves[1].Cmd, leaves[2].Cmd)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

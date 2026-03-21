package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Pane is the legacy flat layout entry. Kept for backward-compatible YAML parsing.
type Pane struct {
	Cmd  string `yaml:"cmd"`
	Size string `yaml:"size"`
}

// LayoutNode is a recursive layout tree node.
// A leaf has Cmd set; a branch has Split + Panes set.
type LayoutNode struct {
	// Leaf fields
	Cmd  string `yaml:"cmd,omitempty"`
	Size string `yaml:"size,omitempty"`

	// Branch fields
	Split string        `yaml:"split,omitempty"`
	Panes []*LayoutNode `yaml:"panes,omitempty"`
}

// IsLeaf returns true if this node represents a single pane (has a command).
func (n *LayoutNode) IsLeaf() bool {
	return len(n.Panes) == 0
}

// Leaves returns all leaf nodes in depth-first order.
func (n *LayoutNode) Leaves() []*LayoutNode {
	if n.IsLeaf() {
		return []*LayoutNode{n}
	}
	var leaves []*LayoutNode
	for _, child := range n.Panes {
		leaves = append(leaves, child.Leaves()...)
	}
	return leaves
}

// LayoutSpec wraps a LayoutNode pointer with custom YAML unmarshaling
// to support both the old flat list format and the new nested format.
type LayoutSpec struct {
	Root *LayoutNode
}

func (ls *LayoutSpec) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.SequenceNode:
		// Old flat format: [{cmd, size}, ...]
		var panes []Pane
		if err := value.Decode(&panes); err != nil {
			return fmt.Errorf("decoding flat layout: %w", err)
		}
		children := make([]*LayoutNode, len(panes))
		for i, p := range panes {
			children[i] = &LayoutNode{Cmd: p.Cmd, Size: p.Size}
		}
		ls.Root = &LayoutNode{Panes: children}
		return nil

	case yaml.MappingNode:
		// New nested format: {split, panes: [...]}
		var node LayoutNode
		if err := value.Decode(&node); err != nil {
			return fmt.Errorf("decoding nested layout: %w", err)
		}
		ls.Root = &node
		return nil

	default:
		return fmt.Errorf("layout must be a list or map, got %v", value.Kind)
	}
}

func (ls LayoutSpec) MarshalYAML() (interface{}, error) {
	if ls.Root == nil {
		return nil, nil
	}
	// If all children are leaves and there's no nested split, marshal as flat list for cleaner output
	if ls.Root.Cmd == "" && allLeavesFlat(ls.Root) {
		var panes []Pane
		for _, child := range ls.Root.Panes {
			panes = append(panes, Pane{Cmd: child.Cmd, Size: child.Size})
		}
		return panes, nil
	}
	return ls.Root, nil
}

func allLeavesFlat(n *LayoutNode) bool {
	for _, child := range n.Panes {
		if !child.IsLeaf() {
			return false
		}
	}
	return true
}

type Project struct {
	Path   string      `yaml:"path"`
	Name   string      `yaml:"name"`
	Layout *LayoutSpec `yaml:"layout,omitempty"`
	Split  string      `yaml:"split,omitempty"`
}

type Defaults struct {
	Layout *LayoutSpec `yaml:"layout"`
	Split  string      `yaml:"split"`
}

type Config struct {
	Workspace   string    `yaml:"workspace,omitempty"`
	Coordinator bool      `yaml:"coordinator,omitempty"`
	Defaults    Defaults  `yaml:"defaults"`
	Projects    []Project `yaml:"projects"`
}

var DefaultLayout = &LayoutNode{
	Split: "horizontal",
	Panes: []*LayoutNode{
		{Cmd: "claude", Size: "60%"},
		{Cmd: "shell", Size: "20%"},
		{Cmd: "shell", Size: "20%"},
	},
}

const DefaultSplit = "horizontal"

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "kage", "config.yaml")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefault(path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

func createDefault(path string) (*Config, error) {
	cfg := &Config{
		Defaults: Defaults{
			Layout: &LayoutSpec{Root: DefaultLayout},
			Split:  DefaultSplit,
		},
		Projects: []Project{},
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling default config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing default config: %w", err)
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Defaults.Layout == nil || cfg.Defaults.Layout.Root == nil {
		cfg.Defaults.Layout = &LayoutSpec{Root: DefaultLayout}
	}
	if cfg.Defaults.Split == "" {
		cfg.Defaults.Split = DefaultSplit
	}
	// Fill in the split direction on layout roots that don't have one (from flat format)
	if cfg.Defaults.Layout.Root.Split == "" && !cfg.Defaults.Layout.Root.IsLeaf() {
		cfg.Defaults.Layout.Root.Split = cfg.Defaults.Split
	}
	for i := range cfg.Projects {
		if cfg.Projects[i].Split == "" {
			cfg.Projects[i].Split = cfg.Defaults.Split
		}
		if cfg.Projects[i].Layout != nil && cfg.Projects[i].Layout.Root != nil {
			if cfg.Projects[i].Layout.Root.Split == "" && !cfg.Projects[i].Layout.Root.IsLeaf() {
				cfg.Projects[i].Layout.Root.Split = cfg.Projects[i].Split
			}
		}
	}
}

// EffectiveWorkspace returns the resolved workspace directory.
// Defaults to the user's home directory if not set.
func (c *Config) EffectiveWorkspace() string {
	if c.Workspace == "" {
		home, _ := os.UserHomeDir()
		return home
	}
	path := c.Workspace
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return path
}

// EffectiveLayout returns the layout tree for a project, falling back to defaults.
func (p *Project) EffectiveLayout(defaults Defaults) *LayoutNode {
	if p.Layout != nil && p.Layout.Root != nil {
		return p.Layout.Root
	}
	return defaults.Layout.Root
}

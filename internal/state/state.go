package state

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// State holds persistent metadata that is not part of the config.
type State struct {
	Descriptions map[string]string `yaml:"descriptions,omitempty"`

	path string `yaml:"-"`
}

// DefaultStatePath returns the default state file path.
func DefaultStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "kage", "state.yaml")
}

// Load reads state from the given path. Returns empty state if file doesn't exist.
func Load(path string) (*State, error) {
	s := &State{
		Descriptions: make(map[string]string),
		path:         path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}

	if err := yaml.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	if s.Descriptions == nil {
		s.Descriptions = make(map[string]string)
	}
	s.path = path
	return s, nil
}

// Save writes the state to its file path.
func (s *State) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	return os.WriteFile(s.path, data, 0o644)
}

// DescriptionKey returns the key used for storing a feature's description.
func DescriptionKey(projectName, branch string) string {
	return projectName + "/" + branch
}

// GetDescription returns the description for the given key.
func (s *State) GetDescription(key string) string {
	return s.Descriptions[key]
}

// SetDescription sets or updates a description.
func (s *State) SetDescription(key, desc string) {
	if desc == "" {
		delete(s.Descriptions, key)
	} else {
		s.Descriptions[key] = desc
	}
}

// DeleteDescription removes a description entry.
func (s *State) DeleteDescription(key string) {
	delete(s.Descriptions, key)
}

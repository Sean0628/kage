package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run executes a tmux command and returns its output.
func Run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunSilent executes a tmux command without capturing output.
func RunSilent(args ...string) error {
	_, err := Run(args...)
	return err
}

// InsideTmux returns true if the current process is inside a tmux session.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

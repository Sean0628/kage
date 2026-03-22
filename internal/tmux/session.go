package tmux

import (
	"os"
	"os/exec"
	"syscall"
)

const SessionName = "kage"

// HasSession checks if the kage tmux session exists.
func HasSession() bool {
	err := RunSilent("has-session", "-t", SessionName)
	return err == nil
}

// NewSession creates a new tmux session named "kage" in detached mode.
// The first window is named "dashboard".
// If startDir is non-empty, the session starts in that directory.
func NewSession(startDir string) error {
	args := []string{"new-session", "-d", "-s", SessionName, "-n", "dashboard"}
	if startDir != "" {
		args = append(args, "-c", startDir)
	}
	return RunSilent(args...)
}

// AttachSession replaces the current process with tmux attach-session.
// This uses syscall.Exec so the Go process is replaced entirely.
func AttachSession() error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(tmuxPath, []string{"tmux", "attach-session", "-t", SessionName}, os.Environ())
}

// SwitchClient switches the current tmux client to the kage session.
func SwitchClient() error {
	return RunSilent("switch-client", "-t", SessionName)
}

// SelectWindow switches to a specific window in the kage session.
func SelectWindow(window string) error {
	return RunSilent("select-window", "-t", SessionName+":"+window)
}

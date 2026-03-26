package project

import (
	"path/filepath"
	"strings"
)

// AgentStatus represents the inferred runtime state of an agent pane.
type AgentStatus string

const (
	AgentStatusUnknown           AgentStatus = "unknown"
	AgentStatusIdle              AgentStatus = "idle"
	AgentStatusRunning           AgentStatus = "running"
	AgentStatusWaitingInput      AgentStatus = "waiting_input"
	AgentStatusWaitingPermission AgentStatus = "waiting_permission"
)

var shellNames = map[string]struct{}{
	"":      {},
	"bash":  {},
	"fish":  {},
	"nu":    {},
	"sh":    {},
	"shell": {},
	"tmux":  {},
	"zsh":   {},
}

var knownAgentNames = map[string]struct{}{
	"aider":  {},
	"claude": {},
	"codex":  {},
}

func (s AgentStatus) Label() string {
	switch s {
	case AgentStatusIdle:
		return "idle"
	case AgentStatusRunning:
		return "running"
	case AgentStatusWaitingInput:
		return "waiting input"
	case AgentStatusWaitingPermission:
		return "waiting permission"
	default:
		return "unknown"
	}
}

func IsAgentPane(configCmd, currentProcess string) bool {
	cmdName := commandName(configCmd)
	processName := normalizeName(currentProcess)

	if isKnownAgentName(cmdName) || isKnownAgentName(processName) {
		return true
	}

	if cmdName == "" || isShellName(cmdName) {
		return false
	}

	return true
}

func AgentDisplayName(configCmd, currentProcess string) string {
	cmdName := commandName(configCmd)
	if cmdName != "" && !isShellName(cmdName) {
		return cmdName
	}
	processName := normalizeName(currentProcess)
	if processName != "" && !isShellName(processName) {
		return processName
	}
	return "agent"
}

func DetectAgentStatus(configCmd, currentProcess, output string) AgentStatus {
	normalized := strings.ToLower(output)
	if containsAny(normalized,
		"waiting for permission",
		"waiting on permission",
		"waiting for approval",
		"waiting on approval",
		"permission required",
		"approval required",
		"approve this action",
		"allow this action",
		"do you want to allow",
		"grant permission",
		"requires approval",
	) {
		return AgentStatusWaitingPermission
	}

	if containsAny(normalized,
		"waiting for input",
		"waiting for your input",
		"need your input",
		"awaiting your input",
		"please respond",
		"press enter to continue",
		"confirm to continue",
		"select an option",
		"choose an option",
	) {
		return AgentStatusWaitingInput
	}

	lastLine := lastNonEmptyLine(output)
	if looksIdlePrompt(lastLine) {
		return AgentStatusIdle
	}

	if strings.TrimSpace(output) == "" {
		if isKnownAgentName(commandName(configCmd)) || isKnownAgentName(normalizeName(currentProcess)) {
			return AgentStatusRunning
		}
		return AgentStatusUnknown
	}

	return AgentStatusRunning
}

func commandName(cmd string) string {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return ""
	}
	return normalizeName(fields[0])
}

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToLower(filepath.Base(name))
}

func isKnownAgentName(name string) bool {
	_, ok := knownAgentNames[name]
	return ok
}

func isShellName(name string) bool {
	_, ok := shellNames[name]
	return ok
}

func containsAny(s string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

func lastNonEmptyLine(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func looksIdlePrompt(line string) bool {
	if line == "" {
		return false
	}
	if line == ">" || strings.HasPrefix(line, "> ") || strings.HasSuffix(line, ">") {
		return true
	}
	if strings.HasSuffix(line, "$") || strings.HasSuffix(line, "%") || strings.HasSuffix(line, "#") {
		return true
	}
	return false
}

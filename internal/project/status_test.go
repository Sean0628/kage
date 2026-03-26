package project

import "testing"

func TestDetectAgentStatus(t *testing.T) {
	tests := []struct {
		name           string
		configCmd      string
		currentProcess string
		output         string
		want           AgentStatus
	}{
		{
			name:           "idle prompt",
			configCmd:      "claude",
			currentProcess: "claude",
			output:         "Ready\n>",
			want:           AgentStatusIdle,
		},
		{
			name:           "running output",
			configCmd:      "codex",
			currentProcess: "codex",
			output:         "Thinking through the change...\nUpdating files",
			want:           AgentStatusRunning,
		},
		{
			name:           "waiting input",
			configCmd:      "claude",
			currentProcess: "claude",
			output:         "I need your input before I continue.\nPlease respond with the preferred option.",
			want:           AgentStatusWaitingInput,
		},
		{
			name:           "waiting permission",
			configCmd:      "codex",
			currentProcess: "codex",
			output:         "Command requires approval.\nWaiting for permission to continue.",
			want:           AgentStatusWaitingPermission,
		},
		{
			name:           "unknown unsupported pane with no output",
			configCmd:      "custom-agent",
			currentProcess: "custom-agent",
			output:         "",
			want:           AgentStatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAgentStatus(tt.configCmd, tt.currentProcess, tt.output)
			if got != tt.want {
				t.Fatalf("DetectAgentStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsAgentPane(t *testing.T) {
	tests := []struct {
		name           string
		configCmd      string
		currentProcess string
		want           bool
	}{
		{
			name:           "shell pane is not agent",
			configCmd:      "shell",
			currentProcess: "zsh",
			want:           false,
		},
		{
			name:           "known agent command is agent",
			configCmd:      "claude",
			currentProcess: "claude",
			want:           true,
		},
		{
			name:           "generic non shell command treated as agent pane",
			configCmd:      "custom-agent",
			currentProcess: "custom-agent",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAgentPane(tt.configCmd, tt.currentProcess)
			if got != tt.want {
				t.Fatalf("IsAgentPane() = %v, want %v", got, tt.want)
			}
		})
	}
}

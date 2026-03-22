package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Sean0628/kage/internal/config"
	"github.com/Sean0628/kage/internal/project"
	"github.com/Sean0628/kage/internal/tmux"
)

func registerTools(s *server.MCPServer, cfg *config.Config) {
	s.AddTool(listProjectsTool(), listProjectsHandler(cfg))
	s.AddTool(listFeaturesTool(), listFeaturesHandler(cfg))
	s.AddTool(sendToAgentTool(), sendToAgentHandler(cfg))
	s.AddTool(broadcastToAgentsTool(), broadcastToAgentsHandler(cfg))
	s.AddTool(captureAgentOutputTool(), captureAgentOutputHandler(cfg))
	s.AddTool(getAgentStatusTool(), getAgentStatusHandler(cfg))
}

// --- list_projects ---

func listProjectsTool() gomcp.Tool {
	return gomcp.NewTool("list_projects",
		gomcp.WithDescription("List all configured projects with their features and live/inactive status"),
	)
}

type projectInfo struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Features []featureInfo `json:"features"`
}

type featureInfo struct {
	Branch string `json:"branch"`
	Status string `json:"status"`
	IsMain bool   `json:"is_main"`
}

func listProjectsHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		states := project.LoadAll(cfg)
		var projects []projectInfo
		for _, s := range states {
			p := projectInfo{
				Name: s.Config.Name,
				Path: s.Config.Path,
			}
			for _, f := range s.Features {
				status := "inactive"
				if f.Status == project.StatusLive {
					status = "live"
				}
				p.Features = append(p.Features, featureInfo{
					Branch: f.Branch,
					Status: status,
					IsMain: f.IsMain,
				})
			}
			projects = append(projects, p)
		}
		data, _ := json.MarshalIndent(projects, "", "  ")
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- list_features ---

func listFeaturesTool() gomcp.Tool {
	return gomcp.NewTool("list_features",
		gomcp.WithDescription("List features for a specific project with pane status"),
		gomcp.WithString("project", gomcp.Required(), gomcp.Description("Project name")),
	)
}

type featureDetail struct {
	Branch string       `json:"branch"`
	Status string       `json:"status"`
	IsMain bool         `json:"is_main"`
	Panes  []paneDetail `json:"panes,omitempty"`
}

type paneDetail struct {
	ConfigCmd      string `json:"config_cmd"`
	CurrentProcess string `json:"current_process"`
}

func listFeaturesHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		projName, err := req.RequireString("project")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: project"), nil
		}

		proj, ok := findProject(cfg, projName)
		if !ok {
			return gomcp.NewToolResultErrorf("project %q not found", projName), nil
		}

		state := project.LoadProject(cfg, proj)
		var features []featureDetail
		for _, f := range state.Features {
			status := "inactive"
			if f.Status == project.StatusLive {
				status = "live"
			}
			fd := featureDetail{
				Branch: f.Branch,
				Status: status,
				IsMain: f.IsMain,
			}
			for _, p := range f.Panes {
				fd.Panes = append(fd.Panes, paneDetail{
					ConfigCmd:      p.ConfigCmd,
					CurrentProcess: p.CurrentProcess,
				})
			}
			features = append(features, fd)
		}

		data, _ := json.MarshalIndent(features, "", "  ")
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- send_to_agent ---

func sendToAgentTool() gomcp.Tool {
	return gomcp.NewTool("send_to_agent",
		gomcp.WithDescription("Send a message to a specific feature's Claude Code pane via tmux"),
		gomcp.WithString("project", gomcp.Required(), gomcp.Description("Project name")),
		gomcp.WithString("branch", gomcp.Required(), gomcp.Description("Feature branch name")),
		gomcp.WithString("message", gomcp.Required(), gomcp.Description("Message to send to the Claude Code instance")),
	)
}

func sendToAgentHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		projName, err := req.RequireString("project")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: project"), nil
		}
		branch, err := req.RequireString("branch")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: branch"), nil
		}
		message, err := req.RequireString("message")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: message"), nil
		}

		target, err := resolveClaudePane(cfg, projName, branch)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		// Send message as literal text, then press Enter
		if err := tmux.SendKeysLiteral(target, message); err != nil {
			return gomcp.NewToolResultErrorf("failed to send keys: %v", err), nil
		}
		if err := tmux.RunSilent("send-keys", "-t", target, "Enter"); err != nil {
			return gomcp.NewToolResultErrorf("failed to send Enter: %v", err), nil
		}

		return gomcp.NewToolResultText(fmt.Sprintf("Message sent to %s/%s", projName, branch)), nil
	}
}

// --- broadcast_to_agents ---

func broadcastToAgentsTool() gomcp.Tool {
	return gomcp.NewTool("broadcast_to_agents",
		gomcp.WithDescription("Send a message to all live Claude Code panes, optionally filtered by project"),
		gomcp.WithString("message", gomcp.Required(), gomcp.Description("Message to send to all Claude Code instances")),
		gomcp.WithString("project", gomcp.Required(), gomcp.Description("Project name to filter by, or empty string for all projects")),
	)
}

type broadcastResult struct {
	Target string `json:"target"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func broadcastToAgentsHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		message, err := req.RequireString("message")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: message"), nil
		}
		projFilter, _ := req.RequireString("project")

		states := project.LoadAll(cfg)
		var results []broadcastResult
		for _, s := range states {
			if projFilter != "" && s.Config.Name != projFilter {
				continue
			}
			for _, f := range s.Features {
				if f.Status != project.StatusLive {
					continue
				}
				target, err := resolveClaudePane(cfg, s.Config.Name, f.Branch)
				if err != nil {
					results = append(results, broadcastResult{
						Target: fmt.Sprintf("%s/%s", s.Config.Name, f.Branch),
						Status: "skipped",
						Error:  err.Error(),
					})
					continue
				}

				sendErr := tmux.SendKeysLiteral(target, message)
				if sendErr == nil {
					sendErr = tmux.RunSilent("send-keys", "-t", target, "Enter")
				}
				if sendErr != nil {
					results = append(results, broadcastResult{
						Target: fmt.Sprintf("%s/%s", s.Config.Name, f.Branch),
						Status: "error",
						Error:  sendErr.Error(),
					})
				} else {
					results = append(results, broadcastResult{
						Target: fmt.Sprintf("%s/%s", s.Config.Name, f.Branch),
						Status: "sent",
					})
				}
			}
		}

		data, _ := json.MarshalIndent(results, "", "  ")
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- capture_agent_output ---

func captureAgentOutputTool() gomcp.Tool {
	return gomcp.NewTool("capture_agent_output",
		gomcp.WithDescription("Capture recent visible output from a feature's Claude Code pane"),
		gomcp.WithString("project", gomcp.Required(), gomcp.Description("Project name")),
		gomcp.WithString("branch", gomcp.Required(), gomcp.Description("Feature branch name")),
		gomcp.WithNumber("lines", gomcp.Description("Number of lines to capture (default: 50)")),
	)
}

func captureAgentOutputHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		projName, err := req.RequireString("project")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: project"), nil
		}
		branch, err := req.RequireString("branch")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: branch"), nil
		}
		lines := req.GetInt("lines", 50)

		target, err := resolveClaudePane(cfg, projName, branch)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		output, err := tmux.CapturePane(target, lines)
		if err != nil {
			return gomcp.NewToolResultErrorf("failed to capture pane: %v", err), nil
		}

		return gomcp.NewToolResultText(output), nil
	}
}

// --- get_agent_status ---

func getAgentStatusTool() gomcp.Tool {
	return gomcp.NewTool("get_agent_status",
		gomcp.WithDescription("Check if a Claude Code pane is idle (at prompt) or busy (mid-execution)"),
		gomcp.WithString("project", gomcp.Required(), gomcp.Description("Project name")),
		gomcp.WithString("branch", gomcp.Required(), gomcp.Description("Feature branch name")),
	)
}

type agentStatus struct {
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Status  string `json:"status"`
	Last5   string `json:"last_5_lines"`
}

func getAgentStatusHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		projName, err := req.RequireString("project")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: project"), nil
		}
		branch, err := req.RequireString("branch")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: branch"), nil
		}

		target, err := resolveClaudePane(cfg, projName, branch)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		output, err := tmux.CapturePane(target, 5)
		if err != nil {
			return gomcp.NewToolResultErrorf("failed to capture pane: %v", err), nil
		}

		status := detectAgentStatus(output)
		result := agentStatus{
			Project: projName,
			Branch:  branch,
			Status:  status,
			Last5:   output,
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- helpers ---

func findProject(cfg *config.Config, name string) (config.Project, bool) {
	for _, p := range cfg.Projects {
		if p.Name == name {
			return p, true
		}
	}
	return config.Project{}, false
}

// resolveClaudePane finds the tmux pane target for the Claude Code pane in a feature window.
func resolveClaudePane(cfg *config.Config, projName, branch string) (string, error) {
	proj, ok := findProject(cfg, projName)
	if !ok {
		return "", fmt.Errorf("project %q not found", projName)
	}

	windowName := project.FeatureWindowName(projName, branch)

	// Find the window index
	windows, err := tmux.ListWindows()
	if err != nil {
		return "", fmt.Errorf("listing windows: %w", err)
	}

	var windowIndex string
	for _, w := range windows {
		if w.Name == windowName {
			windowIndex = w.Index
			break
		}
	}
	if windowIndex == "" {
		return "", fmt.Errorf("no live window for %s/%s", projName, branch)
	}

	windowTarget := fmt.Sprintf("%s:%s", tmux.SessionName, windowIndex)

	// Find the Claude pane index from the layout config
	layout := proj.EffectiveLayout(cfg.Defaults)
	leaves := layout.Leaves()
	claudeIdx := -1
	for i, leaf := range leaves {
		if leaf.Cmd == "claude" {
			claudeIdx = i
			break
		}
	}
	if claudeIdx < 0 {
		return "", fmt.Errorf("no claude pane configured for project %q", projName)
	}

	// Get actual pane list and map to the claude index
	panes, err := tmux.ListPanes(windowTarget)
	if err != nil {
		return "", fmt.Errorf("listing panes: %w", err)
	}
	if claudeIdx >= len(panes) {
		return "", fmt.Errorf("claude pane index %d out of range (have %d panes)", claudeIdx, len(panes))
	}

	return fmt.Sprintf("%s.%d", windowTarget, panes[claudeIdx].Index), nil
}

// detectAgentStatus infers whether Claude Code is idle or busy from captured output.
func detectAgentStatus(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return "unknown"
	}
	lastLine := strings.TrimSpace(lines[len(lines)-1])

	// Claude Code shows ">" prompt when idle
	if lastLine == ">" || strings.HasSuffix(lastLine, "> ") || strings.HasPrefix(lastLine, "> ") {
		return "idle"
	}
	// Shell prompt indicators
	if strings.HasSuffix(lastLine, "$ ") || strings.HasSuffix(lastLine, "% ") {
		return "idle"
	}
	return "busy"
}

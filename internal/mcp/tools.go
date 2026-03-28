package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Sean0628/kage/internal/config"
	"github.com/Sean0628/kage/internal/project"
	"github.com/Sean0628/kage/internal/state"
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
	ID          int    `json:"id"`
	Branch      string `json:"branch"`
	Status      string `json:"status"`
	IsMain      bool   `json:"is_main"`
	Description string `json:"description,omitempty"`
}

func listProjectsHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		st, _ := state.Load(state.DefaultStatePath())
		states := project.LoadAll(cfg, st)
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
					ID:          f.ID,
					Branch:      f.Branch,
					Status:      status,
					IsMain:      f.IsMain,
					Description: f.Description,
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
	ID          int          `json:"id"`
	Branch      string       `json:"branch"`
	Status      string       `json:"status"`
	IsMain      bool         `json:"is_main"`
	Description string       `json:"description,omitempty"`
	Panes       []paneDetail `json:"panes,omitempty"`
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

		st, _ := state.Load(state.DefaultStatePath())
		states := project.LoadAll(cfg, st)
		var features []featureDetail
		found := false
		for _, ps := range states {
			if ps.Config.Name != projName {
				continue
			}
			found = true
			for _, f := range ps.Features {
				status := "inactive"
				if f.Status == project.StatusLive {
					status = "live"
				}
				fd := featureDetail{
					ID:          f.ID,
					Branch:      f.Branch,
					Status:      status,
					IsMain:      f.IsMain,
					Description: f.Description,
				}
				for _, p := range f.Panes {
					fd.Panes = append(fd.Panes, paneDetail{
						ConfigCmd:      p.ConfigCmd,
						CurrentProcess: p.CurrentProcess,
					})
				}
				features = append(features, fd)
			}
			break
		}
		if !found {
			return gomcp.NewToolResultErrorf("project %q not found", projName), nil
		}

		data, _ := json.MarshalIndent(features, "", "  ")
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- send_to_agent ---

func sendToAgentTool() gomcp.Tool {
	opts := []gomcp.ToolOption{
		gomcp.WithDescription("Send a message to a specific feature's Claude Code pane via tmux"),
	}
	opts = append(opts, featureTargetOptions()...)
	opts = append(opts, gomcp.WithString("message", gomcp.Required(), gomcp.Description("Message to send to the Claude Code instance")))
	return gomcp.NewTool("send_to_agent", opts...)
}

func sendToAgentHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		targetRef, err := resolveFeatureTarget(cfg, req)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		message, err := req.RequireString("message")
		if err != nil {
			return gomcp.NewToolResultError("missing required parameter: message"), nil
		}

		target, err := resolveClaudePane(cfg, targetRef.Project, targetRef.Branch)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		if err := tmux.SendKeysLiteral(target, message); err != nil {
			return gomcp.NewToolResultErrorf("failed to send keys: %v", err), nil
		}
		if err := tmux.RunSilent("send-keys", "-t", target, "Enter"); err != nil {
			return gomcp.NewToolResultErrorf("failed to send Enter: %v", err), nil
		}

		return gomcp.NewToolResultText(fmt.Sprintf("Message sent to %s", formatFeatureTarget(targetRef))), nil
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

		st, _ := state.Load(state.DefaultStatePath())
		states := project.LoadAll(cfg, st)
		var results []broadcastResult
		for _, s := range states {
			if projFilter != "" && s.Config.Name != projFilter {
				continue
			}
			for _, f := range s.Features {
				if f.Status != project.StatusLive {
					continue
				}
				ref := featureTarget{
					ID:      f.ID,
					Project: s.Config.Name,
					Branch:  f.Branch,
				}
				target, err := resolveClaudePane(cfg, ref.Project, ref.Branch)
				if err != nil {
					results = append(results, broadcastResult{
						Target: formatFeatureTarget(ref),
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
						Target: formatFeatureTarget(ref),
						Status: "error",
						Error:  sendErr.Error(),
					})
				} else {
					results = append(results, broadcastResult{
						Target: formatFeatureTarget(ref),
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
	opts := []gomcp.ToolOption{
		gomcp.WithDescription("Capture recent visible output from a feature's Claude Code pane"),
	}
	opts = append(opts, featureTargetOptions()...)
	opts = append(opts, gomcp.WithNumber("lines", gomcp.Description("Number of lines to capture (default: 50)")))
	return gomcp.NewTool("capture_agent_output", opts...)
}

func captureAgentOutputHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		targetRef, err := resolveFeatureTarget(cfg, req)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		lines := req.GetInt("lines", 50)

		target, err := resolveClaudePane(cfg, targetRef.Project, targetRef.Branch)
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
	opts := []gomcp.ToolOption{
		gomcp.WithDescription("Check if a Claude Code pane is idle (at prompt) or busy (mid-execution)"),
	}
	opts = append(opts, featureTargetOptions()...)
	return gomcp.NewTool("get_agent_status", opts...)
}

type agentStatus struct {
	ID      int    `json:"id,omitempty"`
	Project string `json:"project"`
	Branch  string `json:"branch"`
	Status  string `json:"status"`
	Last5   string `json:"last_5_lines"`
}

func getAgentStatusHandler(cfg *config.Config) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		targetRef, err := resolveFeatureTarget(cfg, req)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		target, err := resolveClaudePane(cfg, targetRef.Project, targetRef.Branch)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}

		output, err := tmux.CapturePane(target, 5)
		if err != nil {
			return gomcp.NewToolResultErrorf("failed to capture pane: %v", err), nil
		}

		status := string(project.DetectAgentStatus("claude", "claude", output))
		result := agentStatus{
			ID:      targetRef.ID,
			Project: targetRef.Project,
			Branch:  targetRef.Branch,
			Status:  status,
			Last5:   output,
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		return gomcp.NewToolResultText(string(data)), nil
	}
}

// --- helpers ---

type featureTarget struct {
	ID      int
	Project string
	Branch  string
}

func featureTargetOptions() []gomcp.ToolOption {
	return []gomcp.ToolOption{
		gomcp.WithNumber("id", gomcp.Description("Global feature ID from list_projects/list_features; optional alternative to project + branch")),
		gomcp.WithString("project", gomcp.Description("Project name; required when id is omitted")),
		gomcp.WithString("branch", gomcp.Description("Feature branch name; required when id is omitted")),
	}
}

func resolveFeatureTarget(cfg *config.Config, req gomcp.CallToolRequest) (featureTarget, error) {
	args := req.GetArguments()
	if _, ok := args["id"]; ok {
		id, err := req.RequireInt("id")
		if err != nil {
			return featureTarget{}, fmt.Errorf("invalid id: %w", err)
		}

		st, _ := state.Load(state.DefaultStatePath())
		states := project.LoadAll(cfg, st)
		for _, ps := range states {
			for _, f := range ps.Features {
				if f.ID == id {
					return featureTarget{
						ID:      f.ID,
						Project: ps.Config.Name,
						Branch:  f.Branch,
					}, nil
				}
			}
		}
		return featureTarget{}, fmt.Errorf("feature id %d not found", id)
	}

	projName, err := req.RequireString("project")
	if err != nil {
		return featureTarget{}, fmt.Errorf("missing required parameter: project or id")
	}
	branch, err := req.RequireString("branch")
	if err != nil {
		return featureTarget{}, fmt.Errorf("missing required parameter: branch")
	}
	return featureTarget{
		Project: projName,
		Branch:  branch,
	}, nil
}

func formatFeatureTarget(target featureTarget) string {
	if target.ID > 0 {
		return fmt.Sprintf("#%d %s/%s", target.ID, target.Project, target.Branch)
	}
	return fmt.Sprintf("%s/%s", target.Project, target.Branch)
}

func findProject(cfg *config.Config, name string) (config.Project, bool) {
	for _, p := range cfg.Projects {
		if p.Name == name {
			return p, true
		}
	}
	return config.Project{}, false
}

func resolveClaudePane(cfg *config.Config, projName, branch string) (string, error) {
	proj, ok := findProject(cfg, projName)
	if !ok {
		return "", fmt.Errorf("project %q not found", projName)
	}

	windowName := project.FeatureWindowName(projName, branch)

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

	panes, err := tmux.ListPanes(windowTarget)
	if err != nil {
		return "", fmt.Errorf("listing panes: %w", err)
	}
	if claudeIdx >= len(panes) {
		return "", fmt.Errorf("claude pane index %d out of range (have %d panes)", claudeIdx, len(panes))
	}

	return fmt.Sprintf("%s.%d", windowTarget, panes[claudeIdx].Index), nil
}

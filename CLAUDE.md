# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is kage?

kage (影) is a CLI tool that manages multiple AI coding agent worktree sessions via tmux. It provides a Bubble Tea TUI dashboard to create, launch, switch between, and delete git worktree-based feature branches, each running in its own tmux window with a configurable pane layout. It works with any CLI-based coding agent (Claude Code, Codex, Aider, etc.).

## Build & Run

```bash
go build -o bin/kage .        # build binary
go test ./...                  # run all tests
go test ./internal/worktree/   # run tests for a single package
go vet ./...                   # lint
```

## Architecture

The app follows a standard cobra CLI + Bubble Tea TUI pattern:

- **`cmd/`** — Cobra commands. `root.go` handles tmux session bootstrap (create/attach/switch logic). `dash.go` launches the TUI.
- **`internal/config/`** — YAML config from `~/.config/kage/config.yaml`. Defines projects (repo path, name) and pane layouts with defaults.
- **`internal/project/`** — Core orchestration. Merges worktree state with tmux window state into `ProjectState`. `LaunchFeature` creates worktrees + tmux windows; `DeleteFeature` tears them down.
- **`internal/worktree/`** — Git worktree operations (list/add/remove). Worktree paths follow the convention `<repo-parent>/<repo-name>-<branch>`.
- **`internal/tmux/`** — tmux wrapper. `session.go` manages the "kage" session. `window.go` handles window/pane creation, splits, and layout setup including relative split-size calculation.
- **`internal/tui/`** — Bubble Tea model with three modes: Normal (navigate/launch), NewBranch (text input), ConfirmDelete. Auto-refreshes state every 2 seconds.

## Key design details

- The tmux session is always named `"kage"`. Windows are named `<project>/<branch>`.
- `AttachSession` uses `syscall.Exec` to replace the Go process entirely.
- Pane layout sizes are specified as absolute percentages in config but converted to relative tmux split percentages via `CalcRelativeSplitSizes`.
- The special command `"shell"` in layout config means "leave the pane as a plain shell" (no command sent).
- Layouts can be nested trees for grid-like pane arrangements. A node is either a leaf (`cmd` + `size`) or a branch (`split` + `panes`). The old flat list format is still supported for backward compatibility.

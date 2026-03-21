# kage (影)

[![CI](https://github.com/Sean0628/kage/actions/workflows/ci.yml/badge.svg)](https://github.com/Sean0628/kage/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/Sean0628/kage/blob/main/LICENSE)

A CLI tool that manages multiple AI coding agent worktree sessions via tmux.

**kage** (影, *shadow*) is inspired by **影分身 (Kage Bunshin / Shadow Clone)** — the technique of creating multiple clones of yourself, each working independently on a different task. With kage, you summon shadow clones of AI coding agents, each operating in its own git worktree and tmux window, working on separate features in parallel. You are the original; your agents are the clones.

```
                          ┌─────────────────────────────────┐
                          │         kage TUI Dashboard       │
                          │  ┌─────────────────────────────┐ │
                          │  │ ● app/feat-auth        [3p] │ │
                          │  │   app/feat-search      [3p] │ │
                          │  │   app/fix-login        [3p] │ │
                          │  │                             │ │
                          │  │ n:new  enter:jump  d:delete │ │
                          │  └─────────────────────────────┘ │
                          └──────────────┬──────────────────┘
                                         │
                      ┌──────────────────┼──────────────────┐
                      │                  │                  │
                      ▼                  ▼                  ▼
              ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
              │ tmux window  │  │ tmux window  │  │ tmux window  │
              │ feat-auth    │  │ feat-search  │  │ fix-login    │
              │┌────────────┐│  │┌────────────┐│  │┌────────────┐│
              ││ Claude Code ││  ││   Aider    ││  ││   Codex    ││
              ││   (60%)    ││  ││   (60%)    ││  ││   (60%)    ││
              │├────────────┤│  │├────────────┤│  │├────────────┤│
              ││ shell (20%)││  ││ shell (20%)││  ││ shell (20%)││
              │├────────────┤│  │├────────────┤│  │├────────────┤│
              ││ shell (20%)││  ││ shell (20%)││  ││ shell (20%)││
              │└────────────┘│  │└────────────┘│  │└────────────┘│
              └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
                     │                 │                 │
                     ▼                 ▼                 ▼
              ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
              │ git worktree │  │ git worktree │  │ git worktree │
              │ app-feat-auth│  │app-feat-search│ │ app-fix-login│
              └──────────────┘  └──────────────┘  └──────────────┘

         Each "shadow clone" works independently in its own
           isolated worktree — no conflicts, no interference.
```

## Features

- **Git worktree isolation** — Each feature branch runs in its own worktree, so agents never interfere with each other
- **tmux-based session management** — One tmux session, one window per feature, with customizable pane layouts
- **Bubble Tea TUI dashboard** — Create, launch, switch between, and delete worktree sessions interactively
- **Flexible pane layouts** — Configure horizontal/vertical splits with nested tree layouts or simple flat lists
- **Agent-agnostic** — Works with Claude Code, Codex, Aider, or any CLI tool
- **Coordinator mode** — Optionally launch a coordinator Claude Code pane on the dashboard with an MCP server for cross-agent orchestration

## Requirements

- Go 1.26+
- tmux
- Git

## Installation

```bash
go install github.com/shoito/kage@latest
```

Or build from source:

```bash
git clone https://github.com/shoito/kage.git
cd kage
go build -o bin/kage .
```

## Quick Start

1. Create a config file at `~/.config/kage/config.yaml`:

```yaml
defaults:
  layout:
    split: horizontal
    panes:
      - cmd: claude
        size: 60%
      - cmd: shell
        size: 20%
      - cmd: shell
        size: 20%

projects:
  - name: my-project
    path: /path/to/your/repo

# Optional: launch a coordinator Claude Code pane on the dashboard
# coordinator: true
```

2. Launch kage:

```bash
kage
```

This creates (or attaches to) a tmux session named `kage` and opens the TUI dashboard.

## Usage

### Dashboard Keybindings

| Key | Action |
|-----|--------|
| `↑`/`k` | Move up |
| `↓`/`j` | Move down |
| `n` | New feature branch |
| `enter` | Jump to window |
| `a` | Attach agent to existing worktree |
| `d` | Delete feature (worktree + window) |
| `r` | Refresh |
| `q` | Quit dashboard |

While in tmux, press `Ctrl+b K` to jump back to the dashboard.

### Configuration

The config file (`~/.config/kage/config.yaml`) defines your projects and pane layouts.

#### Nested Layout (tree-based)

```yaml
defaults:
  layout:
    split: horizontal
    panes:
      - cmd: claude
        size: 60%
      - split: vertical
        size: 40%
        panes:
          - cmd: shell
            size: 50%
          - cmd: shell
            size: 50%
```

#### Flat Layout (legacy)

```yaml
defaults:
  layout:
    - cmd: claude
      size: 60%
    - cmd: shell
      size: 20%
    - cmd: shell
      size: 20%
```

The special command `shell` leaves the pane as a plain shell with no command executed.

#### Multiple Projects

```yaml
projects:
  - name: frontend
    path: /path/to/frontend-repo
    layout:
      split: horizontal
      panes:
        - cmd: claude
          size: 70%
        - cmd: shell
          size: 30%

  - name: backend
    path: /path/to/backend-repo
```

Projects without a `layout` key inherit from `defaults`.

#### Coordinator Mode

When `coordinator: true` is set, kage splits the dashboard window and launches a Claude Code instance with the kage MCP server pre-configured. This coordinator can orchestrate work across all feature agents — listing projects, sending messages, capturing output, and checking agent status.

```yaml
coordinator: true
```

This is disabled by default.

#### Workspace Directory

You can set a default working directory for the kage tmux session with the `workspace` key. It defaults to your home directory if not set.

```yaml
workspace: ~/work
```

## How It Works

1. `kage` creates a tmux session named `kage`
2. When you create a new feature, it:
   - Creates a git worktree at `<repo-parent>/<repo-name>-<branch>`
   - Opens a tmux window named `<project>/<branch>`
   - Sets up panes according to your layout config
   - Launches the configured commands in each pane
3. The TUI dashboard shows all projects and their active feature branches
4. Deleting a feature removes both the tmux window and the git worktree

## Development

```bash
go build -o bin/kage .    # build
go test ./...             # run all tests
go vet ./...              # lint
```

## Copyright
Copyright (c) 2026 Sho Ito. See [LICENSE.md](https://github.com/Sean0628/kage/blob/main/LICENSE) for further details.

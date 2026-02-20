# cli-scheduler

A CLI scheduler for Claude Code automation. Run `claude -p` jobs on cron schedules with secure, predefined execution environments.

**Core idea:** Separate "what to think about" (prompt file) from "what you're allowed to do" (YAML config). The prompt is the only dynamic part — everything else is a declarative config.

## Features

- **Interactive TUI** — main menu with job management, daemon control, and guided wizards
- **Cron scheduling** with hot-reload (edit YAML, daemon picks it up)
- **Prompts as files** — version-controlled `.md` files, no YAML escaping
- **Effort control** — set thinking effort per job (low/medium/high/max)
- **Usage tracking** — per-job and total token usage, cost breakdown (input, output, cache read, cache write)
- **JSON output** — `claude -p` always runs with `--output-format json`; stdout logs are `.json` files
- **Claude Code skill** — `/schedule` command for managing jobs from within Claude Code
- **Cross-platform** — Linux, macOS, and Windows (systemd / Windows Service)
- **Execution logging** — SQLite audit trail with token usage and cost per run
- **Report summarization** — optional Telegram-style summary after each run
- **Context control** — choose which Claude Code memory sources to load per job

## Quick Start

```bash
# Build
go build -o build/scheduler ./cmd/scheduler/

# Open the interactive TUI (recommended)
./build/scheduler

# Or use individual commands directly
./build/scheduler add              # Create a job (interactive wizard)
./build/scheduler list             # List all jobs
./build/scheduler edit my-job      # Edit a job
./build/scheduler disable my-job   # Pause a job
./build/scheduler enable my-job    # Resume a job
./build/scheduler run my-job       # Execute immediately
./build/scheduler logs             # View execution history
./build/scheduler start            # Start the daemon
./build/scheduler status           # Check daemon + next runs
./build/scheduler stop             # Stop the daemon
./build/scheduler remove my-job    # Delete a job
./build/scheduler validate         # Validate all configs
```

> **Windows PowerShell:** Use `.\build\scheduler.exe` instead of `./build/scheduler`. The binary is not on PATH by default.

## Interactive TUI

Running `scheduler` with no arguments opens the main menu:

```
  CLI Scheduler — Claude Code Automation
  Schedule and manage automated Claude Code tasks

  Daemon: stopped    Jobs: 3 total, 2 enabled    Next: nightly-review in 8h30m

  What would you like to do?
  > Add new job             Create a new scheduled task
    Manage jobs             View, edit, enable/disable, or remove jobs
    Run job now             Execute a job immediately
    View logs               See execution history
    Daemon                  Start, stop, or check daemon status
    Exit
```

**Navigation:** Use arrow keys to move, Enter to select, Ctrl+C to cancel.

### Add Wizard

The wizard walks through 6 steps:
1. **Job name & working directory** — identifier and project root
2. **Context sources** — which Claude Code memory to load
3. **Schedule** — preset or custom cron expression
4. **Prompt** — multiline prompt editor (or use `/schedule add` for Claude-written prompts)
5. **Model & effort** — choose model and thinking effort level
6. **Timeout, max turns & summary** — execution limits and reporting

### Manage Jobs

Select a job to view its action menu: edit, run now, usage stats, disable/enable, or remove.

### Usage Tracking

Per-run breakdown with totals:

```
  Usage: nightly-review

  DATE                  STATUS    INPUT     OUTPUT   CACHE READ  CACHE WRITE       COST
  ------------------------------------------------------------------------------------------
  2026-02-20 02:00:05   success     23       960      153.8K        4.6K       $0.0259
  2026-02-19 02:00:03   success     16       497       66.3K       38.8K       $0.0576
  ------------------------------------------------------------------------------------------
  TOTAL (2 runs)                    39      1.5K      220.1K       43.3K       $0.0835
```

## Non-Interactive Mode

For scripting and the Claude Code skill (`/schedule add`), use `--non-interactive`:

```bash
./build/scheduler add --non-interactive \
  --name "nightly-review" \
  --schedule "0 2 * * *" \
  --prompt-content "Review all changed files for security vulnerabilities" \
  --working-dir "/path/to/project" \
  --model sonnet \
  --effort medium \
  --timeout 300 \
  --max-turns 20 \
  --summary \
  --context "project_memory"
```

### All Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Job name (lowercase, hyphens, underscores) |
| `--schedule` | (required) | Cron expression (5-field) |
| `--working-dir` | (required) | Project directory |
| `--prompt-file` | `<name>.md` | Prompt filename |
| `--prompt-content` | — | Prompt text (written to prompt file) |
| `--model` | `sonnet` | Claude model: `sonnet`, `opus`, `haiku` |
| `--effort` | (empty = high) | Effort level: `low`, `medium`, `high`, `max` |
| `--permission-mode` | `bypassPermissions` | Permission mode |
| `--max-budget` | `0` (unlimited) | Max cost in USD per run |
| `--max-turns` | `0` (unlimited) | Max agentic turns per run |
| `--timeout` | `300` | Wall-clock timeout in seconds |
| `--add-dir` | — | Additional directories (repeatable) |
| `--mcp-config` | — | MCP server config file path |
| `--summary` | `false` | Enable Telegram-style summary |
| `--context` | `all` | Context sources (see below) |

## Effort Levels

Control how much thinking effort Claude puts into each job. Mapped to the `CLAUDE_CODE_EFFORT_LEVEL` environment variable.

| Level | Token usage | Best for |
|-------|------------|----------|
| `low` | Minimal | Simple checks, linting, quick queries |
| `medium` | Balanced | Standard code generation, routine automation |
| `high` | Full (default) | Complex reasoning, difficult coding tasks |
| `max` | Maximum (**Opus only**) | Critical tasks requiring absolute best results |

Configure via the TUI wizard, YAML (`effort: medium`), or `--effort` flag.

## Daemon

The daemon runs your scheduled jobs automatically. It watches for config changes and hot-reloads without restart.

### Running in foreground

```bash
./build/scheduler start
```

Runs in your terminal. Press Ctrl+C to stop. See logs in real time.

### Installing as a system service

```bash
./build/scheduler start --install
```

Registers as a system service that starts on boot:
- **Windows:** Windows Service (visible in `services.msc`). Requires administrator.
- **Linux:** systemd unit. Requires root.

## Job Configuration

Each job is a YAML file in the schedules directory:

```yaml
id: "a1b2c3d4"
name: nightly-review
schedule: "0 2 * * *"
working_dir: /path/to/project
prompt_file: "nightly-review.md"
model: sonnet
effort: medium
permission_mode: bypassPermissions
max_turns: 20
timeout: 300
summary_enabled: true
disable_user_memory: true
disable_local_memory: true
disable_auto_memory: true
disable_skills: true
no_session_persistence: true
enabled: true
```

### YAML ↔ `claude -p` Flag Mapping

| YAML Field | `claude -p` equivalent | Notes |
|-----------|----------------------|-------|
| `model` | `--model` | sonnet/opus/haiku |
| `effort` | `CLAUDE_CODE_EFFORT_LEVEL` env | low/medium/high/max |
| `permission_mode` | `--permission-mode` | bypassPermissions for unattended |
| `max_budget_usd` | `--max-budget-usd` | 0 = unlimited |
| `max_turns` | `--max-turns` | 0 = unlimited |
| `prompt_file` | stdin | Prompt piped via stdin |
| `no_session_persistence` | `--no-session-persistence` | Always true |
| `summary_enabled` | Prompt injection | Appends summary directive |
| `disable_*_memory` | `--setting-sources` | Combined into sources list |
| `disable_auto_memory` | `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` | Environment variable |
| `disable_skills` | `--disable-slash-commands` | CLI flag |
| `add_dirs` | `--add-dir` (repeated) | Additional directories |
| `mcp_config` | `--mcp-config` | MCP server config |
| — | `--output-format json` | Always hardcoded |

## CLI Commands

| Command | Description |
|---------|-------------|
| `scheduler` | Open interactive TUI menu |
| `scheduler add` | Create a new job (wizard or `--non-interactive`) |
| `scheduler list` | List all jobs |
| `scheduler edit [name]` | Edit a job |
| `scheduler disable [name]` | Disable a job |
| `scheduler enable [name]` | Enable a job |
| `scheduler remove [name]` | Remove a job (`-f` to skip confirmation) |
| `scheduler run <name>` | Execute a job immediately (shows cost + tokens) |
| `scheduler logs [name]` | View execution history |
| `scheduler start` | Start the daemon (foreground) |
| `scheduler start --install` | Install as system service |
| `scheduler stop` | Stop the daemon |
| `scheduler status` | Show daemon status + next runs |
| `scheduler validate` | Validate all job configs |

## Context & Memory

Claude Code loads several types of context when running a job. Control which ones per job:

| Context Source | What It Contains | Recommendation |
|---|---|---|
| **Project memory** | `CLAUDE.md`, `.claude/rules/` | Enable for most jobs — project conventions |
| **User memory** | `~/.claude/CLAUDE.md` | Usually noise for scheduled jobs |
| **Local memory** | `CLAUDE.local.md` | Rarely needed |
| **Auto memory** | Auto-generated notes | May be stale |
| **Skills** | Slash commands | Not useful for automated execution |

**Recommended:** Enable project memory only (`--context "project_memory"`).

```bash
# Only project memory (recommended)
scheduler add --non-interactive --context "project_memory" ...

# No context (fully isolated)
scheduler add --non-interactive --context "none" ...

# All context (default)
scheduler add --non-interactive --context "all" ...
```

Every prompt automatically includes a preamble instructing Claude to execute autonomously.

## Report Summarization

When `summary_enabled: true`, a prompt injection is appended that instructs Claude to write a concise Telegram-style markdown summary after completing the task.

Summaries saved to the `summary/` directory: `<job-name>-<date>.md`.

```
**nightly-review** | 2026-02-20 02:00

🔍 Reviewed **12 changed files** across 3 packages
🛡️ Found **2 potential SQL injection** vulnerabilities in `handlers/user.go`
📁 No issues in test files

✅ **Success**
```

## Claude Code Integration

### Install the `/schedule` skill

The skill lives in `.agents/skills/schedule/SKILL.md` and gives Claude Code the `/schedule` command.

```bash
# Linux/macOS
mkdir -p ~/.claude/skills/schedule
cp .agents/skills/schedule/SKILL.md ~/.claude/skills/schedule/SKILL.md

# Windows (PowerShell)
New-Item -ItemType Directory -Force "$env:USERPROFILE\.claude\skills\schedule"
Copy-Item .agents\skills\schedule\SKILL.md "$env:USERPROFILE\.claude\skills\schedule\SKILL.md"

# Or use make
make install-skill
```

Then use `/schedule add` in Claude Code — Claude helps write high-quality prompts and handles all configuration.

### Project instructions (optional)

To give Claude Code context about your project when creating jobs, add a `CLAUDE.md` to your project root. See `AGENTS.md` for architecture details and agent configuration.

## Config Directory

```
~/.cli-scheduler/          (Linux/macOS, or $XDG_CONFIG_HOME/cli-scheduler)
%APPDATA%\cli-scheduler\   (Windows)
  ├── schedules/            # YAML job configs
  ├── prompts/              # Prompt files (.md)
  ├── system-prompts/       # Reusable system prompts (advanced)
  ├── logs/                 # stdout (.json) / stderr (.log) per execution
  ├── summary/              # Execution summaries (Telegram-style)
  ├── data/scheduler.db     # SQLite execution logs + token usage
  └── scheduler.pid         # Daemon PID file
```

## Requirements

- Go 1.21+
- Claude Code CLI (`claude`) installed and authenticated
- Works with API keys and Pro/Max/Team subscriptions

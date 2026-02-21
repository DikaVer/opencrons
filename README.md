# cli-scheduler

A CLI scheduler for Claude Code automation. Run `claude -p` jobs on cron schedules and chat with Claude from Telegram — all managed through an interactive TUI or command-line interface.

**Core idea:** Separate "what to think about" (prompt file) from "what you're allowed to do" (YAML config). The prompt is the only dynamic part — everything else is a declarative config.

## Features

- **Interactive TUI** — main menu with job management, settings, daemon control, and guided wizards
- **Cron scheduling** with hot-reload (edit YAML, daemon picks it up automatically)
- **Telegram bot** — chat with Claude and manage scheduled jobs from your phone
- **Persistent chat sessions** — continue conversations across messages via `--session-id`
- **First-run setup wizard** — provider detection, Telegram pairing, daemon configuration
- **Prompts as files** — version-controlled `.md` files, no YAML escaping
- **Effort control** — set thinking effort per job (low/medium/high/max)
- **Usage tracking** — per-job and total token usage, cost breakdown (input, output, cache read, cache write)
- **JSON output** — `claude -p` always runs with `--output-format json`; stdout logs are `.json` files
- **Claude Code skill** — `/schedule` command for managing jobs from within Claude Code
- **Cross-platform** — Linux, macOS, and Windows (systemd / Windows Service)
- **Execution logging** — SQLite audit trail with token usage and cost per run
- **Report summarization** — optional Telegram-style summary after each run

## Quick Start

### Build

```bash
go build -o build/scheduler ./cmd/scheduler/
```

On Windows, this produces `build/scheduler.exe`.

### First Run

```bash
./build/scheduler
```

The first time you run the scheduler, the setup wizard walks you through:

1. **Provider detection** — verifies Claude Code CLI is installed and authenticated
2. **Telegram integration** (optional) — connect a Telegram bot for remote access
3. **Chat defaults** — choose default model and effort level for chat sessions
4. **Daemon mode** — background process or system service

After setup, you'll land in the interactive TUI.

### Create a Job

```bash
# Interactive wizard (recommended)
./build/scheduler add

# Or non-interactive for scripting
./build/scheduler add --non-interactive \
  --name "nightly-review" \
  --schedule "0 2 * * *" \
  --prompt-content "Review all changed files for security vulnerabilities" \
  --working-dir "/path/to/project" \
  --model sonnet \
  --effort medium \
  --timeout 300
```

### Start the Daemon

```bash
# Foreground (Ctrl+C to stop)
./build/scheduler start

# Or install as a system service
./build/scheduler start --install
```

The daemon runs your scheduled jobs and the Telegram bot (if configured) in a single process.

> **Windows PowerShell:** Use `.\build\scheduler.exe` instead of `./build/scheduler`.

## Interactive TUI

Running `scheduler` with no arguments opens the main menu:

```
  CLI Scheduler — Claude Code Automation
  Schedule and manage automated Claude Code tasks

  Daemon: running    Jobs: 3 total, 2 enabled    Chat: telegram    Next: nightly-review in 8h30m

  What would you like to do?
  > Add new job             Create a new scheduled task
    Manage jobs             View, edit, enable/disable, or remove jobs
    Run job now             Execute a job immediately
    View logs               See execution history
    Daemon                  Start, stop, or check daemon status
    Settings                Manage provider, messenger, and preferences
    Exit
```

**Navigation:** Arrow keys to move, Enter to select, Ctrl+C to cancel.

### Add Wizard

The wizard walks through 6 steps:
1. **Job name & working directory** — identifier and project root
2. **Schedule** — preset or custom cron expression
3. **Prompt** — multiline prompt editor (or use `/schedule add` for Claude-written prompts)
4. **Model** — choose Claude model (sonnet, opus, haiku)
5. **Effort level** — thinking effort (low/medium/high/max)
6. **Timeout & summary** — execution limits and optional reporting

### Manage Jobs

Select a job to view its action menu: edit, run now, usage stats, disable/enable, or remove.

### Settings

Manage all configuration from the Settings menu:

```
  Settings

  Provider:   anthropic
  Messenger:  telegram (1 users)
  Chat:       sonnet / high
  Daemon:     background
  Debug:      off

  What would you like to change?
  > Provider               View/change AI provider
    Messenger              View/change Telegram settings
    Chat defaults          Change model and effort
    Daemon mode            Background vs. system service
    Debug logging          Toggle on/off
    Re-run setup           Start setup wizard again
    << Back
```

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

## Telegram Bot

Connect a Telegram bot to chat with Claude and manage your scheduled jobs from anywhere.

### Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) on Telegram
2. Run `scheduler setup` (or let the first-run wizard guide you)
3. Paste your bot token
4. Choose a pairing method:
   - **Pairing token** — the bot generates a code to verify your identity
   - **Allow list** — manually enter Telegram usernames or user IDs

### Bot Commands

| Command | Description |
|---------|-------------|
| `/new` | Start a fresh chat session (clears context) |
| `/jobs` | List scheduled jobs with inline buttons to enable/disable/run |
| `/model` | Switch Claude model (sonnet/opus/haiku) via inline keyboard |
| `/effort` | Switch effort level (low/medium/high/max) via inline keyboard |
| `/status` | Show daemon status, job count, active session info |
| `/help` | Show available commands |

Any other text message is sent to Claude as a chat message.

### Chat Sessions

- Each Telegram user gets a **persistent session** backed by Claude's `--session-id`
- Claude maintains conversation history internally — no prompt reconstruction needed
- Use `/new` to clear context and start fresh
- Sessions track model, effort level, and working directory independently
- All messages are logged to SQLite for visibility (terminal echo, `scheduler logs`)

### Job Management

Use `/jobs` to see your scheduled jobs as an interactive list:

```
Scheduled Jobs:
(+ enabled, - disabled)

[+] nightly-review (0 2 * * *)
[-] weekly-audit (0 9 * * 1)
```

Tap a job to see its details and action buttons: Enable/Disable, Run Now, or Back.

### How It Works

The Telegram bot runs **inside the daemon** alongside the cron scheduler. When you run `scheduler start`, both start together in a single process:

- No IPC needed — bot has direct access to the same DB, config, and executor
- Job completion notifications are sent to all authorized users
- Typing indicators refresh every 5 seconds while Claude is processing
- One message at a time per user — concurrent messages get a "please wait" reply

## CLI Commands

| Command | Description |
|---------|-------------|
| `scheduler` | Open interactive TUI menu |
| `scheduler setup` | Run (or re-run) the setup wizard |
| `scheduler settings` | Manage provider, messenger, chat, debug settings |
| `scheduler add` | Create a new job (wizard or `--non-interactive`) |
| `scheduler list` | List all jobs in a styled table |
| `scheduler edit [name]` | Edit a job's config and prompt |
| `scheduler disable [name]` | Disable a job (keeps config) |
| `scheduler enable [name]` | Enable a disabled job |
| `scheduler remove [name]` | Remove a job (`-f` to skip confirmation) |
| `scheduler run <name>` | Execute a job immediately (shows cost + tokens) |
| `scheduler logs [name]` | View execution history |
| `scheduler start` | Start the daemon (foreground, includes Telegram bot) |
| `scheduler start --install` | Install as a system service |
| `scheduler stop` | Stop the daemon |
| `scheduler status` | Show daemon status + next scheduled runs |
| `scheduler validate` | Validate all job configs |
| `scheduler debug [on\|off]` | Toggle debug logging |

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
  --summary
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Job name (lowercase, hyphens, underscores) |
| `--schedule` | (required) | Cron expression (5-field) |
| `--working-dir` | (required) | Project directory |
| `--prompt-file` | `<name>.md` | Prompt filename |
| `--prompt-content` | — | Prompt text (written to prompt file) |
| `--model` | `sonnet` | Claude model: `sonnet`, `opus`, `haiku` |
| `--effort` | (empty = high) | Effort level: `low`, `medium`, `high`, `max` |
| `--timeout` | `300` | Wall-clock timeout in seconds |
| `--summary` | `false` | Enable Telegram-style summary |

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
timeout: 300
summary_enabled: true
no_session_persistence: true
enabled: true
```

### YAML to `claude -p` Mapping

| YAML Field | `claude -p` Flag | Notes |
|-----------|-----------------|-------|
| `model` | `--model` | sonnet/opus/haiku |
| `effort` | `--effort` | low/medium/high/max |
| `prompt_file` | stdin | Prompt piped via stdin |
| `no_session_persistence` | `--no-session-persistence` | Always true for scheduled jobs |
| `summary_enabled` | Prompt injection | Appends summary directive |
| — | `--permission-mode bypassPermissions` | Always hardcoded |
| — | `--output-format json` | Always hardcoded |

## Effort Levels

Control how much thinking effort Claude puts into each job.

| Level | Token Usage | Best For |
|-------|------------|----------|
| `low` | Minimal | Simple checks, linting, quick queries |
| `medium` | Balanced | Standard code generation, routine automation |
| `high` | Full (default) | Complex reasoning, difficult coding tasks |
| `max` | Maximum (**Opus only**) | Critical tasks requiring absolute best results |

Set via the TUI wizard, YAML config (`effort: medium`), `--effort` flag, or Telegram `/effort` command.

## Daemon

The daemon runs your scheduled jobs and the Telegram bot automatically. It watches for config changes and hot-reloads without restart.

### Foreground

```bash
./build/scheduler start
```

Runs in your terminal. Press Ctrl+C to stop. See job execution and Telegram chat logs in real time.

### System Service

```bash
./build/scheduler start --install
```

Registers as a system service that starts on boot:
- **Windows:** Windows Service (visible in `services.msc`). Requires administrator.
- **Linux:** systemd unit. Requires root.

### Hot-Reload

Edit any YAML file in the schedules directory and the daemon picks it up within 500ms. No restart needed. The daemon uses fsnotify with a debounce timer and atomically re-registers all jobs.

## Report Summarization

When `summary_enabled: true`, a prompt injection is appended that instructs Claude to write a concise Telegram-style markdown summary after completing the task.

Summaries are saved to the `summary/` directory: `<job-name>-<date>.md`.

```
**nightly-review** | 2026-02-20 02:00

Reviewed **12 changed files** across 3 packages
Found **2 potential SQL injection** vulnerabilities in `handlers/user.go`
No issues in test files

**Success**
```

## Claude Code Integration

### Install the `/schedule` Skill

The skill lives in `.workspace/.agents/skills/schedule/SKILL.md` and gives Claude Code the `/schedule` command for creating and managing jobs with AI-assisted prompt writing.

```bash
# Linux/macOS
mkdir -p ~/.claude/skills/schedule
cp .workspace/.agents/skills/schedule/SKILL.md ~/.claude/skills/schedule/SKILL.md

# Windows (PowerShell)
New-Item -ItemType Directory -Force "$env:USERPROFILE\.claude\skills\schedule"
Copy-Item .workspace\.agents\skills\schedule\SKILL.md "$env:USERPROFILE\.claude\skills\schedule\SKILL.md"

# Or use make
make install-skill
```

Then use `/schedule add` in Claude Code — Claude helps write high-quality prompts and handles all configuration.

## Config Directory

```
~/.cli-scheduler/          (Linux/macOS, or $XDG_CONFIG_HOME/cli-scheduler)
%APPDATA%\cli-scheduler\   (Windows)
  ├── schedules/            # YAML job configs
  ├── prompts/              # Prompt files (.md)
  ├── logs/                 # stdout (.json) / stderr (.log) per execution
  ├── summary/              # Execution summaries
  ├── workspace/            # CLAUDE.md + .claude/ (copied during setup)
  ├── data/scheduler.db     # SQLite: execution logs, chat sessions, messages
  ├── settings.json         # All settings (provider, messenger, chat, daemon, debug)
  └── scheduler.pid         # Daemon PID file
```

## Architecture

```
cmd/scheduler/main.go
  └ internal/cmd/              Cobra commands + TUI menu loop
       ├ internal/config/       JobConfig struct, YAML load/save, prompt file I/O
       ├ internal/tui/          Menus, wizards, settings, validators (charmbracelet/huh)
       ├ internal/executor/     Builds `claude -p` command, runs it, parses JSON output
       ├ internal/storage/      SQLite: execution logs, chat sessions, messages
       ├ internal/daemon/       Cron orchestrator, fsnotify hot-reload, OS service, Telegram bot
       ├ internal/platform/     Cross-platform paths, PID, process detection, settings
       ├ internal/logger/       Debug logger (singleton, gated by settings.json)
       ├ internal/provider/     AI provider interface + Anthropic implementation
       ├ internal/messenger/    Messenger interface
       │    └ telegram/         Bot lifecycle, handlers, chat, pairing, inline keyboards
       └ internal/chat/         Chat session manager + Claude runner (--session-id)
```

### Three Modes of Operation

1. **Interactive TUI** — `scheduler` with no args opens the main menu
2. **CLI subcommands** — `scheduler add`, `scheduler list`, etc. for scripting
3. **Telegram bot** — runs inside the daemon alongside cron for remote access

### Platform Support

| Platform | PID Detection | Config Path |
|----------|--------------|-------------|
| Windows | `OpenProcess` | `%APPDATA%\cli-scheduler\` |
| Linux | `syscall.Signal(0)` | `~/.cli-scheduler/` or `$XDG_CONFIG_HOME/cli-scheduler/` |
| macOS | `syscall.Signal(0)` | `~/.cli-scheduler/` |

## Development

```bash
# Build
go build -o build/scheduler ./cmd/scheduler/
make build              # with version ldflags

# Cross-compile
make build-linux        # linux/amd64
make build-windows      # windows/amd64
make build-all          # both

# Test & lint
go test ./...
go build ./...          # compile check all packages
go vet ./...
make lint               # golangci-lint

# Tidy dependencies
make tidy
```

## Requirements

- Go 1.21+
- Claude Code CLI (`claude`) installed and authenticated
- Works with API keys and Pro/Max/Team subscriptions
- Telegram bot token (optional, for Telegram integration)

# AGENTS.md

Agent configuration and architecture guide for cli-scheduler.

## Configure with Claude Code

### 1. Install the `/schedule` skill

The skill gives Claude Code the `/schedule` command to create and manage jobs conversationally.

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

After installing, use `/schedule add` inside Claude Code. Claude will help write high-quality prompts and handle all configuration.

### 2. Add project instructions (optional)

If you want Claude Code to auto-load project context when working on this repo, create a `CLAUDE.md` at the project root. You can copy from this file or write your own.

Claude Code reads these files automatically:
- `CLAUDE.md` — project root instructions (loaded for all sessions in this directory)
- `.claude/rules/*.md` — additional project rules

### 3. Build the binary

```bash
go build -o build/scheduler ./cmd/scheduler/

# Or use make
make build
```

On Windows PowerShell: `.\build\scheduler.exe`

### 4. Verify installation

```bash
scheduler --help        # Show available commands
scheduler validate      # Validate all job configs
scheduler status        # Check daemon status
```

---

## Repo Structure

```
cli-scheduler/
├── .agents/
│   └── skills/
│       └── schedule/
│           └── SKILL.md          # Claude Code skill definition (/schedule command)
├── cmd/scheduler/main.go         # Entry point
├── internal/
│   ├── cmd/                      # Cobra commands + TUI menu loop
│   ├── config/                   # JobConfig struct, YAML load/save
│   ├── tui/                      # Interactive UI (menus, wizards, validators)
│   ├── executor/                 # Builds `claude -p` command, runs it, parses output
│   ├── storage/                  # SQLite execution log + token usage
│   ├── daemon/                   # Cron orchestrator, hot-reload, OS service
│   └── platform/                 # Cross-platform paths, PID management
├── prompts/                      # Reference prompt templates
├── AGENTS.md                     # This file
├── README.md                     # User documentation
├── Makefile                      # Build targets
├── go.mod / go.sum               # Go module
└── .gitignore
```

---

## Architecture

CLI scheduler that runs Claude Code (`claude -p`) jobs on cron schedules. Separates "what to think about" (prompt files) from "what you're allowed to do" (YAML config).

### Package dependency graph

```
cmd/scheduler/main.go
  └→ internal/cmd/          (Cobra commands + main TUI menu loop)
       ├→ internal/config/   (JobConfig struct, YAML load/save, prompt file I/O)
       ├→ internal/tui/      (Interactive UI: menus, wizards, validators)
       ├→ internal/executor/ (Builds `claude -p` command, runs it, parses JSON output)
       ├→ internal/storage/  (SQLite execution log + token usage)
       ├→ internal/daemon/   (Cron orchestrator, fsnotify hot-reload, OS service)
       └→ internal/platform/ (Cross-platform paths, PID management, process detection)
```

### Two modes of operation

1. **Interactive TUI** (`scheduler` with no args): `root.go:runMainMenu()` runs a loop → `tui.RunMainMenu()` → dispatches to handlers → returns to menu.

2. **CLI subcommands** (`scheduler add`, `scheduler list`, etc.): Each `internal/cmd/*.go` registers a Cobra command.

Shared logic in the `cmd` package as unexported functions so both modes reuse the same code.

### Key data flows

**Job creation:** `cmd/add.go` → TUI wizard or CLI flags → writes `prompts/<name>.md` + `schedules/<name>.yml`. Duplicate names validated in both paths.

**Job execution:** `executor.Run()` → timeout via `context.WithTimeout` → `BuildCommand(ctx)` reads prompt → prepends embedded task preamble → appends optional summary injection → pipes prompt via stdin to `claude -p [flags]` → sets `CLAUDE_CODE_EFFORT_LEVEL` env var → passes `--max-turns`, `--output-format json`, context flags → captures stdout/stderr to log files → parses JSON for cost/usage → writes to SQLite

**Daemon:** `daemon.Run()` → PID file → SQLite → loads configs → cron entries (`SkipIfStillRunning`) → fsnotify watcher → blocks on SIGINT/SIGTERM → graceful shutdown

**Hot-reload:** fsnotify → 500ms debounce → `Reload()` holds mutex for entire operation: clears and re-registers all jobs atomically

### Important defaults

| Field | Default | Notes |
|-------|---------|-------|
| `permission_mode` | `bypassPermissions` | Jobs run unattended |
| `timeout` | `300` | 5 minutes |
| `max_budget_usd` | `0` | Unlimited |
| `max_turns` | `0` | Unlimited |
| `effort` | (empty = high) | Claude Code default |
| `output_format` | `json` | Hardcoded, not configurable |
| `summary_enabled` | `false` | Optional Telegram-style summary |
| `no_session_persistence` | `true` | Hardcoded |
| Context | All enabled | `disable_*` flags control sources |

### Platform support

| Platform | PID detection | Config path |
|----------|--------------|-------------|
| **Windows** | `OpenProcess` (`lock_windows.go`) | `%APPDATA%\cli-scheduler\` |
| **Linux** | `syscall.Signal(0)` (`lock_unix.go`) | `~/.cli-scheduler/` or `$XDG_CONFIG_HOME/cli-scheduler/` |
| **macOS** | `syscall.Signal(0)` (`lock_unix.go`) | `~/.cli-scheduler/` |

### Runtime config directory

```
~/.cli-scheduler/          (Linux/macOS)
%APPDATA%\cli-scheduler\   (Windows)
  ├── schedules/            # One YAML per job
  ├── prompts/              # One .md per job (prompt content)
  ├── system-prompts/       # Reusable system prompts (advanced)
  ├── logs/                 # stdout (.json) / stderr (.log) per execution
  ├── summary/              # Execution summaries
  ├── data/scheduler.db     # SQLite (WAL mode)
  └── scheduler.pid         # Daemon lock file
```

### SQLite schema (execution_logs)

Core: `id`, `job_id`, `job_name`, `started_at`, `finished_at`, `exit_code`, `stdout_path`, `stderr_path`, `status`, `trigger_type`, `error_msg`

Usage: `cost_usd`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`

Usage columns auto-migrated via `ALTER TABLE ADD COLUMN` (idempotent).

### Concurrency model

- `sync.Mutex` protects daemon's job map during hot-reload (held for entire operation)
- `cron.SkipIfStillRunning` prevents overlapping execution of the same job
- SQLite WAL mode with 5s busy timeout
- Each job runs as isolated subprocess via `exec.CommandContext`
- `Watcher.Stop()` uses `sync.Once` to prevent double-close panic

---

## Build & Test

```bash
# Build
go build -o build/scheduler ./cmd/scheduler/

# Build all packages (compile check)
go build ./...

# Run tests
go test ./...

# Tidy dependencies
go mod tidy

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o build/scheduler-linux-amd64 ./cmd/scheduler/
GOOS=darwin GOARCH=arm64 go build -o build/scheduler-darwin-arm64 ./cmd/scheduler/
```

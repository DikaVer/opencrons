# opencron

OpenCron runs Claude Code (`claude -p`) jobs on cron schedules, with Telegram bot integration for remote chat and job management. Go + Cobra + charmbracelet TUI.

## Commands

```bash
# Build & test
go build -o build/opencron ./cmd/opencron/
go test ./...
go build ./...          # compile check all packages
make build              # build with version ldflags
make lint               # golangci-lint

# CLI subcommands
opencron                # interactive TUI menu
opencron setup          # run (or re-run) the setup wizard
opencron settings       # manage provider, messenger, chat, debug settings
opencron add            # create a new job (wizard or flags)
opencron list           # list all jobs
opencron run <name>     # execute a job immediately
opencron edit <name>    # edit job config
opencron enable <name>  # enable a job
opencron disable <name> # disable a job
opencron remove <name>  # delete job + prompt file
opencron logs [name]    # view execution logs
opencron start          # start daemon (foreground, includes Telegram bot)
opencron stop           # stop running daemon
opencron status         # check daemon status
opencron validate       # validate all job configs
opencron debug [on|off] # toggle debug logging
```

## Architecture

### Package dependency graph

```
cmd/opencron/main.go
  └→ internal/cmd/              Cobra commands + TUI menu loop
       ├→ internal/config/       JobConfig struct, YAML load/save, prompt file I/O
       ├→ internal/tui/          Interactive UI: menus, wizards, settings, validators (charmbracelet/huh)
       ├→ internal/executor/     Builds `claude -p` command, runs it, parses JSON output
       ├→ internal/storage/      SQLite execution log + token usage + chat sessions (modernc.org/sqlite)
       ├→ internal/daemon/       Cron orchestrator, fsnotify hot-reload, OS service, Telegram bot
       ├→ internal/platform/     Cross-platform paths, PID management, process detection, settings
       ├→ internal/logger/       Debug logger (singleton, gated by platform.IsDebugEnabled())
       ├→ internal/provider/     AI provider interface + Anthropic implementation
       ├→ internal/messenger/    Messenger interface
       │    └→ telegram/         Telegram bot: handlers, chat, pairing, inline keyboards
       └→ internal/chat/         Chat session manager + Claude runner (--session-id)
```

### Three modes of operation

1. **Interactive TUI** (`opencron` with no args): `root.go:runMainMenu()` loops → `tui.RunMainMenu()` → dispatches to handlers → returns to menu.
2. **CLI subcommands** (`opencron add`, `opencron list`, etc.): Each `internal/cmd/*.go` registers a Cobra command.
3. **Telegram bot** (inside daemon): Runs alongside cron in `opencron start`. Handles `/new`, `/jobs`, `/model`, `/effort`, `/status`, `/help` commands + free-text chat with Claude.

Shared logic lives in the `cmd` package as unexported functions so both modes reuse the same code.

### Key data flows

**First-run setup:** `rootCmd.PersistentPreRunE` checks `platform.IsSetupComplete()` → if false, runs `runSetupWizard()` → detects provider → configures messenger/chat/daemon → copies `.workspace/` to config dir → saves `settings.json`.

**Job creation:** `cmd/add.go` → TUI wizard or CLI flags → writes `prompts/<name>.md` + `schedules/<name>.yml`. Duplicate names validated in both paths.

**Job execution:** `executor.Run()` → timeout via `context.WithTimeout` → `BuildCommand(ctx)` reads prompt → prepends embedded `task-preamble.txt` → appends optional `summary-prompt.txt` injection → pipes prompt via stdin to `claude -p [flags]` → passes `--effort`, `--permission-mode bypassPermissions`, `--output-format json` → captures stdout/stderr to log files → parses JSON for cost/usage → writes to SQLite.

**Daemon:** `daemon.Run()` → PID file → SQLite → loads configs → cron entries (`SkipIfStillRunning`) → starts Telegram bot (if configured) → fsnotify watcher → blocks on SIGINT/SIGTERM → stops bot → stops cron → graceful shutdown.

**Telegram chat:** User sends text → `handleChatMessage()` → per-user lock (prevents concurrent processing) → `sessionManager.GetOrCreateSession()` → starts typing indicator loop → `chat.Runner.Run()` executes `claude -p --session-id <uuid>` → logs to SQLite → sends response to Telegram + echoes to terminal.

**Hot-reload:** fsnotify → 500ms debounce → `Reload()` holds mutex for entire operation: clears and re-registers all jobs atomically.

### Settings (platform/settings.go)

```go
Settings {
    Debug, SetupComplete bool
    Provider   { ID string }                          // "anthropic"
    Messenger  { Type, BotToken, Pairing, AllowedUsers }  // "telegram"
    Chat       { Model, Effort }                      // defaults for chat sessions
    DaemonMode string                                 // "background" | "service"
}
```

### Database tables (storage/db.go)

- `execution_logs` — job execution records (status, cost, tokens, timestamps)
- `chat_sessions` — maps Telegram userID → session UUID for `--session-id`
- `chat_messages` — logged chat messages for visibility (terminal echo, `opencron logs`)

### JobConfig fields (config/job.go)

`ID`, `Name`, `Schedule`, `WorkingDir`, `PromptFile`, `Model`, `Timeout`, `Effort`, `SummaryEnabled`, `NoSessionPersist`, `Enabled`

### Hardcoded execution defaults

| Setting | Value | Notes |
|---------|-------|-------|
| `permission_mode` | `bypassPermissions` | Always — jobs run unattended |
| `output_format` | `json` | Always — for structured parsing |
| `no_session_persistence` | `true` | Default in wizard |
| `timeout` | `300` | 5 minutes default |
| `effort` | (empty = high) | Claude Code default |
| `summary_enabled` | `false` | Optional summary injection |

### Platform support

| Platform | PID detection | Config path |
|----------|--------------|-------------|
| Windows | `OpenProcess` (`lock_windows.go`) | `%APPDATA%\opencron\` |
| Linux | `syscall.Signal(0)` (`lock_unix.go`) | `~/.opencron/` or `$XDG_CONFIG_HOME/opencron/` |
| macOS | `syscall.Signal(0)` (`lock_unix.go`) | `~/.opencron/` |

### Runtime config directory

```
<BaseDir>/
  ├── schedules/          # One YAML per job
  ├── prompts/            # One .md per job (prompt content)
  ├── logs/               # stdout (.json) / stderr (.log) per execution
  ├── summary/            # Execution summaries (when summary_enabled)
  ├── workspace/          # CLAUDE.md + .claude/ (copied from .workspace/ during setup)
  ├── data/opencron.db   # SQLite (WAL mode)
  ├── settings.json       # All settings (debug, provider, messenger, chat, daemon)
  └── opencron.pid       # Daemon lock file
```

### Telegram bot architecture

Bot runs inside the daemon (`opencron start`). Single process — no IPC needed.

**Commands:** `/new` (clear session), `/jobs` (inline keyboard job list), `/model` (inline keyboard model picker), `/effort` (inline keyboard effort picker), `/status` (daemon + session info), `/help`

**Chat flow:** Text message → auth check → per-user mutex → get/create session → typing indicator loop (5s refresh) → `claude -p --session-id <uuid>` → parse JSON → send response + log to SQLite + echo to terminal

**Session management:** `chat_sessions` maps Telegram userID → UUID. The UUID is passed as `--session-id` to Claude Code, which manages conversation history internally. `/new` deactivates current session and creates a fresh UUID.

**Authorization:** Two pairing modes: `gatherToken` (generate code, send to bot to verify) or `allowList` (manually enter user IDs/@usernames).

### Concurrency model

- `sync.Mutex` protects daemon's job map during hot-reload (held for entire operation)
- `cron.SkipIfStillRunning` prevents overlapping execution of the same job
- `sync.Map` per-user processing lock prevents concurrent Telegram message handling
- SQLite WAL mode with 5s busy timeout
- Each job runs as isolated subprocess via `exec.CommandContext`
- `Watcher.Stop()` uses `sync.Once` to prevent double-close panic

## Gotchas

- **Embedded files:** `executor/task-preamble.txt` and `executor/summary-prompt.txt` are `//go:embed`-ed — changes require rebuild
- **Prompt piped via stdin:** Prompt content is passed via stdin (not CLI args) to avoid OS argument length limits and process list exposure
- **TUI library:** Uses `charmbracelet/huh` for forms and `lipgloss` for styling — Catppuccin Mocha color palette (`#cba6f7` purple, `#a6e3a1` green, `#f38ba8` red, `#fab387` orange, `#6c7086` dim)
- **Debug logging:** Gated by `settings.json` — only writes to `logs/opencron-debug.log` when `platform.IsDebugEnabled()` returns true
- **Job name validation:** Alphanumeric + hyphens + underscores only
- **Prompt file security:** Must be relative path, no `..` traversal, no absolute paths
- **Model validation:** Only allows `sonnet`, `opus`, `haiku` and their full model IDs
- **First-run detection:** `PersistentPreRunE` on rootCmd checks `IsSetupComplete()` — skips for `setup`, `help`, `version` commands
- **Chat timeout:** 120s for chat messages (vs 300s default for scheduled jobs)
- **Telegram bot token:** Stored in `settings.json` — not committed to git

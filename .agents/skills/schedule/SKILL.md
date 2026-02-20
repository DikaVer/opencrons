---
name: schedule
description: Manage scheduled Claude Code automation jobs. Use when user wants to create, list, edit, run, enable/disable, or remove scheduled tasks, or set up cron-like automation.
user-invocable: true
argument-hint: [add|list|edit|run|enable|disable|remove|logs|status]
---

You help the user manage cli-scheduler jobs. This is the RECOMMENDED way to create jobs because you can write high-quality, actionable prompts.

## Commands

| Command | Description |
|---------|-------------|
| `/schedule add` | Create a new scheduled job (you write the prompt) |
| `/schedule list` | Show all configured jobs |
| `/schedule edit <name>` | Edit an existing job's config |
| `/schedule run <name>` | Execute a job immediately (shows cost + token usage) |
| `/schedule enable <name>` | Enable a disabled job |
| `/schedule disable <name>` | Disable a job (keeps config) |
| `/schedule remove [name]` | Remove a job (`-f` to skip confirmation) |
| `/schedule logs [name]` | View execution history |
| `/schedule status` | Show daemon status + next scheduled runs |

---

## Adding a Job (`/schedule add`)

This is where you shine. Walk the user through these steps:

### Step 1: Understand the goal
Ask what they want to automate, how often, and what the expected output should be.

### Step 2: Write the prompt
Craft a detailed, specific prompt. Good prompts for scheduled jobs:
- Start with the objective and expected outcome
- Include specific file paths, patterns, or scopes when relevant
- Define the output format if you want structured results
- Set boundaries (e.g., "only check files modified in the last 24h")
- Are self-contained — don't assume interactive context

### Step 3: Show the prompt, iterate if needed

### Step 4: Determine the config
Choose schedule, model, effort, timeout, and context based on the task complexity.

### Step 5: Save everything

**First**, write the prompt to the prompts directory:
- **Linux/macOS:** `~/.cli-scheduler/prompts/<job-name>.md`
- **Windows:** `%APPDATA%\cli-scheduler\prompts\<job-name>.md`

**Then**, call `scheduler add --non-interactive` with all parameters:

```bash
scheduler add --non-interactive \
  --name "job-name" \
  --schedule "0 2 * * *" \
  --prompt-file "job-name.md" \
  --working-dir "/path/to/project" \
  --model sonnet \
  --effort medium \
  --timeout 300 \
  --max-turns 20 \
  --summary \
  --context "project_memory"
```

---

## Complete Flag Reference (`scheduler add --non-interactive`)

### Required flags

| Flag | Description | Example |
|------|-------------|---------|
| `--name` | Unique job name (lowercase, hyphens, underscores) | `"nightly-review"` |
| `--schedule` | 5-field cron expression | `"0 2 * * *"` |
| `--working-dir` | Project directory where Claude executes | `"/home/user/project"` |

### Optional flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prompt-file` | `<name>.md` | Prompt filename (in prompts/ directory) |
| `--prompt-content` | — | Prompt text (written to prompt file) |
| `--model` | `sonnet` | Claude model: `sonnet`, `opus`, `haiku` |
| `--effort` | (empty = high) | Effort level: `low`, `medium`, `high`, `max` |
| `--permission-mode` | `bypassPermissions` | Permission mode: `plan`, `default`, `bypassPermissions` |
| `--max-budget` | `0` (unlimited) | Max cost in USD per execution |
| `--max-turns` | `0` (unlimited) | Max agentic turns (tool calls) per execution |
| `--timeout` | `300` | Wall-clock timeout in seconds |
| `--add-dir` | — | Additional directories (repeatable) |
| `--mcp-config` | — | Path to MCP server config file |
| `--summary` | `false` | Enable Telegram-style execution summary |
| `--context` | `all` | Context sources (see below) |

### Effort levels

Effort controls how much thinking Claude puts into the task. Set via `--effort` flag. This maps to the `CLAUDE_CODE_EFFORT_LEVEL` environment variable.

| Level | Description | Best for |
|-------|-------------|----------|
| `low` | Most token-efficient, significant savings | Simple tasks, quick checks |
| `medium` | Balanced speed, cost, and quality | Standard automation, code generation |
| `high` | Full capability (default) | Complex reasoning, difficult tasks |
| `max` | Absolute maximum capability (**Opus only**) | Critical tasks requiring best results |

**Recommendation:** Use `medium` for routine jobs to save tokens, `high` for anything complex.

### Context control (`--context`)

Controls which Claude Code memory/context sources are loaded. Values: `all`, `none`, or comma-separated list.

| Source | What it loads | Recommendation |
|--------|--------------|----------------|
| `project_memory` | CLAUDE.md, .claude/rules/ | **Always enable** — project conventions |
| `user_memory` | ~/.claude/CLAUDE.md | Usually adds noise for scheduled jobs |
| `local_memory` | CLAUDE.local.md | Rarely needed |
| `auto_memory` | Claude's auto-generated notes | May be stale |
| `skills` | Slash commands | Not useful for automated execution |

**Recommended:** `--context "project_memory"` for most jobs. Use `"none"` for fully isolated execution.

**Note:** Claude Code's internal system prompt (~150K tokens) always loads regardless of context settings.

### Cron schedule reference

| Expression | Meaning |
|-----------|---------|
| `0 * * * *` | Every hour |
| `0 */6 * * *` | Every 6 hours |
| `0 2 * * *` | Daily at 2 AM |
| `0 9 * * *` | Daily at 9 AM |
| `0 9 * * 1` | Weekly, Monday 9 AM |
| `0 9 * * 1-5` | Weekdays at 9 AM |
| `*/30 * * * *` | Every 30 minutes |
| `0 9,17 * * *` | At 9 AM and 5 PM |

### Timeout guide

| Task complexity | Recommended timeout |
|----------------|---------------------|
| Quick checks (lint, simple queries) | 60-120s |
| Standard tasks (review, generate) | 300s (default) |
| Complex tasks (refactor, multi-file analysis) | 600-900s |

### Max turns guide

| Task type | Recommended max-turns |
|-----------|----------------------|
| Read-only analysis | 5-10 |
| Standard code changes | 10-20 |
| Complex multi-file work | 20-50 |
| Unlimited (careful) | 0 (default) |

---

## YAML ↔ CLI ↔ `claude -p` Flag Mapping

This cross-reference shows how each config field maps across all three interfaces:

| YAML field | CLI flag (`scheduler add`) | `claude -p` flag | Notes |
|-----------|---------------------------|------------------|-------|
| `name` | `--name` | — | Job identifier |
| `schedule` | `--schedule` | — | Cron expression |
| `working_dir` | `--working-dir` | `cmd.Dir` | Working directory |
| `prompt_file` | `--prompt-file` | stdin | Prompt content piped via stdin |
| `model` | `--model` | `--model` | sonnet/opus/haiku |
| `effort` | `--effort` | `CLAUDE_CODE_EFFORT_LEVEL` env | low/medium/high/max |
| `permission_mode` | `--permission-mode` | `--permission-mode` | bypassPermissions for unattended |
| `max_budget_usd` | `--max-budget` | `--max-budget-usd` | 0 = unlimited |
| `max_turns` | `--max-turns` | `--max-turns` | 0 = unlimited |
| `timeout` | `--timeout` | `context.WithTimeout` | Kills process on exceed |
| `add_dirs` | `--add-dir` | `--add-dir` | Repeatable |
| `mcp_config` | `--mcp-config` | `--mcp-config` | MCP server config |
| `no_session_persistence` | — (always true) | `--no-session-persistence` | Hardcoded |
| `summary_enabled` | `--summary` | Prompt injection | Appends summary directive |
| `disable_project_memory` | `--context` | `--setting-sources` | Combined into sources list |
| `disable_user_memory` | `--context` | `--setting-sources` | Combined into sources list |
| `disable_local_memory` | `--context` | `--setting-sources` | Combined into sources list |
| `disable_auto_memory` | `--context` | `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` | Environment variable |
| `disable_skills` | `--context` | `--disable-slash-commands` | Flag |
| — | — | `--output-format json` | Always hardcoded |

---

## Other Commands

### Edit: `scheduler edit <name>`
Opens the interactive edit wizard. Or edit the YAML directly at:
- `~/.cli-scheduler/schedules/<name>.yml` (Linux/macOS)
- `%APPDATA%\cli-scheduler\schedules\<name>.yml` (Windows)

The daemon hot-reloads YAML changes automatically (500ms debounce).

### Enable/Disable
```bash
scheduler disable <name>   # Pauses job (keeps config, daemon skips it)
scheduler enable <name>    # Resumes running on schedule
```

### List: `scheduler list`
Shows all jobs with name, schedule, model, permission mode, and enabled/disabled status.

### Run: `scheduler run <name>`
Execute immediately (bypass schedule). Shows status, duration, exit code, cost, and token breakdown (input/output/cache read/cache write).

### Remove: `scheduler remove <name>`
Deletes the job config YAML and its prompt file. Use `-f` to skip confirmation.

### Logs: `scheduler logs [name]`
View execution history. Shows job name, start time, status, trigger type, cost, and token I/O.

### Status: `scheduler status`
Shows daemon running/stopped status and next scheduled run time for each enabled job.

### Validate: `scheduler validate`
Validates all job configs. Reports errors (invalid cron, missing working dir) and warnings (missing prompt file).

---

## Daemon

The daemon is a background process that runs scheduled jobs and hot-reloads on config changes.

```bash
scheduler start              # Run in foreground (Ctrl+C to stop)
scheduler start --install    # Install as OS service (requires admin/root)
scheduler stop               # Stop running daemon
scheduler status             # Check daemon + next runs
```

---

## Runtime Directories

| Path | Contents |
|------|----------|
| `schedules/` | One YAML config per job |
| `prompts/` | One .md file per job (prompt content) |
| `system-prompts/` | Reusable system prompts (advanced) |
| `logs/` | stdout (.json) and stderr (.log) per execution |
| `summary/` | Telegram-style summaries (when enabled) |
| `data/scheduler.db` | SQLite execution log + token usage |
| `scheduler.pid` | Daemon PID lock file |

**Linux/macOS:** `~/.cli-scheduler/` (or `$XDG_CONFIG_HOME/cli-scheduler/`)
**Windows:** `%APPDATA%\cli-scheduler\`

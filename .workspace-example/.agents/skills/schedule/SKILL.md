---
name: schedule
description: Manage scheduled Claude Code automation jobs. Use when user wants to create, list, edit, run, enable/disable, or remove scheduled tasks, or set up cron-like automation.
user-invocable: true
argument-hint: [add|list|edit|run|enable|disable|remove|logs|status]
---

You help the user manage opencrons jobs. This is the RECOMMENDED way to create jobs because you guide the user through writing high-quality, production-ready prompts.

## Commands

| Command | Description |
|---------|-------------|
| `/schedule add` | Create a new scheduled job (interactive prompt workshop) |
| `/schedule list` | Show all configured jobs |
| `/schedule edit <name>` | Edit an existing job's config or prompt |
| `/schedule run <name>` | Execute a job immediately (shows cost + token usage) |
| `/schedule enable <name>` | Enable a disabled job |
| `/schedule disable <name>` | Disable a job (keeps config) |
| `/schedule remove [name]` | Remove a job (`-f` to skip confirmation) |
| `/schedule logs [name]` | View execution history |
| `/schedule status` | Show daemon status + next scheduled runs |

---

## Adding a Job (`/schedule add`)

This is where you shine. You don't just collect inputs — you actively help the user craft a prompt that will work reliably when running unattended.

### Phase 1: Understand the Goal

Ask the user two things:

1. **What do you want to automate?** (get a plain-language description)
2. **How often should it run?** (help pick a cron schedule)

From the description, classify the task type:

| Type | Description | Example |
|------|-------------|---------|
| `read-only` | Analysis, scanning, reporting — no file mutations | Security scan, code review, metric collection |
| `modification` | Creates or edits files | Refactoring, code generation, formatting |
| `build-test` | Runs builds, tests, or lints | CI checks, test suites, compilation |
| `reporting` | Generates summaries or changelogs | Daily digest, changelog, audit report |

The task type determines the default constraints and safety guardrails you'll suggest in Phase 2.

### Phase 2: Prompt Workshop

This is the core of the skill. You write a first draft of the prompt, then walk the user through improving it using a quality rubric.

#### 2a: Write the First Draft

Structure every prompt using this template:

```markdown
## Objective
[One sentence: what this job accomplishes and what success looks like]

## Context
- Repository: [brief description of the project]
- Schedule: [human-readable schedule, e.g., "Daily at 2 AM"]
- This job runs unattended — do not ask questions or wait for input.

## Steps
1. [First concrete action with explicit scope]
2. [Second action, referencing step 1 output if needed]
3. [Final action: produce output]

## Constraints
- Scope: [file patterns, directories, boundaries]
- Do NOT: [explicit forbidden actions list]
- [Maximum mutation count if applicable]
- If nothing needs to be done, report "No changes needed" and exit cleanly

## Output
[Format: plain text, markdown, or JSON — choose what best fits the task. Natural language is preferred unless the output will be machine-parsed.]

## Error Handling
- [Specific fallback for each likely failure mode]
- Always produce output, even on failure
```

**Writing principles:**

- **Be explicit, not vague.** Think of the agent as a capable but brand-new team member with zero context on your project.
- **One atomic task per job.** If the description contains "and" connecting unrelated actions, suggest splitting into two jobs.
- **Explain the "why", not just the "what".** Instead of "never modify test files," say "never modify test files — they serve as the correctness baseline and changes could mask regressions."
- **Scope aggressively.** Always specify which files/directories are in scope and what the maximum blast radius is.
- **Specify output format.** Unattended jobs need clear output for monitoring. Natural language text is fine — use JSON only when the output will be parsed by another system.
- **Include a Do-NOT list.** Unattended agents need explicit boundaries since there's no human to say "stop."

**Auto-suggest constraints by task type:**

For `read-only` tasks:
```
- Read-only: Do NOT modify, create, or delete any files
- Do NOT run commands that change state (no git commit, push, install)
```

For `modification` tasks:
```
- Do NOT delete files or create new directories unless explicitly needed
- Do NOT modify test files, CI configs, or lock files
- Maximum files to change per run: [suggest 5-10 based on scope]
- Check current state before making changes — skip if already correct (idempotency)
```

For `build-test` tasks:
```
- Do NOT modify source files or install new dependencies
- Do NOT push, commit, or create branches
- If tests fail, report failures — do not attempt to fix them
```

For `reporting` tasks:
```
- Read-only: Do NOT modify source files
- Write output only to the designated report path
- If no data to report, produce an empty report (not silence)
```

#### 2b: Score the Prompt

After writing the draft, silently evaluate it against this rubric:

| Dimension | 0 (Missing) | 1 (Weak) | 2 (Strong) |
|-----------|-------------|----------|------------|
| **Objective clarity** | No goal stated | Vague goal | One-sentence atomic objective with success criteria |
| **Scope boundaries** | No boundaries | Partial (dir but not file types) | Explicit file patterns, dirs, and limits |
| **Output format** | Unspecified | "Give me a summary" | JSON schema or structured template |
| **Constraints** | None | Partial | Explicit Do-NOT list + mutation limits |
| **Error handling** | None | "Handle errors" | Specific fallback per error type |
| **Idempotency** | No mention | Implicit | Explicit check-before-write pattern |
| **Context** | None | Some background | Repo, schedule, and purpose stated |
| **Self-contained** | Assumes interactive context | Partially | Fully standalone, no external dependencies |
| **Conciseness** | Too short or padded | Reasonable | Focused, every sentence earns its place |
| **Structure** | Plain text blob | Some headings | Clear sections with headers |

**Target: 16+/20.** If your draft scores below 16, improve it before showing the user. Do NOT show the score — use it internally to guide your improvements.

#### 2c: Present and Iterate

Show the user the prompt draft. Ask:

> "Here's the prompt I'd suggest for this job. Want me to adjust anything — scope, constraints, output format, or anything else?"

If the user says it's fine, move on. If they have feedback, revise and show the updated version. One or two rounds is typical.

**Common user feedback patterns and how to handle them:**

| User says | Action |
|-----------|--------|
| "Make it simpler" | Remove error handling boilerplate, merge steps, drop JSON output for plain text |
| "Make it more thorough" | Add sub-steps, expand file scope, increase mutation limits |
| "I don't need JSON output" | Switch to markdown or plain text, but keep structure |
| "Add X check" | Add a step, update scope if needed |
| "It's too restrictive" | Loosen constraints, but warn about risks for unattended execution |

### Phase 3: Configure the Job

Based on the task, recommend settings:

| Setting | How to choose |
|---------|---------------|
| **Model** | `sonnet` for most jobs. `opus` only for tasks requiring deep reasoning or critical correctness. `haiku` for simple, high-frequency checks. |
| **Effort** | `medium` for routine jobs (saves tokens). `high` for complex reasoning. `low` for trivial checks. `max` for critical Opus tasks only. |
| **Timeout** | 60-120s for quick checks. 300s (default) for standard tasks. 600-900s for complex multi-file work. |
| **Summary** | Enable for jobs where you want a human-readable execution digest. |
| **Disallowed tools** | Restrict specific tools for safety. E.g., `Bash(git:*)` blocks git commands, `Edit` blocks file edits. Use for read-only jobs. |

Present the config as a summary table and confirm with the user.

### Phase 3b: Telegram Notifications (when summary is enabled)

If the user has `--summary` enabled **and** a Telegram integration is configured, automatically append a Telegram notification step to the prompt:

```markdown
## Notifications
After completing all steps, send the summary to Telegram:
- Read the Telegram bot token from environment variable `TELEGRAM_BOT_TOKEN`
- Read the chat ID from environment variable `TELEGRAM_CHAT_ID`
- Send the summary as a message using the Telegram Bot API:
  POST https://api.telegram.org/bot{TOKEN}/sendMessage
  Body: { "chat_id": "{CHAT_ID}", "text": "<summary text>", "parse_mode": "Markdown" }
- If the environment variables are not set, skip silently — do not fail the job
- Keep the message concise (under 4000 characters)
```

**When to include this:** Always add this block if `--summary` is set, so the job self-reports to Telegram on every run. The user must have `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` set in their environment — remind them if they haven't mentioned it.

**Setup reminder to show the user:**
```
To enable Telegram notifications, set these environment variables:
  TELEGRAM_BOT_TOKEN=your_bot_token   (from @BotFather)
  TELEGRAM_CHAT_ID=your_chat_id       (your personal or group chat ID)
```

### Phase 4: Save Everything

**First**, write the prompt to the prompts directory:
- **Linux/macOS:** `~/.opencrons/prompts/<job-name>.md`
- **Windows:** `%APPDATA%\opencrons\prompts\<job-name>.md`

**Then**, call `opencrons add --non-interactive` with all parameters:

```bash
opencrons add --non-interactive \
  --name "job-name" \
  --schedule "0 2 * * *" \
  --prompt-file "job-name.md" \
  --working-dir "/path/to/project" \
  --model sonnet \
  --effort medium \
  --timeout 300 \
  --summary \
  --disallowed-tools "Bash(git:*)"
```

---

## Prompt Anti-Patterns

When reviewing or writing prompts, watch for these and fix them:

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| "Review the code" | No scope, no criteria, no output format | "Review `src/auth/*.ts` for SQL injection. Report: file, line, severity, fix." |
| "Clean up the project" | Unbounded, destructive potential | "Remove `*.log` files from `build/`. Do not modify source files." |
| "Make it better" | Subjective, no success criteria | "Reduce `processData()` cyclomatic complexity below 10. Run tests after each change." |
| "Fix any issues" | Unbounded scope | "Fix TypeScript errors in `build.log`. Do not change test files." |
| "Be thorough and careful" | Anti-laziness filler (Claude 4 ignores or over-amplifies this) | Remove it. Use `effort: high` instead. |
| Multiple unrelated tasks | Split attention, lower quality | One job per atomic task. |

---

## Example: Before and After

**User says:** "I want a daily code review"

**Bad prompt (score: 4/20):**
```
Review my code for issues and fix them.
```

**Good prompt (score: 18/20):**
```markdown
## Objective
Scan TypeScript source files for common security vulnerabilities and report findings.
Do not modify any files.

## Context
- Repository: Node.js API server (Express + Prisma)
- Schedule: Daily at 2 AM
- This job runs unattended — do not ask questions or wait for input.

## Steps
1. Read all `.ts` files in `src/api/` and `src/middleware/`
2. Check for: SQL injection (string concatenation in queries), XSS (unsanitized
   user input in responses), hardcoded secrets (API keys, passwords in source)
3. For each finding, record: file path, line number, vulnerability type,
   severity (high/medium/low), and suggested fix

## Constraints
- Read-only: Do NOT modify, create, or delete any files
- Scope: Only `src/api/` and `src/middleware/`
- Ignore: `*.test.ts`, `*.spec.ts`, `node_modules/`
- If no vulnerabilities found, report "No issues found"

## Output
Respond with JSON:
{
  "status": "clean | issues_found",
  "scan_summary": { "files_scanned": 0, "issues_found": 0 },
  "findings": [
    {
      "file": "src/api/users.ts",
      "line": 42,
      "type": "sql_injection",
      "severity": "high",
      "description": "User input concatenated into SQL query",
      "suggestion": "Use parameterized query"
    }
  ]
}

## Error Handling
- If a file cannot be read, skip it and add to a `skipped_files` array
- Always produce the JSON output, even if all files were skipped
```

---

## Complete Flag Reference (`opencrons add --non-interactive`)

### Required flags

| Flag | Description | Example |
|------|-------------|---------|
| `--name` | Unique job name (alphanumeric, hyphens, underscores) | `"nightly-review"` |
| `--schedule` | 5-field cron expression | `"0 2 * * *"` |
| `--working-dir` | Project directory where Claude executes | `"/home/user/project"` |

### Optional flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prompt-file` | `<name>.md` | Prompt filename (in prompts/ directory) |
| `--prompt-content` | — | Prompt text (written to prompt file) |
| `--model` | `sonnet` | Claude model: `sonnet`, `opus`, `haiku` |
| `--effort` | (empty = high) | Effort level: `low`, `medium`, `high`, `max` |
| `--timeout` | `300` | Wall-clock timeout in seconds |
| `--summary` | `false` | Enable execution summary |
| `--disallowed-tools` | — | Tools to deny (repeatable: `--disallowed-tools "Bash(git:*)" --disallowed-tools "Edit"`) |

### Cron schedule reference

| Expression | Meaning |
|-----------|---------|
| `* * * * *` | Every minute |
| `*/5 * * * *` | Every 5 minutes |
| `*/30 * * * *` | Every 30 minutes |
| `0 * * * *` | Every hour |
| `0 */6 * * *` | Every 6 hours |
| `0 2 * * *` | Daily at 2 AM |
| `0 9 * * *` | Daily at 9 AM |
| `0 9 * * 1` | Weekly, Monday 9 AM |
| `0 9 * * 1-5` | Weekdays at 9 AM |
| `0 0 1 * *` | Monthly, 1st at midnight |
| `0 9,17 * * *` | At 9 AM and 5 PM |

### Timeout guide

| Task complexity | Recommended timeout |
|----------------|---------------------|
| Quick checks (lint, simple queries) | 60-120s |
| Standard tasks (review, generate) | 300s (default) |
| Complex tasks (refactor, multi-file analysis) | 600-900s |

---

## YAML <-> CLI <-> `claude -p` Flag Mapping

| YAML field | CLI flag | `claude -p` flag | Notes |
|-----------|----------|------------------|-------|
| `name` | `--name` | — | Job identifier |
| `schedule` | `--schedule` | — | Cron expression |
| `working_dir` | `--working-dir` | `cmd.Dir` | Working directory |
| `prompt_file` | `--prompt-file` | stdin | Piped via stdin |
| `model` | `--model` | `--model` | sonnet/opus/haiku |
| `effort` | `--effort` | `--effort` | low/medium/high/max |
| `timeout` | `--timeout` | `context.WithTimeout` | Kills process on exceed |
| `no_session_persistence` | — (always true) | `--no-session-persistence` | Hardcoded |
| `summary_enabled` | `--summary` | Prompt injection |
| `disallowed_tools` | `--disallowed-tools` | `--disallowed-tools` | Repeatable, restricts tool access |
| — | — | `--permission-mode bypassPermissions` | Always hardcoded |
| — | — | `--output-format json` | Always hardcoded |

---

## Other Commands

### Edit: `opencrons edit <name>`
Opens the interactive edit wizard. Or edit files directly:
- YAML: `~/.opencrons/schedules/<name>.yml` (Linux/macOS) or `%APPDATA%\opencrons\schedules\<name>.yml` (Windows)
- Prompt: `~/.opencrons/prompts/<name>.md` (Linux/macOS) or `%APPDATA%\opencrons\prompts\<name>.md` (Windows)

The daemon hot-reloads YAML changes automatically (500ms debounce).

### Enable/Disable
```bash
opencrons disable <name>   # Pauses job (keeps config, daemon skips it)
opencrons enable <name>    # Resumes running on schedule
```

### List: `opencrons list`
Shows all jobs with name, schedule, model, effort, and enabled/disabled status.

### Run: `opencrons run <name>`
Execute immediately (bypass schedule). Shows status, duration, exit code, cost, and token breakdown.

### Remove: `opencrons remove <name>`
Deletes the job config YAML and its prompt file.
- `-f` / `--force`: skip confirmation prompt
- `--keep-prompt`: delete config only, keep the prompt `.md` file

### Logs: `opencrons logs [name]`
View execution history. Shows job name, start time, status, trigger type, cost, and token I/O.
- `-n` / `--limit`: number of entries to show (default 20). Example: `opencrons logs my-job -n 50`

### Status: `opencrons status`
Shows daemon running/stopped status and next scheduled run time for each enabled job.

### Validate: `opencrons validate`
Validates all job configs. Reports errors (invalid cron, missing working dir) and warnings (missing prompt file).

---

## Daemon

```bash
opencrons start              # Run in foreground (Ctrl+C to stop)
opencrons start --install    # Install as OS service (requires admin/root)
opencrons stop               # Stop running daemon
opencrons status             # Check daemon + next runs
```

---

## Runtime Directories

| Path | Contents |
|------|----------|
| `schedules/` | One YAML config per job |
| `prompts/` | One .md file per job (prompt content) |
| `logs/` | stdout (.json) and stderr (.log) per execution |
| `summary/` | Execution summaries (when enabled) |
| `data/opencrons.db` | SQLite execution log + token usage |
| `settings.json` | All settings (provider, messenger, chat, daemon, debug) |
| `opencrons.pid` | Daemon PID lock file |

**Linux/macOS:** `~/.opencrons/` (or `$XDG_CONFIG_HOME/opencrons/`)
**Windows:** `%APPDATA%\opencrons\`

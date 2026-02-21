<h1>🟪🟩 OpenCron — Automated AI Scheduler</h1>

<div align="center">
  <img src="public/header.png" alt="OpenCron" width="500">
  <br><br>
  <p>
    Automate Claude Code on a schedule.<br>
    Chat with Claude from Telegram.<br>
    Manage everything from a beautiful TUI.
  </p>

  <a href="https://github.com/DikaVer/opencron/actions"><img src="https://img.shields.io/github/actions/workflow/status/DikaVer/opencron/ci.yml?style=flat-square&label=build" alt="Build"></a>
  <a href="https://github.com/DikaVer/opencron/releases"><img src="https://img.shields.io/github/v/release/DikaVer/opencron?style=flat-square&color=a6e3a1&label=release" alt="Release"></a>
  <a href="https://github.com/DikaVer/opencron/blob/main/LICENSE"><img src="https://img.shields.io/github/license/DikaVer/opencron?style=flat-square&color=cba6f7" alt="License"></a>
  <a href="https://github.com/DikaVer/opencron"><img src="https://img.shields.io/github/go-mod/go-version/DikaVer/opencron?style=flat-square&color=89b4fa" alt="Go Version"></a>
</div>

<p align="center">
  <a href="#-why-opencron">Why OpenCron</a> ·
  <a href="#-install">Install</a> ·
  <a href="#%EF%B8%8F-how-it-works">How it works</a> ·
  <a href="#-quick-start">Quick start</a> ·
  <a href="#-telegram-bot">Telegram bot</a> ·
  <a href="#-cli-reference">CLI reference</a> ·
  <a href="#%EF%B8%8F-configuration">Configuration</a> ·
  <a href="#%EF%B8%8F-roadmap">Roadmap</a>

</p>

---

OpenCron is an open-source scheduler that runs [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (`claude -p`) jobs on cron schedules. It pairs a terminal-native TUI with a Telegram bot — so you can define, monitor, and chat with your AI jobs from anywhere.

**Built with** Go · Cobra · Charmbracelet · SQLite · Catppuccin Mocha

---

<h2><img src="public/logo.png" alt="" width="128" height="128" align="absmiddle"> Why OpenCron 💡<h2>

### The OAuth lockdown changed the game

In January 2026, [Anthropic deployed server-side restrictions](https://winbuzzer.com/2026/02/19/anthropic-bans-claude-subscription-oauth-in-third-party-apps-xcxwbn/) that blocked third-party tools from authenticating with Claude Pro and Max subscription OAuth tokens. Tools like OpenCode, OpenClaw, and other Third-party [stopped working overnight](https://venturebeat.com/technology/anthropic-cracks-down-on-unauthorized-claude-usage-by-third-party-harnesses). The official policy is clear:

> *OAuth authentication (used with Free, Pro, and Max plans) is intended **exclusively** for Claude Code and Claude.ai. Using OAuth tokens in any other product, tool, or service is not permitted.*
> — [Claude Code Docs — Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)

But here's the thing — **`claude -p` is Claude Code**. It's Anthropic's own CLI, running with your own subscription, exactly as intended. No OAuth hijacking, no third-party token routing, no terms of service violations. Just Claude Code doing what Claude Code was built to do.

OpenCron wraps `claude -p` in a structured scheduler. Every job is a direct invocation of Claude Code — the same binary, the same auth, the same process you'd run by hand in your terminal. Nothing in between.

### OpenClaw is great, but cron deserves better

[OpenClaw](https://openclaw.ai/) is a fantastic personal AI assistant — WhatsApp, Telegram, iMessage, 500+ integrations. But for many developers, [80–90% of what they actually use it for is cron jobs](https://docs.openclaw.ai/automation/cron-jobs): daily code reviews, morning briefings, CI monitoring, scheduled cleanups.

OpenCron takes that core use case and gives it the dedicated tooling it deserves:

| | OpenClaw | OpenCron |
|---|---|---|
| 🎯 **Focus** | General-purpose AI assistant | Purpose-built cron scheduler |
| 👁️ **Visibility** | Jobs buried in a JSON file | Interactive TUI + structured logs |
| 📊 **Tracking** | Minimal job history | SQLite with cost, tokens, status per run |
| 🔐 **Auth** | Uses Claude Code OAuth | Uses Claude Code directly — fully compliant |

If you need a Swiss Army knife, use OpenClaw. If you need your cron jobs to **just work** — with clear visibility, cost tracking, and a beautiful interface — that's OpenCron.

---

## 📦 Install

Requires [Go 1.25+](https://go.dev/dl/) and [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

```bash
go install github.com/DikaVer/opencron/cmd/opencron@latest
```

That's it. The binary lands in `$GOPATH/bin` — already in your PATH if Go is set up correctly.

<details>
<summary><strong>🔨 Build from source</strong></summary>

```bash
git clone https://github.com/DikaVer/opencron.git
cd opencron

# Linux / macOS
sudo make install

# Windows
go install ./cmd/opencron/
```

</details>

<details>
<summary><strong>🗑️ Uninstall</strong></summary>

```bash
# Linux / macOS
sudo make uninstall

# Windows (PowerShell)
Remove-Item "$(go env GOPATH)\bin\opencron.exe"
```

</details>

Verify:

```bash
opencron --help
```

---

## ⚙️ How it works

OpenCron has three modes of operation:

| Mode | What it does |
|------|-------------|
| 🖥️ **Interactive TUI** | Run `opencron` with no args. A full-screen menu for creating, editing, and managing jobs. |
| ⌨️ **CLI commands** | Scriptable subcommands — `opencron add`, `opencron list`, `opencron run`, etc. |
| 💬 **Telegram bot** | Runs inside the daemon. Chat with Claude, trigger jobs, and get notifications — all from your phone. |

### The execution flow

```
📝 You define a job
  → ⏰ cron schedule triggers it
    → 📄 OpenCron reads the prompt file
      → 🚀 pipes it to `claude -p` with your configured model & effort
        → 📊 captures output, cost, and token usage
          → 💾 logs everything to SQLite
            → 💬 (optionally) sends a summary to Telegram
```

Every job runs as an isolated subprocess with `--permission-mode bypassPermissions` for unattended operation, and `--output-format json` for structured result parsing.

---

## 🚀 Quick start

### 1. Run the setup wizard

```bash
opencron setup
```

The wizard walks you through:
- 🔑 **Provider** — detects your Anthropic configuration
- 💬 **Messenger** — connect a Telegram bot (optional)
- 🤖 **Chat defaults** — pick a default model and effort level
- ⚡ **Daemon mode** — background process or OS service

### 2. Create your first job

```bash
opencron add
```

The interactive wizard asks for a name, cron schedule, working directory, model, and prompt. Or go fully non-interactive:

```bash
opencron add --non-interactive \
  --name "daily-review" \
  --schedule "0 9 * * *" \
  --working-dir "/path/to/project" \
  --prompt-content "Review open PRs and summarize findings." \
  --model sonnet
```

### 3. Start the daemon

```bash
opencron start
```

The daemon runs your cron jobs, watches for config changes (hot-reload), and starts the Telegram bot if configured. Stop it with `opencron stop`.

### 4. Check the logs

```bash
opencron logs                    # all jobs
opencron logs daily-review       # specific job
opencron logs daily-review -n 50 # specific job, last 50 entries
```

---

## 💬 Telegram bot

The Telegram integration turns OpenCron into a remote AI assistant you can reach from your pocket.

### Setup

1. 🤖 Create a bot via [@BotFather](https://t.me/BotFather) on Telegram
2. 🔧 Run `opencron setup` or `opencron settings` to configure the bot token
3. 🔐 Pair your account:
   - **Verification code** — OpenCron generates a code, you send it to your bot to prove ownership
   - **Allowlist** — manually enter Telegram user IDs or @usernames in settings

### Bot commands

| Command | Action |
|---------|--------|
| `/new` | 🆕 Start a fresh chat session |
| `/stop` | 🛑 Cancel a running query |
| `/jobs` | 📋 Browse and trigger jobs |
| `/model` | 🧠 Switch model (Sonnet, Opus, Haiku) |
| `/effort` | ⚡ Adjust effort level |
| `/status` | 📊 Daemon and session info |
| `/help` | ❓ Show all commands |

Send any text message to chat with Claude directly. Sessions persist across messages — Claude remembers context until you `/new`.

---

## 📖 CLI reference

### 📋 Job management

```bash
opencron add              # create a job (interactive wizard)
opencron list             # list all jobs
opencron edit <name>      # edit a job
opencron remove <name>    # delete a job (--force to skip confirmation)
opencron enable <name>    # enable a disabled job
opencron disable <name>   # disable a job
opencron validate         # validate all job configs
```

### ▶️ Execution

```bash
opencron run <name>       # run a job immediately
opencron logs [name]      # view execution logs (-n to set limit)
```

### 🔄 Daemon

```bash
opencron start            # start the daemon (foreground)
opencron start --install  # install as OS service
opencron stop             # stop the daemon
opencron status           # check daemon status
```

### 🔧 Settings

```bash
opencron setup            # first-time setup wizard
opencron settings         # manage all settings
opencron debug [on|off]   # toggle debug logging
```

### 🏳️ Global flags

```bash
opencron --verbose        # verbose output (any subcommand)
opencron --help           # help for any command
```

---

## 🗂️ Configuration

### Job config

Each job is a YAML file in `schedules/` with a corresponding prompt in `prompts/`.

| Field | Description | Default |
|-------|-------------|---------|
| `name` | Unique identifier (alphanumeric, hyphens, underscores) | required |
| `schedule` | Cron expression (`0 9 * * *`) | required |
| `working_dir` | Project directory for execution | required |
| `prompt_file` | Markdown file with the prompt | `<name>.md` |
| `model` | `sonnet`, `opus`, or `haiku` | provider default |
| `effort` | `low`, `medium`, `high`, or `max` | `high` |
| `timeout` | Seconds before killing the job | `300` |
| `disallowed_tools` | Tool restrictions (e.g. `Bash(git:*)`) | none |
| `summary_enabled` | Generate execution summary | `false` |
| `enabled` | Whether the job runs on schedule | `true` |

### 📂 Directory structure

OpenCron stores its configuration and data in a platform-specific directory:

| Platform | Path |
|----------|------|
| 🐧 **Linux** | `~/.opencron/` or `$XDG_CONFIG_HOME/opencron/` |
| 🍎 **macOS** | `~/.opencron/` or `$XDG_CONFIG_HOME/opencron/` |
| 🪟 **Windows** | `%APPDATA%\opencron\` |

```
~/.opencron/
├── schedules/        # job configs (YAML)
├── prompts/          # prompt files (Markdown)
├── logs/             # execution stdout/stderr
├── summary/          # execution summaries
├── workspace/        # shared AGENTS.md
├── data/opencron.db  # SQLite database
├── settings.json     # all settings
└── opencron.pid      # daemon lock file
```

### 🤖 Workspace (AGENTS.md)

OpenCron copies a [`workspace/AGENTS.md`](.workspace-example/AGENTS.md) into your config directory during setup. This file is injected into every job as context — it acts as a shared system prompt so Claude understands it's running inside OpenCron.

You can customize it to add project-wide instructions, coding standards, or constraints that apply to all your scheduled jobs.

A ready-to-use example is included in the repo at [`.workspace-example/`](.workspace-example/) — it's copied automatically on first run via `opencron setup`.

### 🖥️ Platform support

| | Linux | macOS | Windows |
|-|-------|-------|---------|
| CLI & TUI | ✅ | ✅ | ✅ |
| Daemon | ✅ | ✅ | ✅ |
| OS service install | ✅ | ✅ | ✅ |

---

## 🔍 How jobs execute

When a job triggers, OpenCron:

1. 📄 Reads the prompt file and prepends a [task preamble](internal/executor/task-preamble.txt)
2. 📎 Optionally appends a [summary prompt](internal/executor/summary-prompt.txt)
3. 🔒 Pipes everything via stdin to `claude -p` (avoids argument length limits)
4. 🚀 Passes `--effort`, `--model`, `--permission-mode bypassPermissions`, `--output-format json`
5. 📝 Captures stdout/stderr to log files
6. 📊 Parses the JSON response for cost, token usage, and result
7. 💾 Writes execution records to SQLite

Config changes are picked up automatically — the daemon watches the `schedules/` directory with [fsnotify](https://github.com/fsnotify/fsnotify) and hot-reloads jobs without restart.

---

## 🗺️ Roadmap

OpenCron is focused on Claude Code today, but the vision is broader.

### 🔜 Coming soon

- 🧠 **Anthropic API** — run jobs directly with [Anthropic API](https://docs.anthropic.com/en/docs/build-with-claude/overview) without requiring Claude Code installed — lower overhead, API key auth, usage-based billing
- 🤖 **OpenAI API** — first-class support for the [OpenAI API](https://platform.openai.com/docs/overview) — same scheduling, same TUI, same logs, different provider
- ⌨️ **Codex CLI support** — run OpenAI's [Codex CLI](https://github.com/openai/codex) alongside Claude Code jobs
- 🔌 **Plugin system** — interactive, controllable integrations:

| Plugin | What it does |
|--------|-------------|
| 🐙 **GitHub** | Auto-review PRs, create issues, merge when checks pass |
| 📧 **Email** | Morning digest, inbox triage, auto-replies |
| 📐 **Linear** | Create/update issues from job results, sync sprint status |
| 💬 **Slack** | Post summaries, respond to threads, channel notifications |
| 📊 **Custom** | Bring your own plugin — webhook-based architecture |

### 🔮 Future ideas

- 📈 Web dashboard for job monitoring and cost analytics
- 🔗 Job chaining — pipe output from one job into the next
- 🏷️ Job tags and filtering
- 📱 Push notifications (beyond Telegram)
- 🔀 **Multi-provider jobs** — run the same prompt against Claude and GPT in parallel, compare results

Have an idea? [Open an issue](https://github.com/DikaVer/opencron/issues) — contributions are welcome.

---

## 🛠️ Development

```bash
make build          # build binary
make build-all      # cross-compile (linux + windows)
make test           # run tests
make lint           # golangci-lint
make tidy           # go mod tidy
make clean          # remove build artifacts
```

### Architecture

```
cmd/opencron/           → entry point
internal/
├── cmd/                → Cobra commands + TUI menu
├── config/             → job config, YAML I/O, prompt files
├── tui/                → interactive wizards and menus
├── executor/           → claude -p command builder and runner
├── storage/            → SQLite (execution logs, chat sessions)
├── daemon/             → cron orchestrator, hot-reload, OS service
├── platform/           → cross-platform paths, PID, settings
├── provider/           → AI provider interface
├── messenger/telegram/ → Telegram bot handlers
├── chat/               → chat session manager
├── logger/             → debug logging
└── ui/                 → shared styles (Catppuccin Mocha)
```


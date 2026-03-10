<h1>🟪🟩 OpenCrons — Automated Agent Scheduler</h1>

<div align="center">
  <img src="public/header.png" alt="OpenCrons" width="500">
  <br><br>
  <p>
    Automate Claude Code on a schedule.<br>
    Chat with Claude from Telegram.<br>
    Manage everything from a beautiful TUI.
  </p>

  <a href="https://github.com/DikaVer/opencrons/actions"><img src="https://img.shields.io/github/actions/workflow/status/DikaVer/opencrons/ci.yml?style=flat-square&label=build" alt="Build"></a>
      <a href="https://github.com/DikaVer/opencrons/blob/main/LICENSE"><img src="https://img.shields.io/github/license/DikaVer/opencrons?style=flat-square&color=cba6f7" alt="License"></a>
        <a href="https://github.com/DikaVer/opencrons"><img src="https://img.shields.io/github/go-mod/go-version/DikaVer/opencrons?style=flat-square&color=89b4fa" alt="Go Version"></a>
          <a href="https://github.com/DikaVer/opencrons/stargazers"><img src="https://img.shields.io/github/stars/DikaVer/opencrons?style=flat-square&color=f9e2af" alt="Stars"></a>
            <a href="https://github.com/DikaVer/opencrons/issues"><img src="https://img.shields.io/github/issues/DikaVer/opencrons?style=flat-square&color=f38ba8" alt="Issues"></a>
              <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-a6e3a1?style=flat-square" alt="Platform">
                <img src="https://img.shields.io/badge/claude--code-compatible-cba6f7?style=flat-square" alt="Claude Code">
</div>

<p align="center">
  <a href="#-why-opencrons">Why OpenCrons</a> ·
  <a href="#-install">Install</a> ·
  <a href="#%EF%B8%8F-how-it-works">How it works</a> ·
  <a href="#-quick-start">Quick start</a> ·
  <a href="#-telegram-bot">Telegram bot</a> ·
  <a href="#-cli-reference">CLI reference</a> ·
  <a href="#%EF%B8%8F-configuration">Configuration</a> ·
  <a href="#%EF%B8%8F-security">Security</a> ·
  <a href="#%EF%B8%8F-roadmap">Roadmap</a>

</p>

---

OpenCrons is an open-source scheduler that runs [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (`claude -p`) jobs on cron schedules. It pairs a terminal-native TUI with a Telegram bot — so you can define, monitor, and chat with your AI jobs from anywhere. Built for developers, researchers, and teams who want structured, repeatable AI automation.

**Built with** Go · Cobra · Charmbracelet · SQLite · Catppuccin Mocha

---


## ⚠️ Security

OpenCrons is a young project — security coverage is incomplete. Use it with this in mind.

### Agent execution model

Every scheduled job runs `claude -p` with `--permission-mode bypassPermissions`. This means:

- **No sandbox** — the agent process runs with your full user permissions, in your working directory, with access to your filesystem, network, and any tools Claude Code can invoke
- **No tool restrictions by default** — unless you explicitly set `disallowed_tools` on a job, the agent can read files, write files, run shell commands, and call external services
- **Unattended** — jobs trigger on a cron schedule with no human in the loop to approve or reject individual actions

This is intentional for automation — but it means **the prompt is the security boundary**. A poorly scoped prompt can lead to unintended writes, deletions, or network calls.

**Practical guidance:**
- Scope prompts tightly to the task at hand
- Use `disallowed_tools` to restrict capabilities where possible (e.g. `Bash(rm:*)`)
- Set a `working_dir` that contains only what the job needs access to
- Review execution logs regularly (`opencrons logs`)
- Keep your Claude Code version up to date
- Use `container: podman` or `container: docker` to run jobs inside an isolated container — the working directory is bind-mounted and credentials are passed read-only

This project just released and does not yet cover all security aspects. Contributions and issues are welcome.

---

## 📦 Install

Requires [Go 1.25+](https://go.dev/dl/) and [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

```bash
go install github.com/DikaVer/opencrons/cmd/opencrons@latest
```

That's it. The binary lands in `$GOPATH/bin` — already in your PATH if Go is set up correctly.

<details>
<summary><strong>🔨 Build from source</strong></summary>

```bash
git clone https://github.com/DikaVer/opencrons.git
cd opencrons

# Linux / macOS
sudo make install

# Windows
go install ./cmd/opencrons/
```

</details>

<details>
<summary><strong>🗑️ Uninstall</strong></summary>

```bash
# Linux / macOS
sudo make uninstall

# Windows (PowerShell)
Remove-Item "$(go env GOPATH)\bin\opencrons.exe"
```

</details>

Verify:

```bash
opencrons --help
```

---

## ⚙️ How it works

OpenCrons has three modes of operation:

| Mode | What it does |
|------|-------------|
| 🖥️ **Interactive TUI** | Run `opencrons` with no args. A full-screen menu for creating, editing, and managing jobs. |
| ⌨️ **CLI commands** | Scriptable subcommands — `opencrons add`, `opencrons list`, `opencrons run`, etc. |
| 💬 **Telegram bot** | Runs inside the daemon. Chat with Claude, trigger jobs, and get notifications — all from your phone. |

### The execution flow

```
📝 You define a job
  → ⏰ cron schedule triggers it
    → 📄 OpenCrons reads the prompt file
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
opencrons setup
```

The wizard walks you through:
- 🔑 **Provider** — detects your Anthropic configuration
- 💬 **Messenger** — connect a Telegram bot (optional)
- 🤖 **Chat defaults** — pick a default model and effort level
- ⚡ **Daemon mode** — background process or OS service

### 2. Create your first job

```bash
opencrons add
```

The interactive wizard asks for a name, cron schedule, working directory, model, and prompt. Or go fully non-interactive:

```bash
opencrons add --non-interactive \
  --name "daily-review" \
  --schedule "0 9 * * *" \
  --working-dir "/path/to/project" \
  --prompt-content "Review open PRs and summarize findings." \
  --model sonnet \
  --container podman \
  --container-image claude-runner:latest
```

### 3. Start the daemon

```bash
opencrons start
```

The daemon runs your cron jobs, watches for config changes (hot-reload), and starts the Telegram bot if configured. Stop it with `opencrons stop`.

### 4. Check the logs

```bash
opencrons logs                    # all jobs
opencrons logs daily-review       # specific job
opencrons logs daily-review -n 50 # specific job, last 50 entries
```

---

## 💬 Telegram bot

The Telegram integration turns OpenCrons into a remote AI assistant you can reach from your pocket.

### Setup

1. 🤖 Create a bot via [@BotFather](https://t.me/BotFather) on Telegram
2. 🔧 Run `opencrons setup` or `opencrons settings` to configure the bot token
3. 🔐 Pair your account:
   - **Verification code** — OpenCrons generates a code, you send it to your bot to prove ownership

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
opencrons add              # create a job (interactive wizard)
opencrons list             # list all jobs
opencrons edit <name>      # edit a job
opencrons remove <name>    # delete a job (--force to skip confirmation)
opencrons enable <name>    # enable a disabled job
opencrons disable <name>   # disable a job
opencrons validate         # validate all job configs
```

### ▶️ Execution

```bash
opencrons run <name>       # run a job immediately
opencrons logs [name]      # view execution logs (-n to set limit)
```

### 🔄 Daemon

```bash
opencrons start            # start the daemon (foreground)
opencrons start --install  # install as user service (no sudo needed)
opencrons start --install --system  # install as system service (requires sudo)
opencrons stop             # stop the daemon
opencrons status           # check daemon status
```

### 🔧 Settings

```bash
opencrons setup            # first-time setup wizard
opencrons settings         # manage all settings
opencrons debug [on|off]   # toggle debug logging
```

### 🏳️ Global flags

```bash
opencrons --verbose        # verbose output (any subcommand)
opencrons --help           # help for any command
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
| `container` | Container runtime: `docker` or `podman` | none (host) |
| `container_image` | Container image (required when container is set) | none |
| `max_retries` | Retries on failure (0-10) | `0` |
| `retry_backoff` | Retry strategy: `exponential` or `linear` | `exponential` |
| `on_success` | Jobs to chain on success | none |
| `enabled` | Whether the job runs on schedule | `true` |

### 📂 Directory structure

OpenCrons stores its configuration and data in a platform-specific directory:

| Platform | Path |
|----------|------|
| 🐧 **Linux** | `~/.opencrons/` or `$XDG_CONFIG_HOME/opencrons/` |
| 🍎 **macOS** | `~/.opencrons/` or `$XDG_CONFIG_HOME/opencrons/` |
| 🪟 **Windows** | `%APPDATA%\opencrons\` |

```
~/.opencrons/
├── .agents/              # agent config directory (canonical)
│   └── skills/           # scheduling skill + plugin skills
├── .claude/ → .agents/   # provider symlink (Anthropic)
├── AGENTS.md             # agent instructions (canonical)
├── CLAUDE.md → AGENTS.md # provider symlink (Anthropic)
├── schedules/            # job configs (YAML)
├── prompts/              # prompt files (Markdown)
├── logs/                 # execution stdout/stderr
├── summary/              # execution summaries
├── data/opencrons.db     # SQLite database
├── settings.json         # all settings
└── opencrons.pid         # daemon lock file
```

The `.agents/` directory and `AGENTS.md` file are the canonical (real) locations. Provider-specific names like `.claude/` and `CLAUDE.md` are created as symlinks so that each provider's tooling finds what it expects. On Windows without developer mode, junctions and hard links are used as fallbacks.

### 🤖 Agent instructions (AGENTS.md)

OpenCrons copies [`AGENTS.md`](.workspace-example/AGENTS.md) and [`.agents/`](.workspace-example/.agents/) into your config directory during setup. `AGENTS.md` is injected into every job as context — it acts as a shared system prompt so Claude understands it's running inside OpenCrons.

You can customize it to add project-wide instructions, coding standards, or constraints that apply to all your scheduled jobs.

A ready-to-use example is included in the repo at [`.workspace-example/`](.workspace-example/) — it's copied automatically on first run via `opencrons setup`.

### 🖥️ Platform support

| | Linux | macOS | Windows |
|-|-------|-------|---------|
| CLI & TUI | ✅ | ✅ | ✅ |
| Daemon | ✅ | ✅ | ✅ |
| OS service install | ✅ | ✅ | ✅ |

---

## 🔍 How jobs execute

When a job triggers, OpenCrons:

1. 📄 Reads the prompt file and prepends a [task preamble](internal/executor/task-preamble.txt)
2. 📎 Optionally appends a [summary prompt](internal/executor/summary-prompt.txt)
3. 🔒 Pipes everything via stdin to `claude -p` (avoids argument length limits)
4. 🚀 Passes `--effort`, `--model`, `--permission-mode bypassPermissions`, `--output-format json`; optionally wraps in Docker/Podman container
5. 📝 Captures stdout/stderr to log files
6. 📊 Parses the JSON response for cost, token usage, and result
7. 💾 Writes execution records to SQLite

Config changes are picked up automatically — the daemon watches the `schedules/` directory with [fsnotify](https://github.com/fsnotify/fsnotify) and hot-reloads jobs without restart.

---

## 🗺️ Roadmap

OpenCrons is focused on Claude Code today, but the vision is broader.

### 🔜 Coming soon

- ⌨️ **Codex CLI support** — run OpenAI's [Codex CLI](https://github.com/openai/codex) alongside Claude Code jobs

### 🔮 Future ideas

- 🏷️ Job tags and filtering
- 📱 Push notifications (beyond Telegram)
- 🔀 **Multi-provider jobs** — run the same prompt against Claude and GPT in parallel, compare results
- 🔄 **Workflow agent pipeline** — chain multiple agents into a directed pipeline where each agent's output becomes the next one's input; branch on conditions, fan out in parallel, merge results

Have an idea? [Open an issue](https://github.com/DikaVer/opencrons/issues) — contributions are welcome.

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
cmd/opencrons/           → entry point
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


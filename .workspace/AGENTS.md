# OpenCron Agent

Hey there! I'm your scheduling assistant for OpenCron. I help you automate Claude Code tasks so they run on their own — no babysitting required.

## What I Can Help With

**Creating scheduled jobs** — This is my bread and butter. Tell me what you want to automate and I'll:
- Help you pick the right schedule (cron can be cryptic, I'll translate)
- Write a solid prompt that actually works when running unattended
- Walk you through improving it together — scope, constraints, output format
- Set up the config (model, effort, timeout) with sensible defaults

**Managing existing jobs** — List, edit, enable/disable, remove, check logs, view token usage.

**Troubleshooting** — If a job is failing or producing weird output, I can help debug the prompt or config.

**Developing OpenCron itself** — I know the codebase well. Need to add a feature, fix a bug, or understand how something works? I've got you.

## Quick Start

Just tell me what you want in plain language:

> "I want to run a security scan on my API every night"

> "Can you set up a weekly test runner for my project?"

> "I need a daily report of TODO comments in my codebase"

I'll take it from there — ask a couple questions, draft a prompt, and get everything wired up.

## The `/schedule` Skill

The interactive scheduling skill lives at:

```
.workspace/.agents/skills/schedule/SKILL.md
```

To install it so you can use `/schedule` commands in any Claude Code session:

```bash
# Linux/macOS
mkdir -p ~/.claude/skills/schedule
cp .workspace/.agents/skills/schedule/SKILL.md ~/.claude/skills/schedule/SKILL.md

# Windows (PowerShell)
New-Item -ItemType Directory -Force "$env:USERPROFILE\.claude\skills\schedule"
Copy-Item .workspace\.agents\skills\schedule\SKILL.md "$env:USERPROFILE\.claude\skills\schedule\SKILL.md"

# Or just
make install-skill
```

Once installed, you can use these anywhere:

| Command | What it does |
|---------|-------------|
| `/schedule add` | Create a new job together (I'll help write the prompt) |
| `/schedule list` | See all your jobs at a glance |
| `/schedule edit <name>` | Tweak a job's config or prompt |
| `/schedule run <name>` | Run a job right now (see cost + tokens) |
| `/schedule enable <name>` | Turn a paused job back on |
| `/schedule disable <name>` | Pause a job without deleting it |
| `/schedule remove <name>` | Delete a job for good |
| `/schedule logs [name]` | Check how recent runs went |
| `/schedule status` | See if the daemon is running + next scheduled runs |

## How I Write Prompts

When you ask me to create a job, I don't just take your description and dump it into a file. I follow a workshop approach:

1. **Understand your goal** — What are you automating? How often? What does success look like?
2. **Classify the task** — Is it read-only analysis, file modification, build/test, or reporting? This shapes the safety guardrails.
3. **Draft a structured prompt** — With a clear objective, scoped steps, explicit constraints, output format, and error handling.
4. **Review it together** — I'll show you the draft and we iterate until you're happy.
5. **Pick the right config** — Model, effort level, and timeout matched to the task complexity.
6. **Save and verify** — Write the prompt file, create the job, confirm it's valid.

The goal is a prompt that works reliably on its own at 3 AM with nobody watching.

## Building & Testing OpenCron

```bash
go build -o build/opencron ./cmd/opencron/   # Build the binary
go test ./...                                    # Run all tests
go build ./...                                   # Compile check
make build                                       # Build with version info
make lint                                        # Run golangci-lint
```

## Project Layout

For the full architecture, package graph, and technical details, check out [CLAUDE.md](./CLAUDE.md) — that's the deep-dive reference. This file is about how I work with you, not how the code works internally.

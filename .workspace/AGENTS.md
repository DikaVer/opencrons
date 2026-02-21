# 🤖 OpenCron Scheduling Assistant

👋 Hey! I'm your scheduling buddy for OpenCron. Tell me what you want to automate and I'll handle the rest — cron expressions, prompts, configs, all of it.

---

## 💡 What I Can Help With

**⚡ Creating scheduled jobs** — My main thing. Just describe what you want and I'll:
- 🗓️ Pick the right cron schedule (I'll translate the cryptic stuff into plain English)
- ✍️ Write a prompt that actually works when running unattended
- 🔧 Set up config (model, effort, timeout) with solid defaults
- 🔄 Iterate with you until you're happy with it

**📋 Managing jobs** — List, edit, enable/disable, remove, check logs, view token usage.

**🐛 Troubleshooting** — Job failing or acting weird? I'll help debug the prompt or config.

> ⚠️ **Note:** I'm here purely as your scheduling assistant — I don't touch or develop the OpenCron codebase itself.

---

## 🚀 Quick Start

Just say what you want in plain language:

> 💬 "Run a security scan on my API every night"

> 💬 "Set up a weekly test runner for my project"

> 💬 "Give me a daily report of TODO comments in my codebase"

I'll ask a couple of questions, draft the prompt, and get it all wired up. 🎯

---

## 🛠️ The `/schedule` Skill

The interactive scheduling skill lives at:

```
.workspace/.agents/skills/schedule/SKILL.md
```

**📦 Install it once, use it everywhere:**

```bash
# 🐧 Linux/macOS
mkdir -p ~/.claude/skills/schedule
cp .workspace/.agents/skills/schedule/SKILL.md ~/.claude/skills/schedule/SKILL.md

# 🪟 Windows (PowerShell)
New-Item -ItemType Directory -Force "$env:USERPROFILE\.claude\skills\schedule"
Copy-Item .workspace\.agents\skills\schedule\SKILL.md "$env:USERPROFILE\.claude\skills\schedule\SKILL.md"

# ⚡ Or just
make install-skill
```

**Available commands:**

| Command | What it does |
|---------|-------------|
| `/schedule add` | ➕ Create a new job (I'll help write the prompt) |
| `/schedule list` | 📋 See all your jobs at a glance |
| `/schedule edit <name>` | ✏️ Tweak a job's config or prompt |
| `/schedule run <name>` | ▶️ Run a job right now (see cost + tokens) |
| `/schedule enable <name>` | ✅ Turn a paused job back on |
| `/schedule disable <name>` | ⏸️ Pause a job without deleting it |
| `/schedule remove <name>` | 🗑️ Delete a job for good |
| `/schedule logs [name]` | 📜 Check how recent runs went |
| `/schedule status` | 📡 See if the daemon is running + next scheduled runs |

---

## ✍️ How I Write Prompts

When you ask me to create a job, I don't just wing it — I follow a proper workshop approach:

1. 🎯 **Understand your goal** — What are you automating? How often? What does success look like?
2. 🏷️ **Classify the task** — Read-only? File modification? Build/test? Reporting? Shapes the guardrails.
3. 📝 **Draft a structured prompt** — Clear objective, scoped steps, explicit constraints, output format, error handling.
4. 🔄 **Review it together** — I show you the draft, we iterate until you're satisfied.
5. ⚙️ **Pick the right config** — Model, effort level, and timeout matched to task complexity.
6. ✅ **Save and verify** — Write the prompt file, create the job, confirm it's valid.

The goal: a prompt that works reliably on its own at 3 AM with nobody watching. 🌙

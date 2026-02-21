# Plugin Adapter Guide

Plugins give the LLM access to external services — GitHub issues, Gmail inbox, Linear tickets, etc. Unlike providers (which CLI tool to run) and messengers (how to talk to users), plugins inject **service-specific context and tool instructions** into the LLM prompt so it can interact with APIs on behalf of the user.

---

## Concepts

A plugin has four concerns:

| Concern | What it does | Example |
|---------|-------------|---------|
| **Setup** | Collect credentials, validate access, store config | OAuth flow, API token input, scope selection |
| **Skill** | Teach the LLM how to use the service | Markdown instructions with API patterns, tool usage |
| **Context injection** | Insert skill text into prompts at execution time | Appended to job prompts, added to workspace for chat |
| **Configuration** | Persist credentials and preferences in settings.json | Token, scopes, enabled state |

**Key distinction from providers/messengers:** Plugins don't change *how* opencrons runs — they change *what the LLM knows about* when it runs. A job with the GitHub plugin enabled gets GitHub-specific instructions appended to its prompt. A chat session with the Gmail plugin enabled gets Gmail tool instructions in its workspace context.

---

## Import Architecture

> **Critical constraint:** The `platform` package is a leaf dependency — many packages import it, but it must not import application-level packages. To avoid import cycles, `PluginConfig` lives in `platform`, not in `plugin`.

```
platform (owns PluginConfig, SetupField)
    ↑
plugin (imports platform, defines Plugin interface using platform types)
    ↑
plugin/github, plugin/gmail, plugin/linear (import plugin + platform)
    ↑
tui, executor, cmd (import plugin + platform)
```

The `plugin` package imports `platform` for type references. The `platform` package never imports `plugin`. This mirrors how `MessengerSettings` lives in `platform` while the `Messenger` interface lives in `messenger`.

---

## Interface

```go
// Package plugin — internal/plugin/plugin.go

// Plugin defines the contract for external service integrations.
type Plugin interface {
    // Identity
    ID() string          // "github" | "gmail" | "linear"
    Name() string        // Display name for TUI/logs: "GitHub" | "Gmail" | "Linear"
    Description() string // One-line description for TUI: "Manage issues, PRs, and repos"

    // Setup
    Validate(cfg platform.PluginConfig) error  // Check if credentials are valid (API call)
    SetupFields() []platform.SetupField        // Declare what the TUI should collect

    // Skill (LLM context)
    Skill(cfg platform.PluginConfig) string  // Markdown instructions for the LLM

    // Credential delivery
    EnvVars(cfg platform.PluginConfig) map[string]string  // Env vars to inject at runtime

    // Status
    IsConfigured(cfg platform.PluginConfig) bool  // Has valid config in settings
}
```

### Why `Skill(cfg)` takes a config parameter

The `Skill()` method may need runtime values (default org, team names) to customize the skill text. Rather than having it read `settings.json` from disk on every call (hidden I/O in a value method), the caller passes the already-loaded config. This keeps the method pure, testable, and consistent with `Validate(cfg)` and `IsConfigured(cfg)`.

### Why `EnvVars(cfg)` is part of the interface

Each plugin declares its own credential-to-env-var mapping. This avoids a centralized `switch` statement in the executor that must be updated for every new plugin. The executor calls `EnvVars(cfg)` generically and injects whatever the plugin returns.

### Why no `Start`/`Stop` lifecycle?

Plugins are **stateless from opencrons' perspective**. They don't run background processes or hold connections. The LLM interacts with external services directly via tool calls (Bash for CLI tools, WebFetch for APIs). Plugins just provide the instructions and credentials — the LLM does the work.

If a future plugin needs a persistent connection (e.g., a webhook listener), that should be handled as a daemon-level integration (similar to the Telegram bot), not through the plugin interface.

---

## Supporting Types

These types live in `internal/platform/settings.go` to avoid import cycles:

```go
// PluginConfig holds a single plugin's persisted configuration.
// Credentials and preferences are stored as a flat key-value map
// to avoid plugin-specific struct proliferation in the settings package.
type PluginConfig struct {
    Enabled  bool              `json:"enabled"`
    Settings map[string]string `json:"settings,omitempty"` // "token", "org", "repo", etc.
}

// SetupField declares a single input the TUI should collect during plugin setup.
type SetupField struct {
    Key         string // Settings map key: "token", "org", "repo"
    Label       string // TUI label: "Personal Access Token"
    Description string // Help text: "Create at github.com/settings/tokens"
    Required    bool   // Must be non-empty to proceed
    Secret      bool   // Mask input in TUI (for tokens/passwords)
    Default     string // Pre-filled value (for optional fields)
    Validate    func(string) error // Optional field-level validation
}
```

### Registry

```go
// internal/plugin/registry.go

// pluginOrder controls TUI display order. The map and slice are read-only
// after package initialization — no mutex needed.
var pluginOrder = []string{"github", "gmail", "linear"}

var plugins = map[string]Plugin{
    "github": &github.Plugin{},
    "gmail":  &gmail.Plugin{},
    "linear": &linear.Plugin{},
}

// Get returns a plugin by ID, or nil if not found.
func Get(id string) Plugin { return plugins[id] }

// List returns all registered plugins in display order.
func List() []Plugin {
    list := make([]Plugin, 0, len(pluginOrder))
    for _, id := range pluginOrder {
        if p, ok := plugins[id]; ok {
            list = append(list, p)
        }
    }
    return list
}

// ListEnabled returns plugins that are both configured and enabled.
// This is the authoritative gate — it checks cfg.Enabled AND p.IsConfigured(cfg).
func ListEnabled(cfgs map[string]platform.PluginConfig) []Plugin {
    var enabled []Plugin
    for _, p := range List() {
        if cfg, ok := cfgs[p.ID()]; ok && cfg.Enabled && p.IsConfigured(cfg) {
            enabled = append(enabled, p)
        }
    }
    return enabled
}
```

---

## Settings Integration

Plugin configuration lives in `settings.json` under a `plugins` key:

```json
{
  "debug": false,
  "setup_complete": true,
  "provider": { "id": "anthropic" },
  "messenger": { "type": "telegram", "bot_token": "...", "allowed_users": { "123": true } },
  "chat": { "model": "sonnet", "effort": "high" },
  "plugins": {
    "github": {
      "enabled": true,
      "settings": {
        "token": "ghp_xxxxxxxxxxxx",
        "default_owner": "myorg"
      }
    },
    "gmail": {
      "enabled": true,
      "settings": {
        "app_password": "xxxx xxxx xxxx xxxx"
      }
    },
    "linear": {
      "enabled": false,
      "settings": {
        "api_key": "lin_api_xxxxxxxxxxxx"
      }
    }
  }
}
```

### Settings struct changes (`internal/platform/settings.go`)

```go
type Settings struct {
    Debug         bool                      `json:"debug"`
    SetupComplete bool                      `json:"setup_complete"`
    Provider      *ProviderSettings         `json:"provider,omitempty"`
    Messenger     *MessengerSettings        `json:"messenger,omitempty"`
    Chat          *ChatSettings             `json:"chat,omitempty"`
    DaemonMode    string                    `json:"daemon_mode,omitempty"`
    Plugins       map[string]PluginConfig   `json:"plugins,omitempty"` // NEW
}
```

Note: `PluginConfig` is defined in `platform` (not imported from `plugin`), so no import cycle.

Add convenience functions:

```go
// GetPluginConfigs returns all plugin configs, or an empty map if none configured.
func GetPluginConfigs() map[string]PluginConfig {
    s := LoadSettings()
    if s.Plugins == nil {
        return map[string]PluginConfig{}
    }
    return s.Plugins
}
```

> **Backward compatibility:** The `Plugins` field uses `omitempty`, so existing `settings.json` files without plugins will load correctly — `s.Plugins` will be `nil`.

> **No `GetEnabledPlugins` helper in `platform`:** Use `plugin.ListEnabled(platform.GetPluginConfigs())` instead. The `plugin` package owns the `IsConfigured` check — `platform` should not duplicate that logic.

---

## Context Injection

This is the core mechanism — how plugin knowledge reaches the LLM.

### For scheduled jobs (executor)

Plugin skills are appended to the prompt in `executor.BuildCommand()`, between the user prompt and the optional summary injection:

```
┌──────────────────────────┐
│ task-preamble.txt        │  ← embedded, always present
├──────────────────────────┤
│ user prompt content      │  ← from prompts/<job>.md
├──────────────────────────┤
│ plugin skills (if any)   │  ← NEW: appended per-job or globally enabled
├──────────────────────────┤
│ summary-prompt.txt       │  ← embedded, conditional on summary_enabled
└──────────────────────────┘
```

**Injection point** in `internal/executor/claude.go`:

```go
// Build final prompt: preamble + user prompt + plugin skills + optional summary
prompt := taskPreamble + string(promptContent)

// Inject enabled plugin skills
pluginCfgs := platform.GetPluginConfigs()
for _, p := range plugin.ListEnabled(pluginCfgs) {
    cfg := pluginCfgs[p.ID()]
    prompt += "\n\n" + p.Skill(cfg)
}

// Inject plugin env vars
injectPluginEnv(cmd, pluginCfgs)

// Append summary injection if enabled
if job.SummaryEnabled {
    // ... existing summary code ...
}
```

**Environment variable injection** (generic, no switch statement):

```go
// injectPluginEnv sets environment variables declared by enabled plugins.
func injectPluginEnv(cmd *exec.Cmd, cfgs map[string]platform.PluginConfig) {
    if cmd.Env == nil {
        cmd.Env = os.Environ()
    }
    for _, p := range plugin.ListEnabled(cfgs) {
        cfg := cfgs[p.ID()]
        for key, val := range p.EnvVars(cfg) {
            cmd.Env = append(cmd.Env, key+"="+val)
        }
    }
}
```

### For chat sessions (workspace)

Chat sessions run from `platform.WorkspaceDir()`, so the LLM sees whatever is in that directory. Plugin skills are written as files in the workspace's skill directory.

> **Path note:** The workspace example uses `.agents/skills/` (copied from `.workspace-example/.agents/`). During setup, `cmd/setup.go:copyWorkspace()` renames `.agents/` to `.claude/`. Therefore the runtime workspace uses `.claude/skills/`, which is what Claude Code reads. Plugin skill files go here:

```
<BaseDir>/workspace/
├── CLAUDE.md                           ← existing (copied from .workspace-example/AGENTS.md)
├── .claude/
│   └── skills/
│       ├── schedule/SKILL.md           ← existing
│       ├── github/SKILL.md             ← NEW: written when GitHub plugin is enabled
│       ├── gmail/SKILL.md              ← NEW: written when Gmail plugin is enabled
│       └── linear/SKILL.md             ← NEW: written when Linear plugin is enabled
```

**When to write/remove skill files:**

- **Plugin enabled:** Write `SKILL.md` to `workspace/.claude/skills/<plugin-id>/SKILL.md`
- **Plugin disabled:** Remove the skill directory
- **Setup/reconfigure:** Overwrite with fresh content

This should happen in:
1. `cmd/setup.go` — after the setup wizard completes
2. `cmd/settings.go` — when plugins are toggled via the settings menu
3. The plugin setup TUI flow — when a plugin is first configured

```go
// internal/plugin/workspace.go

// SyncWorkspaceSkills ensures workspace skill files match enabled plugin state.
func SyncWorkspaceSkills(cfgs map[string]platform.PluginConfig) error {
    skillsDir := filepath.Join(platform.WorkspaceDir(), ".claude", "skills")

    for _, p := range List() {
        pluginSkillDir := filepath.Join(skillsDir, p.ID())
        skillFile := filepath.Join(pluginSkillDir, "SKILL.md")

        cfg, exists := cfgs[p.ID()]
        if exists && cfg.Enabled && p.IsConfigured(cfg) {
            // Write/update the skill file
            if err := os.MkdirAll(pluginSkillDir, 0755); err != nil {
                return fmt.Errorf("creating plugin skill dir %s: %w", pluginSkillDir, err)
            }
            if err := os.WriteFile(skillFile, []byte(p.Skill(cfg)), 0644); err != nil {
                return fmt.Errorf("writing plugin skill %s: %w", skillFile, err)
            }
        } else {
            // Remove the skill directory if it exists
            if err := os.RemoveAll(pluginSkillDir); err != nil && !os.IsNotExist(err) {
                return fmt.Errorf("removing plugin skill dir %s: %w", pluginSkillDir, err)
            }
        }
    }
    return nil
}
```

### Per-job plugin selection (optional, future)

The initial implementation enables plugins globally (all jobs get all enabled plugins). A future enhancement could add a `plugins` field to `JobConfig`:

```yaml
# schedules/my-job.yml
name: my-job
schedule: "0 2 * * *"
working_dir: /path/to/project
plugins:           # optional — if omitted, uses all enabled plugins
  - github
  - linear
```

This is **not needed for v1** — global enable/disable is sufficient. Add per-job selection only if users need fine-grained control.

---

## TUI Integration

### Setup wizard (Step 2.5 — after Messenger, before Chat Model)

Add a plugin configuration step to `internal/tui/setup_wizard.go`:

```go
// Step: Plugin Integrations
// Show multi-select of available plugins
// For each selected plugin:
//   1. Collect SetupFields() via huh forms
//   2. Validate credentials via plugin.Validate()
//   3. Store config in SetupResult.Plugins
```

**Flow:**

```
┌─────────────────────────────────────────────────────┐
│  Plugin Integrations (optional)                     │
│                                                     │
│  Select integrations to enable:                     │
│  [x] GitHub — Manage issues, PRs, and repos         │
│  [ ] Gmail — Read and send emails                   │
│  [x] Linear — Track issues and projects             │
│  [ ] Skip all                                       │
│                                                     │
│  ─────────────────────────────────────────────────── │
│                                                     │
│  GitHub Setup                                       │
│                                                     │
│  Personal Access Token: ●●●●●●●●●●●●●●●●           │
│  (Create at github.com/settings/tokens)             │
│                                                     │
│  Default Owner (optional): myorg                    │
│                                                     │
│  ✓ Token validated — access confirmed               │
│                                                     │
│  ─────────────────────────────────────────────────── │
│                                                     │
│  Linear Setup                                       │
│                                                     │
│  API Key: ●●●●●●●●●●●●●●●●                         │
│  (Create at linear.app/settings/api)                │
│                                                     │
│  ✓ API key validated — 3 teams found                │
└─────────────────────────────────────────────────────┘
```

### Wiring `SetupField.Validate` into huh forms

The `Validate` function on `SetupField` must be explicitly wired into `charmbracelet/huh` form inputs:

```go
func buildPluginForm(p plugin.Plugin) *huh.Form {
    var groups []*huh.Group
    for _, field := range p.SetupFields() {
        input := huh.NewInput().
            Title(field.Label).
            Description(field.Description)

        if field.Secret {
            input = input.EchoMode(huh.EchoModePassword)
        }
        if field.Default != "" {
            input = input.Value(&field.Default)
        }
        if field.Validate != nil {
            input = input.Validate(field.Validate)
        }

        groups = append(groups, huh.NewGroup(input))
    }
    return huh.NewForm(groups...).WithTheme(catppuccinTheme())
}
```

### SetupResult changes

The `SetupResult` struct gains a new field (using the `platform`-owned type):

```go
type SetupResult struct {
    Provider   ProviderResult
    Messenger  MessengerResult
    Chat       ChatResult
    DaemonMode string
    Plugins    map[string]platform.PluginConfig // NEW
}
```

### Settings menu

Add a "Plugins" option to `internal/tui/settings_menu.go`:

```
Settings
─────────────────────
  Provider      Anthropic (Claude Code v1.x.x)
  Messenger     Telegram (@mybot)
  Plugins       GitHub ✓, Linear ✓, Gmail ✗     ← NEW
  Chat Model    sonnet / high
  Daemon Mode   background
  Debug         off
  Re-run Setup
```

Selecting "Plugins" opens a sub-menu:

```
Plugin Integrations
─────────────────────
  GitHub        ✓ Enabled (ghp_...xxxx)
  Gmail         ✗ Not configured
  Linear        ✓ Enabled (lin_...xxxx)
  Back
```

Selecting a plugin shows:
- **If configured:** Toggle enable/disable, reconfigure credentials, remove
- **If not configured:** Run the setup flow (collect fields, validate, save)

---

## Skill Format

Each plugin embeds a `skill.md` file via `//go:embed`. This file is the **complete instruction set** the LLM needs to interact with the service. It follows the Claude skill format:

```markdown
---
name: github
description: Interact with GitHub repositories, issues, and pull requests using the gh CLI.
---

## Available Tools

You have access to the `gh` CLI tool for GitHub operations. Authentication is
pre-configured via the GITHUB_TOKEN environment variable.

## Common Operations

### Issues
- List issues: `gh issue list --repo {owner}/{repo}`
- Create issue: `gh issue create --repo {owner}/{repo} --title "..." --body "..."`
...
```

### Runtime skill augmentation

Some skill content is static (embedded), but some needs runtime values. For example, the GitHub skill may include the user's default org, or the Linear skill may include team names. Since `Skill(cfg)` receives the config, use simple template replacement:

```go
func (p *Plugin) Skill(cfg platform.PluginConfig) string {
    skill := skillTemplate // //go:embed skill.md

    // Replace template variables with runtime values
    if owner := cfg.Settings["default_owner"]; owner != "" {
        skill = strings.ReplaceAll(skill, "{{DEFAULT_OWNER}}", owner)
    }
    return skill
}
```

> **Security:** Never inject raw credentials into skill text. The LLM doesn't need the token — it uses `gh` CLI or `curl` with tokens from environment variables. The skill should instruct the LLM to use `$GITHUB_TOKEN` or the `gh` CLI (which reads from its own config). Credentials reach the subprocess via `EnvVars()` → `injectPluginEnv()`.

---

## Package Structure

```
internal/plugin/
├── plugin.go              # Plugin interface (uses platform.PluginConfig, platform.SetupField)
├── registry.go            # Plugin registry (Get, List, ListEnabled)
├── workspace.go           # SyncWorkspaceSkills — write/remove skill files
├── github/
│   ├── plugin.go          # GitHubPlugin struct, interface implementation
│   ├── skill.md           # //go:embed skill instructions for the LLM
│   └── plugin_test.go     # Validation tests
├── gmail/
│   ├── plugin.go
│   ├── skill.md
│   └── plugin_test.go
└── linear/
    ├── plugin.go
    ├── skill.md
    └── plugin_test.go

internal/platform/
├── settings.go            # MODIFIED — adds PluginConfig, SetupField, Plugins field
```

---

## GitHub Plugin Implementation

### `internal/plugin/github/plugin.go`

```go
package github

import (
    _ "embed"
    "fmt"
    "os"
    "os/exec"
    "strings"

    "github.com/DikaVer/opencrons/internal/platform"
)

//go:embed skill.md
var skillTemplate string

type Plugin struct{}

func (p *Plugin) ID() string          { return "github" }
func (p *Plugin) Name() string        { return "GitHub" }
func (p *Plugin) Description() string { return "Manage issues, PRs, and repositories via gh CLI" }

func (p *Plugin) SetupFields() []platform.SetupField {
    return []platform.SetupField{
        {
            Key:         "token",
            Label:       "Personal Access Token",
            Description: "Create at github.com/settings/tokens (needs repo, issues scopes)",
            Required:    true,
            Secret:      true,
            Validate: func(s string) error {
                if !strings.HasPrefix(s, "ghp_") && !strings.HasPrefix(s, "github_pat_") {
                    return fmt.Errorf("token should start with ghp_ or github_pat_")
                }
                return nil
            },
        },
        {
            Key:         "default_owner",
            Label:       "Default Owner/Org",
            Description: "Default repo owner for commands (optional, e.g., 'myorg')",
            Required:    false,
        },
    }
}

func (p *Plugin) Validate(cfg platform.PluginConfig) error {
    token := cfg.Settings["token"]
    if token == "" {
        return fmt.Errorf("GitHub token is required")
    }

    // Validate token by calling the GitHub API with the provided token
    cmd := exec.Command("gh", "api", "user", "--jq", ".login")
    cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+token)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("GitHub authentication failed: %w", err)
    }
    return nil
}

func (p *Plugin) Skill(cfg platform.PluginConfig) string {
    skill := skillTemplate
    if owner := cfg.Settings["default_owner"]; owner != "" {
        skill = strings.ReplaceAll(skill, "{{DEFAULT_OWNER}}", owner)
    }
    return skill
}

func (p *Plugin) EnvVars(cfg platform.PluginConfig) map[string]string {
    vars := make(map[string]string)
    if token := cfg.Settings["token"]; token != "" {
        vars["GITHUB_TOKEN"] = token
    }
    return vars
}

func (p *Plugin) IsConfigured(cfg platform.PluginConfig) bool {
    return cfg.Settings["token"] != ""
}
```

### `internal/plugin/github/skill.md`

```markdown
---
name: github
description: Interact with GitHub repositories, issues, and pull requests.
---

## GitHub Integration

You have access to the `gh` CLI for GitHub operations. Authentication is pre-configured via the `GITHUB_TOKEN` environment variable.

### Issues

| Action | Command |
|--------|---------|
| List open issues | `gh issue list` |
| List with filters | `gh issue list --label bug --assignee @me` |
| Create issue | `gh issue create --title "Title" --body "Description"` |
| View issue | `gh issue view 123` |
| Close issue | `gh issue close 123 --comment "Resolved in #456"` |
| Add comment | `gh issue comment 123 --body "Comment text"` |
| Edit issue | `gh issue edit 123 --add-label "priority:high"` |

### Pull Requests

| Action | Command |
|--------|---------|
| List open PRs | `gh pr list` |
| Create PR | `gh pr create --title "Title" --body "Description" --base main` |
| View PR | `gh pr view 123` |
| Review PR | `gh pr review 123 --approve` or `--request-changes --body "..."` |
| Merge PR | `gh pr merge 123 --squash --delete-branch` |
| Check CI status | `gh pr checks 123` |

### Repository

| Action | Command |
|--------|---------|
| View repo | `gh repo view` |
| List releases | `gh release list` |
| Create release | `gh release create v1.0.0 --title "Release" --notes "..."` |

### Search

| Action | Command |
|--------|---------|
| Search issues | `gh search issues "query" --repo owner/repo` |
| Search code | `gh search code "pattern" --repo owner/repo` |
| Search PRs | `gh search prs "query" --repo owner/repo` |

### API (advanced)

For operations not covered by `gh` subcommands:
```bash
gh api repos/{owner}/{repo}/issues --jq '.[].title'
gh api graphql -f query='{ viewer { login } }'
```

### Guidelines

- Always use `--repo owner/repo` when operating outside the current directory's repo
- Use `--json` flag for machine-parseable output: `gh issue list --json number,title,state`
- Prefer `--jq` for filtering JSON output directly
- Check `gh auth status` if authentication errors occur
```

---

## Gmail Plugin Implementation

### Authentication approach

Gmail uses **SMTP with App Passwords** for simplicity. This avoids requiring `gcloud` CLI or complex OAuth flows. The user generates an App Password at https://myaccount.google.com/apppasswords, and the LLM sends emails via `curl` to the SMTP relay or uses Python's `smtplib`.

For **reading** emails, the skill instructs the LLM to use IMAP via Python's `imaplib` (available on all systems with Python). This keeps the dependency footprint minimal — no Google Cloud SDK required.

### `internal/plugin/gmail/plugin.go`

```go
package gmail

import (
    _ "embed"
    "fmt"
    "net/smtp"
    "strings"

    "github.com/DikaVer/opencrons/internal/platform"
)

//go:embed skill.md
var skillTemplate string

type Plugin struct{}

func (p *Plugin) ID() string          { return "gmail" }
func (p *Plugin) Name() string        { return "Gmail" }
func (p *Plugin) Description() string { return "Read and send emails via Gmail" }

func (p *Plugin) SetupFields() []platform.SetupField {
    return []platform.SetupField{
        {
            Key:         "email",
            Label:       "Gmail Address",
            Description: "Your full Gmail address (e.g., you@gmail.com)",
            Required:    true,
            Validate: func(s string) error {
                if !strings.Contains(s, "@") {
                    return fmt.Errorf("must be a valid email address")
                }
                return nil
            },
        },
        {
            Key:         "app_password",
            Label:       "App Password",
            Description: "Generate at myaccount.google.com/apppasswords (16-char code, spaces ok)",
            Required:    true,
            Secret:      true,
            Validate: func(s string) error {
                cleaned := strings.ReplaceAll(s, " ", "")
                if len(cleaned) != 16 {
                    return fmt.Errorf("app password should be 16 characters (spaces are stripped)")
                }
                return nil
            },
        },
    }
}

func (p *Plugin) Validate(cfg platform.PluginConfig) error {
    email := cfg.Settings["email"]
    pass := strings.ReplaceAll(cfg.Settings["app_password"], " ", "")
    if email == "" || pass == "" {
        return fmt.Errorf("email and app password are required")
    }

    // Validate by attempting SMTP auth (does not send any email)
    auth := smtp.PlainAuth("", email, pass, "smtp.gmail.com")
    // smtp.SendMail with empty recipients will validate auth
    err := smtp.SendMail("smtp.gmail.com:587", auth, email, nil, nil)
    if err != nil && !strings.Contains(err.Error(), "no recipients") {
        return fmt.Errorf("Gmail authentication failed: %w", err)
    }
    return nil
}

func (p *Plugin) Skill(cfg platform.PluginConfig) string {
    skill := skillTemplate
    if email := cfg.Settings["email"]; email != "" {
        skill = strings.ReplaceAll(skill, "{{GMAIL_ADDRESS}}", email)
    }
    return skill
}

func (p *Plugin) EnvVars(cfg platform.PluginConfig) map[string]string {
    vars := make(map[string]string)
    if email := cfg.Settings["email"]; email != "" {
        vars["GMAIL_ADDRESS"] = email
    }
    if pass := cfg.Settings["app_password"]; pass != "" {
        vars["GMAIL_APP_PASSWORD"] = strings.ReplaceAll(pass, " ", "")
    }
    return vars
}

func (p *Plugin) IsConfigured(cfg platform.PluginConfig) bool {
    return cfg.Settings["email"] != "" && cfg.Settings["app_password"] != ""
}
```

### `internal/plugin/gmail/skill.md`

```markdown
---
name: gmail
description: Read and send emails via Gmail using IMAP/SMTP.
---

## Gmail Integration

You can read and send emails using Gmail. Credentials are available via environment variables:
- `GMAIL_ADDRESS` — the Gmail address ({{GMAIL_ADDRESS}})
- `GMAIL_APP_PASSWORD` — the app password for SMTP/IMAP auth

### Sending Emails (SMTP via Python)

```python
import smtplib, os
from email.mime.text import MIMEText

msg = MIMEText("Email body here")
msg["Subject"] = "Subject line"
msg["From"] = os.environ["GMAIL_ADDRESS"]
msg["To"] = "recipient@example.com"

with smtplib.SMTP("smtp.gmail.com", 587) as server:
    server.starttls()
    server.login(os.environ["GMAIL_ADDRESS"], os.environ["GMAIL_APP_PASSWORD"])
    server.send_message(msg)
```

### Reading Emails (IMAP via Python)

```python
import imaplib, email, os

mail = imaplib.IMAP4_SSL("imap.gmail.com")
mail.login(os.environ["GMAIL_ADDRESS"], os.environ["GMAIL_APP_PASSWORD"])
mail.select("inbox")

# Search for unread messages
status, messages = mail.search(None, "UNSEEN")
for num in messages[0].split()[:10]:  # Limit to 10
    status, data = mail.fetch(num, "(RFC822)")
    msg = email.message_from_bytes(data[0][1])
    print(f"From: {msg['from']}, Subject: {msg['subject']}")

mail.logout()
```

### Search Queries (IMAP)

| Query | Description |
|-------|-------------|
| `UNSEEN` | Unread messages |
| `FROM "someone@example.com"` | From specific sender |
| `SUBJECT "keyword"` | Subject contains keyword |
| `SINCE "01-Jan-2026"` | Messages since date |
| `UNSEEN FROM "boss@company.com"` | Combine criteria |

### Guidelines

- Use Python's `smtplib` and `imaplib` — they're in the standard library, no pip install needed
- Rate limit: Don't send more than 5 emails in a single job run
- Never expose full email content in logs — summarize rather than quote
- For HTML emails, use `MIMEMultipart` with `MIMEText(html, "html")` attached
- Always `mail.logout()` after IMAP operations
- The app password is pre-configured — do not ask the user for credentials
```

---

## Linear Plugin Implementation

### `internal/plugin/linear/plugin.go`

```go
package linear

import (
    "context"
    _ "embed"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/DikaVer/opencrons/internal/platform"
)

//go:embed skill.md
var skillTemplate string

type Plugin struct{}

func (p *Plugin) ID() string          { return "linear" }
func (p *Plugin) Name() string        { return "Linear" }
func (p *Plugin) Description() string { return "Track issues and projects in Linear" }

func (p *Plugin) SetupFields() []platform.SetupField {
    return []platform.SetupField{
        {
            Key:         "api_key",
            Label:       "API Key",
            Description: "Create at linear.app/settings/api",
            Required:    true,
            Secret:      true,
            Validate: func(s string) error {
                if !strings.HasPrefix(s, "lin_api_") {
                    return fmt.Errorf("Linear API key should start with lin_api_")
                }
                return nil
            },
        },
        {
            Key:         "default_team",
            Label:       "Default Team Key",
            Description: "Team identifier for new issues (e.g., 'ENG'). Optional.",
            Required:    false,
        },
    }
}

func (p *Plugin) Validate(cfg platform.PluginConfig) error {
    key := cfg.Settings["api_key"]
    if key == "" {
        return fmt.Errorf("Linear API key is required")
    }

    // Validate by calling the Linear GraphQL API with a 10s timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "POST", "https://api.linear.app/graphql",
        strings.NewReader(`{"query":"{ viewer { id name } }"}`))
    if err != nil {
        return fmt.Errorf("building Linear API request: %w", err)
    }
    req.Header.Set("Authorization", key)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return fmt.Errorf("Linear API request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("Linear API returned status %d — check your API key", resp.StatusCode)
    }
    return nil
}

func (p *Plugin) Skill(cfg platform.PluginConfig) string {
    skill := skillTemplate
    if team := cfg.Settings["default_team"]; team != "" {
        skill = strings.ReplaceAll(skill, "{{DEFAULT_TEAM}}", team)
    }
    return skill
}

func (p *Plugin) EnvVars(cfg platform.PluginConfig) map[string]string {
    vars := make(map[string]string)
    if key := cfg.Settings["api_key"]; key != "" {
        vars["LINEAR_API_KEY"] = key
    }
    return vars
}

func (p *Plugin) IsConfigured(cfg platform.PluginConfig) bool {
    return cfg.Settings["api_key"] != ""
}
```

### `internal/plugin/linear/skill.md`

```markdown
---
name: linear
description: Create and manage Linear issues, projects, and cycles.
---

## Linear Integration

You can interact with Linear via its GraphQL API. Authentication is pre-configured via the `LINEAR_API_KEY` environment variable.

### Common Operations

**List my assigned issues:**
```bash
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $LINEAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ viewer { assignedIssues(first: 20, filter: { state: { type: { nin: [\"completed\", \"canceled\"] } } }) { nodes { identifier title state { name } priority } } } }"}' | jq '.data.viewer.assignedIssues.nodes'
```

**Create an issue:**
```bash
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $LINEAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation($input: IssueCreateInput!) { issueCreate(input: $input) { success issue { identifier title url } } }",
    "variables": {
      "input": {
        "teamId": "TEAM_ID",
        "title": "Issue title",
        "description": "Issue description in markdown",
        "priority": 2
      }
    }
  }' | jq '.data.issueCreate.issue'
```

**Update an issue:**
```bash
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $LINEAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation($id: String!, $input: IssueUpdateInput!) { issueUpdate(id: $id, input: $input) { success } }",
    "variables": {
      "id": "ISSUE_UUID",
      "input": { "stateId": "STATE_UUID" }
    }
  }' | jq
```

**Search issues:**
```bash
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $LINEAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ searchIssues(term: \"search query\", first: 10) { nodes { identifier title state { name } } } }"}' | jq '.data.searchIssues.nodes'
```

**List teams (to get team IDs):**
```bash
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $LINEAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ teams { nodes { id key name } } }"}' | jq '.data.teams.nodes'
```

**List workflow states (to get state IDs):**
```bash
curl -s -X POST https://api.linear.app/graphql \
  -H "Authorization: $LINEAR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ workflowStates(first: 50) { nodes { id name type team { key } } } }"}' | jq '.data.workflowStates.nodes'
```

### Priority Levels

| Value | Meaning |
|-------|---------|
| 0 | No priority |
| 1 | Urgent |
| 2 | High |
| 3 | Medium |
| 4 | Low |

### Guidelines

- Always use `jq` to parse GraphQL responses
- Use issue identifiers (e.g., `ENG-123`) in user-facing output, not UUIDs
- When creating issues, always include `teamId` — query teams first if unknown
- Linear's GraphQL API requires `POST` — there are no REST endpoints
- Keep descriptions in markdown format — Linear renders them natively
- Rate limit: 1500 requests per hour. Don't poll in tight loops.
```

---

## Credential Delivery to the LLM

Plugins store credentials in `settings.json`, but the LLM needs access to them at runtime. The `EnvVars(cfg)` method on each plugin declares the mapping.

### How it works

1. The executor (or chat runner) loads plugin configs via `platform.GetPluginConfigs()`
2. For each enabled plugin, calls `p.EnvVars(cfg)` to get `map[string]string`
3. Appends each key-value pair to `cmd.Env` before executing `claude -p`

```go
// injectPluginEnv sets environment variables declared by enabled plugins.
// Called from both executor.BuildCommand() and chat.Runner.Run().
func injectPluginEnv(cmd *exec.Cmd, cfgs map[string]platform.PluginConfig) {
    if cmd.Env == nil {
        cmd.Env = os.Environ()
    }
    for _, p := range plugin.ListEnabled(cfgs) {
        cfg := cfgs[p.ID()]
        for key, val := range p.EnvVars(cfg) {
            cmd.Env = append(cmd.Env, key+"="+val)
        }
    }
}
```

The skill instructions reference these environment variables (`$GITHUB_TOKEN`, `$LINEAR_API_KEY`, `$GMAIL_APP_PASSWORD`), so the LLM uses them in its commands without ever seeing the raw value in the prompt text.

### Alternative: CLI tool auth (for `gh`)

The `gh` CLI has its own auth system. During plugin setup, you could also run `gh auth login --with-token` to configure it once. Then `gh` commands work without any env var. The `EnvVars()` approach covers this too — `gh` respects `GITHUB_TOKEN` in the environment.

---

## Affected Files Summary

| File | Change |
|------|--------|
| `internal/platform/settings.go` | **MODIFY** — Add `PluginConfig`, `SetupField` types, `Plugins` field to `Settings`, add `GetPluginConfigs()` |
| `internal/plugin/plugin.go` | **NEW** — `Plugin` interface (references `platform.PluginConfig`, `platform.SetupField`) |
| `internal/plugin/registry.go` | **NEW** — Plugin registry (Get, List, ListEnabled) |
| `internal/plugin/workspace.go` | **NEW** — SyncWorkspaceSkills |
| `internal/plugin/github/plugin.go` | **NEW** — GitHub plugin |
| `internal/plugin/github/skill.md` | **NEW** — GitHub skill for LLM |
| `internal/plugin/gmail/plugin.go` | **NEW** — Gmail plugin |
| `internal/plugin/gmail/skill.md` | **NEW** — Gmail skill for LLM |
| `internal/plugin/linear/plugin.go` | **NEW** — Linear plugin |
| `internal/plugin/linear/skill.md` | **NEW** — Linear skill for LLM |
| `internal/executor/claude.go` | **MODIFY** — Inject plugin skills into prompt, call `injectPluginEnv()` |
| `internal/tui/setup_wizard.go` | **MODIFY** — Add plugin setup step, wire `SetupField.Validate` into huh |
| `internal/tui/settings_menu.go` | **MODIFY** — Add plugins sub-menu |
| `internal/cmd/setup.go` | **MODIFY** — Persist plugin configs, call `SyncWorkspaceSkills()` |
| `internal/cmd/settings.go` | **MODIFY** — Handle plugin settings changes, call `SyncWorkspaceSkills()` |
| `internal/chat/runner.go` | **MODIFY** — Call `injectPluginEnv()` for chat commands |
| `.workspace-example/AGENTS.md` | **MODIFY** — Document available plugins |

---

## Implementation Order

1. **Platform types** — Add `PluginConfig` and `SetupField` to `internal/platform/settings.go`, add `Plugins` field to `Settings`
2. **Plugin interface and registry** — `plugin.go`, `registry.go` (imports `platform`)
3. **Workspace sync** — `workspace.go` (depends on registry + platform)
4. **First plugin (GitHub)** — Simplest to validate, `gh` CLI is widely available
5. **Executor injection** — Modify `BuildCommand()` to append plugin skills + inject env vars
6. **Chat runner injection** — Call `injectPluginEnv()` in `chat.Runner.Run()`
7. **TUI setup step** — Add to setup wizard (depends on plugin interface + platform types)
8. **TUI settings menu** — Add plugins sub-menu
9. **Remaining plugins** — Gmail, Linear (same pattern as GitHub)
10. **Workspace skill files** — Call `SyncWorkspaceSkills()` after setup and settings changes

---

## Checklist for New Plugins

- [ ] Implement `Plugin` interface (ID, Name, Description, SetupFields, Validate, Skill, EnvVars, IsConfigured)
- [ ] Create `skill.md` with complete LLM instructions (embedded via `//go:embed`)
- [ ] Field-level validation in `SetupFields()` (token format, file existence, etc.)
- [ ] API-level validation in `Validate()` — use explicit HTTP timeout (10s), actually call the service
- [ ] `EnvVars(cfg)` returns all credential env vars the skill references
- [ ] Register in `registry.go` (add to `plugins` map and `pluginOrder` slice)
- [ ] Test setup flow end-to-end (TUI collects → validates → persists → skill appears in workspace)
- [ ] Test skill injection in job execution (prompt includes plugin skill text)
- [ ] Test env var injection in **both** `executor/claude.go` and `chat/runner.go`
- [ ] Test workspace sync (skill file created/removed when plugin toggled)
- [ ] Test credential security (no raw tokens in prompts or logs — only in env vars)
- [ ] Update `.workspace-example/AGENTS.md` to mention the new plugin

# CLI Adapter Guide

This document describes how opencrons integrates CLI tools and how to add support for new ones (Codex CLI, Gemini CLI, Aider, Goose, etc.).

## Current State

All CLI logic is **hardcoded to Claude Code** across three packages:

| Package | File | Responsibility |
|---------|------|----------------|
| `executor` | `claude.go` | Build `claude -p` command for scheduled/manual jobs |
| `executor` | `executor.go` | Orchestrate execution lifecycle, parse JSON output |
| `chat` | `runner.go` | Build `claude -p --session-id` command for Telegram chat |
| `provider` | `anthropic.go` | Detect `claude` binary, check auth, report version |

The `provider.Provider` interface exists but is **only used during setup** — the executor ignores it entirely and hardcodes `"claude"`.

---

## Architecture for Multi-CLI Support

To support multiple CLI tools, we need a `CliAdapter` interface that each tool implements. The adapter handles three concerns:

1. **Detection** — Is the CLI binary available? Is it authenticated?
2. **Command building** — Translate `JobConfig` fields into tool-specific flags
3. **Output parsing** — Extract response text, cost, and token usage from tool-specific JSON

### Proposed Interface

```go
// Package executor

// CliAdapter abstracts a CLI coding tool (Claude Code, Codex, Gemini, etc.)
// so the executor can build and parse commands without knowing tool specifics.
type CliAdapter interface {
    // Identity
    ID() string   // "claude" | "codex" | "gemini" | "aider" | "goose"
    Name() string // Display name for TUI/logs

    // Detection & setup
    BinaryName() string    // "claude" | "codex" | "gemini" | "aider" | "goose"
    Detect() bool          // exec.LookPath(BinaryName())
    CheckAuth() error      // verify credentials are configured
    Version() string       // CLI version string

    // Capabilities — what this tool supports
    Caps() AdapterCaps

    // Command building — scheduled/manual job execution
    BuildJobCommand(ctx context.Context, prompt string, opts JobOpts) (*exec.Cmd, error)

    // Command building — interactive chat session
    BuildChatCommand(ctx context.Context, message string, opts ChatOpts) (*exec.Cmd, error)

    // Output parsing
    ParseJobOutput(stdout []byte) (*JobOutput, error)
    ParseChatOutput(stdout []byte) (*ChatOutput, error)
}
```

### Supporting Types

```go
// AdapterCaps declares what features this CLI adapter supports.
type AdapterCaps struct {
    JSONOutput        bool // can produce structured JSON output
    SessionPersist    bool // supports session/resume for chat
    EffortLevels      bool // supports effort/reasoning level control
    DisallowedTools   bool // supports tool restriction
    SummaryInjection  bool // prompt-based, works with any tool (always true)
    StdinPrompt       bool // accepts prompt via stdin
    ModelSelection    bool // supports --model flag
    CostTracking      bool // output includes cost/token data
}

// JobOpts contains the fields an adapter needs to build a job command.
type JobOpts struct {
    Model            string
    Effort           string
    WorkingDir       string
    DisallowedTools  []string
    NoSessionPersist bool
}

// ChatOpts contains the fields an adapter needs to build a chat command.
type ChatOpts struct {
    SessionID  string
    IsNew      bool   // true = new session, false = resume existing
    Model      string
    Effort     string
    WorkingDir string
}

// JobOutput is the parsed result from a scheduled/manual job.
type JobOutput struct {
    CostUSD             float64
    InputTokens         int
    OutputTokens        int
    CacheReadTokens     int
    CacheCreationTokens int
}

// ChatOutput is the parsed result from a chat interaction.
type ChatOutput struct {
    Response string
    CostUSD  float64
    Tokens   int // total (input + output)
}
```

### Relationship to `provider.Provider`

The existing `provider.Provider` interface covers detection/auth/version only and is used during setup. Rather than maintaining two parallel registries, `CliAdapter` should **replace** `provider.Provider` entirely:

1. `CliAdapter` is a superset — it includes `Detect()`, `CheckAuth()`, `Version()` plus command building and output parsing
2. The setup wizard should be updated to use `ListAdapters()` instead of `provider.List()`
3. The `provider` package can be removed once all its callers migrate to the adapter registry

### Registry

```go
// adapterOrder controls TUI display order (map iteration is non-deterministic in Go).
var adapterOrder = []string{"claude", "codex", "gemini", "aider", "goose"}

var adapters = map[string]CliAdapter{
    "claude": &ClaudeAdapter{},
    "codex":  &CodexAdapter{},
    "gemini": &GeminiAdapter{},
    // register new adapters here
}

func GetAdapter(id string) CliAdapter { return adapters[id] }

func ListAdapters() []CliAdapter {
    list := make([]CliAdapter, 0, len(adapterOrder))
    for _, id := range adapterOrder {
        if a, ok := adapters[id]; ok {
            list = append(list, a)
        }
    }
    return list
}
```

---

## CLI Tool Reference

### Claude Code (`claude`)

**Binary:** `claude`
**Install:** `npm install -g @anthropic-ai/claude-code`

| Concern | Implementation |
|---------|----------------|
| Non-interactive mode | `-p` flag |
| Prompt delivery | stdin (piped) |
| JSON output | `--output-format json` |
| Permission bypass | `--permission-mode bypassPermissions` |
| Model selection | `--model <model>` |
| Effort control | `--effort <low\|medium\|high\|max>` |
| Tool restriction | `--disallowedTools <tool1> <tool2>...` |
| Session (new) | `--session-id <uuid>` |
| Session (resume) | `--resume <uuid>` |
| No persistence | `--no-session-persistence` |
| Working directory | `cmd.Dir = path` |

**JSON output structure (job):**
```json
{
  "total_cost_usd": 0.1234,
  "usage": {
    "input_tokens": 500,
    "output_tokens": 200,
    "cache_read_input_tokens": 50,
    "cache_creation_input_tokens": 25
  }
}
```

**JSON output structure (chat):**
```json
{
  "result": "response text",
  "total_cost_usd": 0.01,
  "usage": { "input_tokens": 100, "output_tokens": 150 }
}
```

**Models:** `sonnet`, `opus`, `haiku` (or full model IDs like `claude-sonnet-4-6`)

**Adapter implementation:** See `claude.go` (current hardcoded version).

---

### Codex CLI (`codex`)

**Binary:** `codex`
**Install:** `npm install -g @openai/codex`

| Concern | Implementation |
|---------|----------------|
| Non-interactive mode | `exec` subcommand |
| Prompt delivery | positional arg or `-` for stdin |
| JSON output | `--json` (JSONL, newline-delimited events) |
| Permission bypass | `--full-auto` or `--approval-policy auto` |
| Model selection | `--model <model>` or `-m` |
| Effort control | Not supported |
| Tool restriction | Not supported |
| Session (new) | Automatic (stored in `$CODEX_HOME/sessions/`) |
| Session (resume) | `exec resume --last` or `exec resume <uuid>` |
| No persistence | `--ephemeral` |
| Working directory | `--cd <path>` or `-C <path>` |

**Command construction (job):**
```go
// codex exec --full-auto --json --model <model> -C <dir> -
args := []string{"exec", "--full-auto", "--json"}
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
args = append(args, "-C", opts.WorkingDir, "-")
cmd := exec.CommandContext(ctx, "codex", args...)
cmd.Stdin = strings.NewReader(prompt)
```

> **Note:** The `--full-auto` flag has changed across Codex CLI versions. Older versions
> use `--approval-policy auto` or `--dangerously-auto-approve-everything`. Verify against
> the installed version with `codex exec --help`.

**Command construction (chat — resume):**
```go
// codex exec resume <session-id> --full-auto --json --model <model> -C <dir> -
args := []string{"exec"}
if !opts.IsNew {
    args = append(args, "resume", opts.SessionID)
}
args = append(args, "--full-auto", "--json")
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
// -C must come before the stdin marker "-"
args = append(args, "-C", opts.WorkingDir, "-")
```

**JSON output structure (JSONL — key events):**
```jsonl
{"type":"thread.started","thread_id":"..."}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"...","type":"agent_message","text":"response text"}}
{"type":"turn.completed","usage":{"input_tokens":1234,"cached_input_tokens":500,"output_tokens":567}}
```

**Parsing strategy:** Read JSONL line-by-line, extract the last `item.completed` with `type: "agent_message"` for the response text, and the last `turn.completed` for token usage.

```go
func (a *CodexAdapter) ParseJobOutput(stdout []byte) (*JobOutput, error) {
    result := &JobOutput{}
    scanner := bufio.NewScanner(bytes.NewReader(stdout))
    for scanner.Scan() {
        var event map[string]interface{}
        if json.Unmarshal(scanner.Bytes(), &event) != nil {
            continue
        }
        if event["type"] == "turn.completed" {
            if usage, ok := event["usage"].(map[string]interface{}); ok {
                result.InputTokens = int(usage["input_tokens"].(float64))
                result.OutputTokens = int(usage["output_tokens"].(float64))
            }
        }
    }
    return result, nil
}
```

**Models:** Model names change frequently. Check `codex models` or the OpenAI documentation at implementation time. Examples at time of writing: `o4-mini`, `codex-mini-latest`, `gpt-4.1`.

**Capabilities:**
```go
func (a *CodexAdapter) Caps() AdapterCaps {
    return AdapterCaps{
        JSONOutput:       true,  // JSONL via --json
        SessionPersist:   true,  // exec resume <uuid>
        EffortLevels:     false, // not supported
        DisallowedTools:  false, // not supported
        SummaryInjection: true,  // prompt-based, always works
        StdinPrompt:      true,  // codex exec -
        ModelSelection:   true,  // --model
        CostTracking:     false, // usage tokens yes, cost USD not in output
    }
}
```

---

### Gemini CLI (`gemini`)

**Binary:** `gemini`
**Install:** `npm install -g @google/gemini-cli`

> **Warning:** Gemini CLI is newer than Claude Code and Codex CLI. Flags and JSON output
> schema may change between versions. Validate all examples against the installed binary
> before implementing. Run `gemini --help` to verify flag names.

| Concern | Implementation |
|---------|----------------|
| Non-interactive mode | `--prompt "text"` or `-p "text"` (takes prompt as value) |
| Prompt delivery | `--prompt "text"` or stdin pipe (without `-p`) |
| JSON output | `--output-format json` |
| Permission bypass | `--yolo` or `-y` |
| Model selection | `--model <model>` or `-m` |
| Effort control | Not supported |
| Tool restriction | Not supported |
| Session (new) | Automatic |
| Session (resume) | `--resume` (latest) or `--resume <uuid>` |
| No persistence | Not supported (sessions always recorded) |
| Working directory | `cmd.Dir = path` (run from directory) |
| Sandbox | `--sandbox` or `-s` |

**Command construction (job):**
```go
// Option A: prompt via --prompt flag
// gemini --prompt "..." --output-format json --yolo --model <model>
args := []string{"--output-format", "json", "--yolo"}
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
args = append(args, "--prompt", prompt)
cmd := exec.CommandContext(ctx, "gemini", args...)
cmd.Dir = opts.WorkingDir

// Option B: prompt via stdin pipe (verify this works with your version)
// echo "..." | gemini --output-format json --yolo --model <model>
args := []string{"--output-format", "json", "--yolo"}
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
cmd := exec.CommandContext(ctx, "gemini", args...)
cmd.Stdin = strings.NewReader(prompt)
cmd.Dir = opts.WorkingDir
```

> **Note:** The `-p` flag is shorthand for `--prompt` and **takes the prompt text as
> its value** (e.g., `-p "do something"`). It is NOT a bare mode flag like Claude's `-p`.
> For stdin-based prompt delivery, omit `-p` entirely and pipe via `cmd.Stdin`.
> Test both approaches with your installed version before choosing.

**Command construction (chat — resume):**
```go
// gemini --prompt "..." --output-format json --yolo --model <model> --resume <uuid>
args := []string{"--output-format", "json", "--yolo"}
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
if !opts.IsNew {
    args = append(args, "--resume", opts.SessionID)
}
args = append(args, "--prompt", message)
cmd := exec.CommandContext(ctx, "gemini", args...)
cmd.Dir = opts.WorkingDir
```

**JSON output structure (unverified — validate against installed binary):**
```json
{
  "response": "The response text",
  "stats": {
    "models": {
      "gemini-2.5-flash": {
        "tokens": {
          "prompt": 1200,
          "candidates": 800,
          "total": 2000,
          "cached": 0,
          "thoughts": 150,
          "tool": 50
        }
      }
    }
  },
  "error": null
}
```

> **Important:** This JSON schema has not been verified against all Gemini CLI versions.
> The `--output-format json` flag has had [compatibility issues](https://github.com/google-gemini/gemini-cli/issues/9009)
> in some releases. Before implementing, run `gemini --prompt "hello" --output-format json`
> and inspect the actual output to confirm the schema.

**Parsing strategy:** Single JSON object. Extract `response` for text, iterate `stats.models` to sum tokens across models. The parsing code below is a best-effort template — adjust field names to match the actual output.

```go
type geminiOutput struct {
    Response string `json:"response"`
    Stats    struct {
        Models map[string]struct {
            Tokens struct {
                Prompt     int `json:"prompt"`
                Candidates int `json:"candidates"`
                Total      int `json:"total"`
                Cached     int `json:"cached"`
            } `json:"tokens"`
        } `json:"models"`
    } `json:"stats"`
    Error *struct {
        Message string `json:"message"`
    } `json:"error"`
}

func (a *GeminiAdapter) ParseJobOutput(stdout []byte) (*JobOutput, error) {
    var out geminiOutput
    if err := json.Unmarshal(stdout, &out); err != nil {
        return nil, err
    }
    result := &JobOutput{}
    for _, m := range out.Stats.Models {
        result.InputTokens += m.Tokens.Prompt
        result.OutputTokens += m.Tokens.Candidates
    }
    return result, nil
}
```

**Models:** `gemini-2.5-pro` (default), `gemini-2.5-flash`, `gemini-2.5-flash-lite`, `gemini-3-pro`, `gemini-3-flash`

**Capabilities:**
```go
func (a *GeminiAdapter) Caps() AdapterCaps {
    return AdapterCaps{
        JSONOutput:       true,  // --output-format json (verify version compatibility)
        SessionPersist:   true,  // --resume <uuid>
        EffortLevels:     false, // not supported
        DisallowedTools:  false, // not supported
        SummaryInjection: true,  // prompt-based, always works
        StdinPrompt:      true,  // stdin pipe (verify with installed version)
        ModelSelection:   true,  // --model
        CostTracking:     false, // tokens yes, cost USD not in output
    }
}
```

**Notes:**
- Gemini uses `GEMINI.md` for project context (analogous to `CLAUDE.md`).
- No cost-per-request in output. Token counts are available per model.
- No effort/reasoning level control.
- Auth via `GEMINI_API_KEY` env var or Google OAuth. `CheckAuth()` should verify the env var exists.

---

### Aider (`aider`)

**Binary:** `aider`
**Install:** `pip install aider-chat`

| Concern | Implementation |
|---------|----------------|
| Non-interactive mode | `--message "text"` or `-m` |
| Prompt delivery | `--message "text"` or `--message-file path` |
| JSON output | **Not supported** |
| Permission bypass | `--yes` or `--yes-always` |
| Model selection | `--model <model>` |
| Effort control | Not supported |
| Tool restriction | Not supported |
| Session (resume) | `--restore-chat-history` |
| Working directory | `cmd.Dir = path` |
| Files to edit | `--file <path>` (repeatable) |

**Command construction (job):**
```go
// aider --yes --message-file <prompt-path> --model <model>
args := []string{"--yes", "--no-auto-commits"}
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
args = append(args, "--message-file", promptPath)
cmd := exec.CommandContext(ctx, "aider", args...)
cmd.Dir = opts.WorkingDir
```

**Parsing strategy:** Aider has **no JSON output mode**. The adapter must either:
1. Treat stdout as plain text (no cost/token tracking)
2. Parse Aider's human-readable output for token counts (fragile, not recommended)

```go
func (a *AiderAdapter) ParseJobOutput(stdout []byte) (*JobOutput, error) {
    // Aider has no structured output — return empty metrics
    return &JobOutput{}, nil
}

func (a *AiderAdapter) ParseChatOutput(stdout []byte) (*ChatOutput, error) {
    // Aider output contains ANSI escape codes and decorative text.
    // Use --no-pretty flag when building the command, and strip any
    // remaining ANSI sequences before returning.
    cleaned := stripANSI(string(stdout))
    return &ChatOutput{
        Response: strings.TrimSpace(cleaned),
    }, nil
}

// stripANSI removes ANSI escape sequences from text.
// Example regex: \x1b\[[0-9;]*[a-zA-Z]
```

**Models:** Multi-provider — works with any model string supported by the underlying provider (OpenAI, Anthropic, Google, local via LiteLLM). Examples: `gpt-4o`, `claude-sonnet-4-6`, `gemini-2.5-pro`.

**Capabilities:**
```go
func (a *AiderAdapter) Caps() AdapterCaps {
    return AdapterCaps{
        JSONOutput:       false, // no structured output
        SessionPersist:   false, // directory-scoped, not UUID-scoped (see notes)
        EffortLevels:     false,
        DisallowedTools:  false,
        SummaryInjection: true,  // prompt-based
        StdinPrompt:      false, // uses --message or --message-file
        ModelSelection:   true,  // --model
        CostTracking:     false, // no cost in output
    }
}
```

**Notes:**
- Aider auto-commits changes by default. Use `--no-auto-commits` for scheduled jobs to keep git control with the user.
- For scheduled jobs, `--message-file` is preferred over `--message` to avoid shell escaping issues.
- **Session semantics differ fundamentally:** Aider's `--restore-chat-history` restores from `.aider.chat.history.md` in the working directory — it is not keyed by UUID. The `ChatOpts.SessionID` is meaningless for Aider. If two chat sessions share the same `WorkingDir`, they share the same history file. Set `SessionPersist: false` in capabilities and do not pass session IDs.
- Aider's stdout includes ANSI color codes, diff output, and decorative text. Always pass `--no-pretty` when building commands, and strip remaining ANSI sequences before using the response.

---

### Goose (`goose`)

**Binary:** `goose`
**Install:** `brew install goose` or from [GitHub](https://github.com/block/goose)

| Concern | Implementation |
|---------|----------------|
| Non-interactive mode | `run` subcommand |
| Prompt delivery | `-t "text"` or `-i <file>` or `-i -` for stdin |
| JSON output | `--output-format json` |
| Permission bypass | Not applicable (Goose manages its own sandbox) |
| Model selection | `--model <model>` + `--provider <provider>` |
| Effort control | Not supported |
| Tool restriction | Not supported |
| Session control | `--no-session` to disable persistence |
| Working directory | `cmd.Dir = path` |

**Command construction (job):**
```go
// goose run --output-format json --no-session --model <model> -i -
args := []string{"run", "--output-format", "json", "--no-session"}
if opts.Model != "" {
    args = append(args, "--model", opts.Model)
}
args = append(args, "-i", "-")
cmd := exec.CommandContext(ctx, "goose", args...)
cmd.Stdin = strings.NewReader(prompt)
cmd.Dir = opts.WorkingDir
```

**JSON output structure:** The exact schema for `--output-format json` has not been verified. Before implementing, run `goose run --output-format json -t "hello"` and inspect the actual output. Implement `ParseJobOutput` based on the real schema.

```go
func (a *GooseAdapter) ParseJobOutput(stdout []byte) (*JobOutput, error) {
    // TODO: Implement based on actual goose --output-format json schema.
    // Run `goose run --output-format json -t "hello"` to discover the format.
    return &JobOutput{}, nil
}
```

**Models:** Multi-provider — `--provider anthropic --model claude-sonnet-4-6`, `--provider openai --model gpt-4o`, etc.

**Capabilities:**
```go
func (a *GooseAdapter) Caps() AdapterCaps {
    return AdapterCaps{
        JSONOutput:       true,  // --output-format json (verify schema before implementing)
        SessionPersist:   true,  // default on, --no-session to disable
        EffortLevels:     false,
        DisallowedTools:  false,
        SummaryInjection: true,  // prompt-based
        StdinPrompt:      true,  // -i -
        ModelSelection:   true,  // --model + --provider
        CostTracking:     false,
    }
}
```

**Notes:**
- Goose has a built-in `goose schedule` command with cron support. Consider whether to use it or opencrons' own scheduler.
- Sessions are persisted by default. Use `--no-session` for scheduled jobs where persistence is unwanted.
- Session resume: `goose session list --format json` lists sessions. Resume support exists but the exact flags should be verified.

---

## Comparison Matrix

| Feature | Claude | Codex | Gemini | Aider | Goose |
|---------|--------|-------|--------|-------|-------|
| JSON output | Single object | JSONL stream | Single object | No | Single object |
| Stdin prompt | Yes | `-` flag | Yes | No (`--message-file`) | `-i -` |
| Auto-approve | `--permission-mode bypassPermissions` | `--full-auto` | `--yolo` | `--yes` | N/A |
| Session resume | `--resume <uuid>` | `exec resume <uuid>` | `--resume <uuid>` | N/A (dir-scoped) | `--no-session` to disable |
| Cost in output | Yes (USD) | No | No | No | No |
| Token tracking | Yes | Yes | Yes | No | No |
| Effort levels | Yes | No | No | No | No |
| Tool restriction | Yes | No | No | No | No |

---

## Implementation Checklist for a New Adapter

### 1. Create the adapter file

Create `internal/executor/<tool>.go`:

```go
package executor

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
)

type MyToolAdapter struct{}

func (a *MyToolAdapter) ID() string         { return "mytool" }
func (a *MyToolAdapter) Name() string       { return "My Tool" }
func (a *MyToolAdapter) BinaryName() string { return "mytool" }

func (a *MyToolAdapter) Detect() bool {
    _, err := exec.LookPath(a.BinaryName())
    return err == nil
}

func (a *MyToolAdapter) CheckAuth() error {
    // Verify the binary is available
    if err := exec.Command(a.BinaryName(), "--version").Run(); err != nil {
        return fmt.Errorf("%s not found or not executable: %w", a.BinaryName(), err)
    }
    // Also check for required env vars (e.g., API keys).
    // Example: if os.Getenv("MYTOOL_API_KEY") == "" { return fmt.Errorf("...") }
    return nil
}

func (a *MyToolAdapter) Version() string {
    out, err := exec.Command(a.BinaryName(), "--version").Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}

func (a *MyToolAdapter) Caps() AdapterCaps {
    return AdapterCaps{ /* fill in */ }
}

func (a *MyToolAdapter) BuildJobCommand(ctx context.Context, prompt string, opts JobOpts) (*exec.Cmd, error) {
    // Build the command with tool-specific flags
    // Set cmd.Stdin if the tool reads prompt from stdin
    // Set cmd.Dir = opts.WorkingDir
    return nil, fmt.Errorf("MyToolAdapter.BuildJobCommand: not implemented")
}

func (a *MyToolAdapter) BuildChatCommand(ctx context.Context, message string, opts ChatOpts) (*exec.Cmd, error) {
    return nil, fmt.Errorf("MyToolAdapter.BuildChatCommand: not implemented")
}

func (a *MyToolAdapter) ParseJobOutput(stdout []byte) (*JobOutput, error) {
    return nil, fmt.Errorf("MyToolAdapter.ParseJobOutput: not implemented")
}

func (a *MyToolAdapter) ParseChatOutput(stdout []byte) (*ChatOutput, error) {
    return nil, fmt.Errorf("MyToolAdapter.ParseChatOutput: not implemented")
}
```

### 2. Register the adapter

Add to the registry in the adapter registry file:

```go
adapters["mytool"] = &MyToolAdapter{}
```

### 3. Update JobConfig

Add a `Provider` field to `config.JobConfig` so each job knows which CLI tool to use:

```go
type JobConfig struct {
    // ... existing fields ...
    Provider string `yaml:"provider"` // "claude" | "codex" | "gemini" | etc.
}
```

### 4. Update the executor

Modify `executor.Run()` to look up the adapter. The executor is responsible for:
- Building the full prompt (preamble + user prompt + optional summary injection) — this is **not** the adapter's job
- Calling `adapter.BuildJobCommand()` with the assembled prompt
- Managing the `SummaryPath` for the `Result` struct

```go
func Run(ctx context.Context, db *storage.DB, job *config.JobConfig, triggerType string) (*Result, error) {
    adapter := GetAdapter(job.Provider)
    if adapter == nil {
        return nil, fmt.Errorf("unknown provider %q", job.Provider)
    }

    // ... existing lifecycle code (validate dir, create log, setup files, timeout) ...

    // Build prompt: preamble + user prompt + optional summary injection
    // This stays in the executor, not in the adapter
    prompt := buildPrompt(job) // extracted from current claude.go logic
    summaryPath := ""
    if job.SummaryEnabled {
        summaryPath = computeSummaryPath(job)
        prompt += buildSummaryInjection(summaryPath, job)
    }

    cmd, err := adapter.BuildJobCommand(ctx, prompt, JobOpts{
        Model:           job.Model,
        Effort:          job.Effort,
        WorkingDir:      job.WorkingDir,
        DisallowedTools: job.DisallowedTools,
    })
    if err != nil {
        result := &Result{ExitCode: -1, Status: "failed", ErrorMsg: err.Error()}
        _ = db.UpdateLog(logID, time.Now(), -1, stdoutPath, stderrPath,
            0, 0, 0, 0, 0, "failed", err.Error())
        return result, nil
    }

    // ... run command, capture output ...

    output, err := adapter.ParseJobOutput(stdoutData)
    if err == nil {
        result.CostUSD = output.CostUSD
        result.InputTokens = output.InputTokens
        result.OutputTokens = output.OutputTokens
        result.CacheReadTokens = output.CacheReadTokens
        result.CacheCreationTokens = output.CacheCreationTokens
    }
    result.SummaryPath = summaryPath

    // ... update log ...
}
```

### 5. Update the TUI

Add provider selection to the job creation wizard (`internal/tui/`):
- Detect available adapters via `ListAdapters()` filtered by `Detect()`
- Show model options based on the selected adapter
- Hide unsupported fields (effort, disallowed tools) based on `Caps()`

### 6. Update validation

In `internal/ui/validators.go`, make model validation adapter-aware:
- Each adapter should define its own valid model list
- The TUI should filter model choices based on the selected adapter

### 7. Update chat runner

Modify `chat/runner.go` to use the adapter for chat commands:
- Store the provider ID in `ChatSession`
- Look up the adapter when building chat commands

---

## Key Design Decisions

### Prompt delivery

All adapters that support stdin should use it. This avoids shell escaping issues and OS argument length limits. For tools that don't support stdin (Aider), use `--message-file` and write a temp file.

### Task preamble and summary injection

The `task-preamble.txt` and `summary-prompt.txt` are **the executor's responsibility**, not the adapter's. The executor assembles the full prompt (preamble + user content + optional summary) and passes it to `BuildJobCommand()` as a single string. This keeps prompt logic centralized and avoids duplication across adapters.

When refactoring `claude.go` into a `ClaudeAdapter`, extract the prompt assembly logic into shared helpers in `executor.go` (e.g., `buildPrompt()`, `buildSummaryInjection()`) that run before calling any adapter.

### Cost tracking

Only Claude Code currently reports USD cost in its JSON output. For other tools, `CostUSD` will be 0. Token counts should be extracted where available for usage tracking.

### Unsupported features

When a `JobConfig` specifies a feature the adapter doesn't support (e.g., `Effort` with Codex), the adapter should **silently ignore it** rather than error. The TUI should prevent users from setting unsupported options, but defensive handling is still needed for manually-edited YAML configs.

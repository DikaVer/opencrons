# Messenger Adapter Guide

This document describes the contract and capabilities that messenger adapters must implement, based on the reference Telegram implementation.

---

## Interface

Every adapter must implement the `Messenger` interface defined in `messenger.go`:

```go
type Messenger interface {
    Type() string                          // Identifier: "telegram", "discord", "slack", etc.
    Start(ctx context.Context) error       // Blocking event loop — runs until ctx is cancelled
    Stop()                                 // Graceful shutdown — cancels the event loop
    Send(chatID int64, text string) error  // Send a formatted message to a chat
    IsAuthorized(userID int64) bool        // Check if a user is allowed to interact
}
```

### Lifecycle contract

1. **Construction** — `New(db, settings, stdlog)` creates the adapter, validates credentials, registers handlers.
2. **Dependency injection** — `SetChatComponents(sessionMgr, runner)` injects chat session manager and Claude runner. Must be called before `Start`.
3. **Start** — Blocks on the platform's event loop (long-polling, websocket, etc.). Called in a background goroutine by the daemon with `context.Background()` — the bot's lifecycle is controlled by `Stop()`, not by external context cancellation.
4. **Stop** — Cancels the event loop context. Called during daemon shutdown (SIGINT/SIGTERM).

> **Note:** The interface `Send` does not carry `context.Context`. Internally, the Telegram adapter uses concrete `Send(ctx, chatID, text)` and `SendPlain(ctx, chatID, text)` helpers that are not part of the interface. Adapters should follow the same pattern: satisfy the interface-compliant `Send(chatID, text)` and manage context internally, or expose additional context-aware helpers for internal use.

> **Note:** The current Telegram `Start` method does not return `error` and therefore does not fully satisfy the `Messenger` interface as defined. New adapters should implement the interface as written (returning `error`). A future refactor should align the Telegram adapter with the interface.

---

## Required Capabilities

### 1. Authentication & Authorization

| Requirement | Details |
|---|---|
| **Token validation** | Validate the bot/app token at construction time (e.g., API call like Telegram's `GetMe`) |
| **User allowlist** | Check every incoming message/interaction against `settings.AllowedUsers` map |
| **Auth middleware** | Wrap all handlers — reject unauthorized users before processing |
| **Rejection message** | Send a clear "not authorized" response to unauthorized users |

The allowlist is stored as `map[string]bool` where keys are string-encoded user IDs. Internal entries prefixed with `__` (like `__placeholder`) must be skipped.

```go
func (b *Bot) IsAuthorized(userID int64) bool {
    idStr := strconv.FormatInt(userID, 10)
    return b.settings.AllowedUsers[idStr]
}
```

### 2. Chat with Claude (Core Feature)

The primary function — users send text messages and get Claude responses.

#### Message flow

```
User sends text
  → Auth check
  → Acquire per-user lock (reject if already processing)
  → Get or create session (sessionMgr.GetOrCreateSession)
  → Store cancel func (for /stop support)
  → Start typing/processing indicator
  → Execute: chatRunner.Run(ctx, session, text, isNew)
  → Stop typing indicator
  → Log to database (db.AddChatLog for both user and assistant)
  → Send response (formatted, then plain text fallback)
  → Echo to terminal (stdlog)
```

#### Per-user concurrency control

Only one message per user can be processed at a time. Use `sync.Map` for lock-free per-user locks:

```go
processing sync.Map // map[int64]bool

func (b *Bot) tryLock(userID int64) bool {
    _, loaded := b.processing.LoadOrStore(userID, true)
    return !loaded
}

func (b *Bot) unlock(userID int64) {
    b.processing.Delete(userID)
}
```

If a user sends a message while one is processing, respond with "Please wait, still processing your previous message..."

#### Session error recovery

Handle these known failure modes:

| Error | Recovery |
|---|---|
| `"No conversation found"` | Clear session, create fresh one, retry with `isNew=true` |
| `"already in use"` | Wait 2 seconds, retry with same session and same `isNew` flag. Only if that also fails: clear session, create fresh one with `isNew=true` |
| Any other error | Send error message + suggest `/new` for a fresh session |

During each retry attempt, restart the typing/processing indicator. The implementation should spawn a new indicator goroutine for each attempt, not rely on the original indicator surviving across retry boundaries.

#### Typing/processing indicator

Show the user that Claude is working. Telegram sends `ChatActionTyping` immediately, then refreshes every 5 seconds until the response is ready. Adapt to whatever "typing" or "processing" indicator the platform supports.

Use a child context derived from the query's cancellable context for the typing loop. When the query is cancelled (e.g., via `/stop`), the typing loop stops automatically without needing a separate channel or flag.

### 3. Commands

Every adapter should support these commands (mapped to the platform's command system):

| Command | Action | Details |
|---|---|---|
| `/new` | Clear session | Deactivates current session via `sessionMgr.ClearSession(userID)` (falls back to `db.DeactivateUserSessions(userID)` if session manager is nil) — starts fresh conversation on next message |
| `/stop` | Cancel running query | Calls the stored `context.CancelFunc` for the user's in-flight Claude query |
| `/jobs` | List scheduled jobs | Loads jobs from `config.LoadAllJobs(platform.SchedulesDir())`, shows with enabled/disabled status |
| `/model` | Change AI model | Shows current model, lets user pick: `sonnet`, `opus`, `haiku` — updates via `db.UpdateSessionModel()` |
| `/effort` | Change effort level | Shows current effort, lets user pick: `low`, `medium`, `high`, `max` — updates via `db.UpdateSessionEffort()` |
| `/status` | Show status | Daemon PID, job counts (total/enabled), active session info (model, effort, working dir) |
| `/help` | Show help | List all available commands with descriptions |
| `/start` | Alias for `/help` | Sent automatically by some platforms (e.g., Telegram) when a user first opens the bot — must show help/welcome |

### 4. Job Management (Interactive)

When a user selects a job from `/jobs`, show these actions:

| Action | Behavior |
|---|---|
| **View details** | Show job name, schedule, model, enabled status |
| **Enable/Disable** | Toggle `job.Enabled`, save via `config.SaveJob()` |
| **Run Now** | Execute immediately via `executor.Run(ctx, db, job, "manual")`, send result summary |
| **Back** | Return to job list |

For job execution results:
1. If `result.SummaryPath` exists and is non-empty, send the summary file content
2. Otherwise, send a formatted message: status, duration, cost, error (if any)

### 5. Job Completion Notifications

Broadcast to all authorized users when scheduled jobs complete:

```go
func (b *Bot) NotifyJobComplete(ctx context.Context, jobName, status, summaryPath string)
```

- Iterate `settings.AllowedUsers`, skip entries starting with `__`
- Read summary file if `summaryPath` is non-empty
- Fall back to `"Job '<name>' completed: <status>"` message
- Try formatted send first, then plain text fallback
- Log notification count via `stdlog` (the standard logger passed to the constructor)

> **Note:** `NotifyJobComplete` is not part of the `Messenger` interface. The daemon currently holds a concrete `*telegram.Bot` via `atomic.Pointer[telegram.Bot]` and calls this method directly. To support multiple adapters, either introduce a `JobNotifier` interface (e.g., with `NotifyJobComplete(ctx, name, status, summaryPath)`) that all adapters implement, or add per-adapter dispatch in the daemon startup code. New adapters must coordinate this change with the daemon.

### 6. Message Formatting

Claude outputs standard markdown. Convert to the platform's native format:

| Markdown | What to handle |
|---|---|
| `# Header` | Platform-appropriate emphasis (bold, etc.) |
| `**bold**` | Bold |
| `*italic*` | Italic |
| `` `inline code` `` | Monospace/code span |
| ```` ```lang ``` ```` | Code block with optional language hint |
| `[text](url)` | Hyperlink |

Provide two send methods:
- **`Send(ctx, chatID, text)`** — converts markdown to platform-native format (e.g., HTML for Telegram)
- **`SendPlain(ctx, chatID, text)`** — sends raw text when formatting fails

Always try formatted first, fall back to plain on error.

> **Formatting order matters:** Apply HTML/entity escaping *before* markdown-to-markup substitution. The regex patterns must match on already-escaped text. For example, escape `&`, `<`, `>` first, then convert `**bold**` to `<b>bold</b>`. If you apply markdown substitutions first and then escape, the generated tags will be escaped too (e.g., `&lt;b&gt;`).

### 7. Cancel Support

Users must be able to cancel an in-flight Claude query:

```go
cancels sync.Map // map[int64]context.CancelFunc
```

- Store the `CancelFunc` when starting a query
- `/stop` command retrieves and calls it
- Clean up the entry when the query completes (in `defer`)

---

## Setup & Pairing

### Setup wizard integration

Each adapter needs a pairing mechanism for the TUI setup wizard (`internal/tui/setup_wizard.go`). The Telegram adapter uses a code-based flow:

1. **Token input** — User enters bot token in the TUI
2. **Token validation** — Start a temporary bot, call platform API to verify credentials
3. **Pairing loop**:
   - User sends a message to the bot on the platform
   - Bot generates a cryptographically random 6-digit code
   - Bot sends the code back to the user on the platform
   - User enters the code in the TUI
   - TUI validates the code and retrieves user info (ID, username, display name)
   - Repeat for additional users or finish
4. **Cleanup** — Stop the temporary pairing bot

The pairing result populates `settings.AllowedUsers`:

```go
type PairingResult struct {
    UserID   int64
    Username string
    Name     string
}
```

Adapters should expose:

```go
func StartPairingBot(token string) (*PairingBot, error)  // Start temp bot, validate token
func (pb *PairingBot) BotName() string                    // Display name for TUI
func (pb *PairingBot) ValidateCode(code string) (*PairingResult, error)  // Single-use code validation
func (pb *PairingBot) Stop()                               // Shutdown temp bot
```

Code generation must be cryptographically secure (`crypto/rand`, not `math/rand`).

---

## Settings

Adapter configuration lives in `settings.json` under the `messenger` key:

```json
{
  "messenger": {
    "type": "telegram",
    "bot_token": "123456:ABC-DEF...",
    "allowed_users": {
      "12345678": true,
      "87654321": true
    }
  }
}
```

Go struct (`internal/platform/settings.go`):

```go
type MessengerSettings struct {
    Type         string          `json:"type"`
    BotToken     string          `json:"bot_token"`
    AllowedUsers map[string]bool `json:"allowed_users"`
}
```

New adapters may need additional fields. Extend `MessengerSettings` or add platform-specific nested structs as needed. Ensure backward compatibility — existing settings files must still load correctly.

---

## Daemon Integration

The daemon (`internal/daemon/daemon.go`) manages the messenger lifecycle:

```go
// Startup — inside a dedicated startTelegramBot() helper
func (d *Daemon) startTelegramBot(db *storage.DB, msgCfg *platform.MessengerSettings, stdlog *log.Logger) error {
    tgBot, err := telegram.New(db, msgCfg, stdlog)
    // ...
    sessionMgr := chat.NewSessionManager(db)
    runner := chat.NewRunner()
    tgBot.SetChatComponents(sessionMgr, runner)
    d.tgBot.Store(tgBot)

    // Use context.Background() — lifecycle controlled by Stop(), not external context
    go tgBot.Start(context.Background())
    return nil
}

// Job completion notification (from cron goroutine)
if bot := d.tgBot.Load(); bot != nil {
    bot.NotifyJobComplete(ctx, jobName, result.Status, result.SummaryPath)
}

// Shutdown (on SIGINT/SIGTERM) — Stop() before cron shutdown
if bot := d.tgBot.Load(); bot != nil {
    bot.Stop()
}
```

The bot uses `context.Background()`, not the daemon's signal-scoped context. This ensures the bot stays alive during graceful shutdown until `Stop()` is explicitly called, rather than being torn down when the signal context is cancelled.

New adapters must:
- Be stored via `atomic.Pointer` for thread-safe access from cron goroutines
- Start in a background goroutine (non-blocking from daemon's perspective)
- Handle graceful shutdown when `Stop()` is called

---

## Database Dependencies

Adapters use these `storage.DB` methods:

### Session management

| Method | Purpose |
|---|---|
| `GetActiveSession(userID)` | Get current active session for a user |
| `DeactivateUserSessions(userID)` | Deactivate all sessions for a user |
| `UpdateSessionModel(sessionID, model)` | Change session model |
| `UpdateSessionEffort(sessionID, effort)` | Change session effort |

### Chat logging

| Method | Purpose |
|---|---|
| `AddChatLog(sessionID, role, content, costUSD, tokens)` | Log a message (`role`: "user" or "assistant") |

Session creation is handled by `chat.SessionManager`, not directly by the adapter.

---

## Package Structure

Follow this layout for new adapters:

```
internal/messenger/<platform>/
├── bot.go          # Adapter struct, New(), Start(), Stop(), Send(), IsAuthorized()
├── handlers.go     # Command handlers, callback handlers, NotifyJobComplete()
├── chat.go         # SetChatComponents(), handleChatMessage(), typing indicator
├── pairing.go      # PairingBot for setup wizard
└── format.go       # Markdown → platform-native format conversion (+tests)
```

---

## Checklist for New Adapters

- [ ] Implement `Messenger` interface (Type, Start, Stop, Send, IsAuthorized)
- [ ] Token validation at construction time
- [ ] Authorization middleware on all handlers
- [ ] Chat message flow with per-user locking
- [ ] Session management (get/create/clear via SessionManager)
- [ ] Cancel support (/stop with stored CancelFunc)
- [ ] Session error recovery (no conversation found, already in use)
- [ ] Typing/processing indicator during Claude execution
- [ ] All commands: /new, /stop, /jobs, /model, /effort, /status, /help, /start (alias for /help)
- [ ] Job listing with enable/disable/run actions
- [ ] Job completion notifications to all authorized users
- [ ] Markdown → platform-native format conversion
- [ ] Plain text fallback for message delivery
- [ ] Database logging (chat messages for both user and assistant)
- [ ] Terminal echo (stdlog) for observability
- [ ] Pairing bot for setup wizard integration
- [ ] Settings struct extensions (if needed)
- [ ] Daemon integration (atomic pointer, background goroutine, graceful shutdown)
- [ ] Add platform option to TUI setup wizard (`internal/tui/setup_wizard.go`)
- [ ] Add platform option to TUI settings menu (`internal/tui/settings_menu.go`)
- [ ] Update daemon startup to dispatch on `msgCfg.Type`

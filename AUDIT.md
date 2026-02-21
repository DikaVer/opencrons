# OpenCron — Cross-Reference Audit Report

## Critical Issues

### 1. Missing Panic Recovery in Cron Scheduler
**File:** `internal/daemon/daemon.go:62-65`
**Severity:** HIGH
**Issue:** The cron scheduler is created with `SkipIfStillRunning` but WITHOUT `cron.Recover()`. In robfig/cron v3, panic recovery is NOT automatic (unlike v1/v2). If any job panics (nil pointer, executor error), the goroutine dies silently with no logging.
**Fix:**
```go
cron: cron.New(
    cron.WithParser(ui.CronParser),
    cron.WithChain(
        cron.Recover(cron.DefaultLogger),
        cron.SkipIfStillRunning(cron.DefaultLogger),
    ),
),
```
**Library docs:** robfig/cron v3 removed automatic panic recovery. Chain wrappers compose as `m1(m2(m3(job)))`, so `Recover` wrapping `SkipIfStillRunning` catches panics from both.

### 2. Hot-Reload Race Window During Running Jobs
**File:** `internal/daemon/daemon.go:208-239`
**Severity:** MEDIUM
**Issue:** When `Reload()` removes and re-adds a job, if the old invocation is still running, the new `SkipIfStillRunning` wrapper gets a fresh semaphore. This means the new schedule entry won't know the old one is still running — briefly allowing overlapping execution of the same logical job during reload.
**Mitigation:** This is an inherent limitation of robfig/cron's decorator model. Document the behavior. For a full fix, maintain an external per-job mutex that persists across reloads.

### 3. `DefaultLogger` Writes to Stdout
**File:** `internal/daemon/daemon.go:64`
**Severity:** LOW
**Issue:** `cron.DefaultLogger` and `cron.Recover(cron.DefaultLogger)` write to stdout, which mixes with the daemon's own `[opencron]` prefixed logger. Skip notifications and panic stack traces go to stdout without the `[opencron]` prefix.
**Fix:** Provide a custom `cron.Logger` that wraps `d.logger` and `logger.Debug()`:
```go
type cronLogger struct {
    stdlog *log.Logger
}
func (l *cronLogger) Info(msg string, keysAndValues ...interface{}) {
    logger.Debug("cron: %s %v", msg, keysAndValues)
}
func (l *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
    l.stdlog.Printf("cron error: %s: %v %v", msg, err, keysAndValues)
    logger.Debug("cron error: %s: %v %v", msg, err, keysAndValues)
}
```

---

## Potential Improvements

### 4. Settings File Read on Every Debug Log Call
**File:** `internal/platform/settings.go:78-79` and `internal/logger/logger.go:47-51`
**Severity:** MEDIUM (performance)
**Issue:** `IsDebugEnabled()` calls `LoadSettings()` which reads and unmarshals `settings.json` from disk on every call. `logger.Debug()` and `logger.Info()` call this on every invocation. In hot paths (job execution, Telegram message handling), this means repeated file I/O for every log line.
**Fix:** Cache the debug flag with a sync.Once or atomic.Bool, invalidated only when settings are explicitly changed:
```go
var debugEnabled atomic.Bool
var debugLoaded sync.Once

func IsDebugEnabled() bool {
    debugLoaded.Do(func() {
        debugEnabled.Store(LoadSettings().Debug)
    })
    return debugEnabled.Load()
}

func SetDebug(enabled bool) error {
    s := LoadSettings()
    s.Debug = enabled
    err := SaveSettings(s)
    if err == nil {
        debugEnabled.Store(enabled)
    }
    return err
}
```

### 5. FindJobByName Loads All Jobs
**File:** `internal/config/loader.go:165-178`
**Severity:** LOW (performance)
**Issue:** `FindJobByName` calls `LoadAllJobs` (reads and parses every YAML file), then iterates to find the matching name. This is called frequently: in Telegram handlers (`toggleJob`, `runJob`, `showJobActions`), in TUI (`RunJobActionMenu`), in `editJob`, `enableJob`, `disableJob`.
**Fix:** Load the single file directly since job name == filename convention:
```go
func FindJobByName(schedulesDir, name string) (*JobConfig, error) {
    path := filepath.Join(schedulesDir, name+".yml")
    if _, err := os.Stat(path); err != nil {
        path = filepath.Join(schedulesDir, name+".yaml")
        if _, err := os.Stat(path); err != nil {
            return nil, fmt.Errorf("job %q not found", name)
        }
    }
    return LoadJob(path)
}
```

### 6. Telegram Message Truncation Uses Byte Length, Not Rune Length
**File:** `internal/messenger/telegram/chat.go:86-92`
**Severity:** LOW
**Issue:** `truncatedText[:100]` and `truncatedResponse[:200]` operate on byte indices, not rune boundaries. For messages containing multi-byte UTF-8 characters (emoji, CJK text), this can split in the middle of a character, producing invalid UTF-8 in terminal output.
**Fix:** Use the `ui.Truncate()` function that already exists in `internal/ui/format.go` and handles runes correctly:
```go
truncatedText := ui.Truncate(text, 100)
truncatedResponse := ui.Truncate(result.Response, 200)
```

### 7. Duplicate Model Validation Logic
**Files:** `internal/config/job.go:68-76` and TUI wizard (`internal/tui/wizard.go:109-119`, `275-285`)
**Severity:** LOW (maintainability)
**Issue:** Model validation is hardcoded in `JobConfig.Validate()` with a `map[string]bool` literal. The TUI wizard and Telegram handlers also hardcode model names in their option lists. When a new model is added, changes are needed in multiple places.
**Fix:** Define a shared `ValidModels` map or slice in the `ui` package and reference it everywhere:
```go
// ui/validate.go
var ValidModels = []string{"sonnet", "opus", "haiku"}
var ValidModelIDs = []string{"claude-sonnet-4-6", "claude-opus-4-6", "claude-haiku-4-5-20251001"}
```

### 8. No Log File Rotation or Cleanup
**Files:** `internal/executor/executor.go:85-86`, `internal/logger/logger.go:32-33`
**Severity:** LOW (operational)
**Issue:** stdout/stderr capture files accumulate indefinitely in the logs directory (one pair per execution). The debug log file also grows without bounds. Over weeks of running, this can consume significant disk space.
**Fix:** Add a log cleanup mechanism — either a `opencron cleanup` command or automatic pruning in the daemon (e.g., delete logs older than 30 days).

### 9. Chat Session Working Directory Defaults to Home
**File:** `internal/chat/session.go:53`
**Severity:** LOW
**Issue:** New chat sessions default `WorkingDir` to `os.UserHomeDir()`. Claude Code running in the user's home directory has access to all user files. There's no way for the user to set a preferred working directory for chat sessions.
**Fix:** Add a `WorkingDir` field to `ChatSettings` in `platform/settings.go`, configurable via the settings menu, and use it as the default for new sessions.

### 10. Error Swallowing in DB Log Updates
**File:** `internal/executor/executor.go:114, 165`
**Severity:** LOW
**Issue:** `db.UpdateLog()` errors are silently discarded with `_ =`. If the DB write fails (disk full, WAL checkpoint stuck), the execution result is lost.
**Fix:** Log the error at minimum:
```go
if err := db.UpdateLog(...); err != nil {
    logger.Debug("Failed to update execution log %d: %v", logID, err)
}
```

### 11. parseInt Silent Failure
**File:** `internal/tui/wizard.go:367-375`
**Severity:** LOW
**Issue:** `parseInt` uses `fmt.Sscanf` which silently returns 0 for non-numeric input like "abc". A user typing "five minutes" in the timeout field gets timeout=0, which means no timeout (infinite).
**Fix:** Use `strconv.Atoi` and return the default on error:
```go
func parseInt(s string, defaultVal int) int {
    s = strings.TrimSpace(s)
    if s == "" {
        return defaultVal
    }
    i, err := strconv.Atoi(s)
    if err != nil || i <= 0 {
        return defaultVal
    }
    return i
}
```

---

## Code Quality Notes

### 12. Consistent Error Wrapping
Most of the codebase uses `fmt.Errorf("context: %w", err)` correctly. A few places use `%s` instead of `%w`, which breaks `errors.Is`/`errors.As` chains. Check: `chat/runner.go:92` uses `%s` for stderr content (acceptable since it's not wrapping a Go error).

### 13. Good Practices Already in Place
- **fsnotify usage:** Watches directory (not files), filters Chmod, debounces 500ms, sync.Once for Stop — all best practices.
- **Prompt via stdin:** Avoids argument length limits and process list exposure.
- **Per-user Telegram lock:** Prevents concurrent message processing using sync.Map.
- **SQLite WAL mode + busy timeout:** Correct for concurrent access from daemon + Telegram bot.
- **Cron SkipIfStillRunning:** Prevents overlapping execution of the same job.
- **Graceful shutdown:** Waits for running jobs via `<-cron.Stop().Done()`.
- **Cryptographic random for pairing codes:** Uses `crypto/rand` in `internal/messenger/telegram/pairing.go`, not `math/rand`.
- **Build tags for platform code:** Clean separation of Unix/Windows implementations.

### 14. No Timezone Configuration
**File:** `internal/daemon/daemon.go:62`
**Issue:** The cron scheduler uses `time.Local` by default (no `cron.WithLocation`). This is fine for single-user local use, but DST transitions can cause jobs to skip or double-fire. Users have no way to explicitly set a timezone.
**Suggestion:** Consider adding an optional `Timezone` field to settings for users in regions with DST.

---

## Summary

| # | Issue | Severity | Category |
|---|-------|----------|----------|
| 1 | Missing panic recovery in cron | HIGH | Bug |
| 2 | Hot-reload race window | MEDIUM | Design limitation |
| 3 | DefaultLogger writes to stdout | LOW | Code quality |
| 4 | Settings read on every debug call | MEDIUM | Performance |
| 5 | FindJobByName loads all jobs | LOW | Performance |
| 6 | Byte-based truncation | LOW | Bug |
| 7 | Duplicate model validation | LOW | Maintainability |
| 8 | No log rotation | LOW | Operational |
| 9 | Chat working dir defaults to home | LOW | UX |
| 10 | Swallowed DB errors | LOW | Reliability |
| 11 | parseInt silent failure | LOW | Bug |
| 12 | Consistent error wrapping | LOW | Code quality |
| 14 | No timezone configuration | LOW | Feature gap |

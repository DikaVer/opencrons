// Package daemon implements the main OpenCron daemon that orchestrates cron
// job scheduling, file system watching for hot-reload, and the Telegram bot.
// The Daemon struct wraps robfig/cron with SkipIfStillRunning to prevent
// overlapping job execution, manages job registration and atomic hot-reload,
// maintains a PID file for single-instance enforcement, opens the SQLite
// database, and handles graceful shutdown on SIGINT/SIGTERM.
package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/DikaVer/opencrons/internal/chat"
	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/executor"
	"github.com/DikaVer/opencrons/internal/logger"
	"github.com/DikaVer/opencrons/internal/messenger/telegram"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/storage"
	"github.com/DikaVer/opencrons/internal/ui"
	"github.com/robfig/cron/v3"
)

// Daemon manages the cron job lifecycle.
type Daemon struct {
	cron    *cron.Cron
	db      *storage.DB
	watcher *Watcher
	jobs    map[string]cron.EntryID // job name -> cron entry ID
	mu      sync.Mutex
	logger  *log.Logger
	tgBot   atomic.Pointer[telegram.Bot] // written once at startup, read from cron goroutines
}

var slogger = logger.New("daemon")

// cronLogger routes cron library messages to OpenCron logging.
type cronLogger struct {
	stdlog *log.Logger
}

func (l *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	args := append([]any{"msg", msg}, keysAndValues...)
	slogger.Debug("cron info", args...)
}

func (l *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	if l.stdlog != nil {
		l.stdlog.Printf("Cron error: %s: %v (fields=%v)", msg, err, keysAndValues)
	}
	args := append([]any{"msg", msg, "err", err}, keysAndValues...)
	slogger.Warn("cron error", args...)
}

// Run starts the daemon in the foreground.
func Run() error {
	logger.Init(platform.LogsDir(), platform.IsDebugEnabled())

	stdlog := log.New(os.Stdout, "[opencrons] ", log.LstdFlags)
	cronLog := &cronLogger{stdlog: stdlog}

	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Write PID file
	if err := platform.WritePID(); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer func() { _ = platform.RemovePID() }()

	// Open database
	db, err := storage.Open(platform.DBPath())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	d := &Daemon{
		cron: cron.New(
			cron.WithParser(ui.CronParser),
			cron.WithChain(
				cron.Recover(cronLog),
				// SkipIfStillRunning treats the entire retry sequence (including
				// sleep delays between attempts) as a single "running" invocation.
				// If a job with retries outlasts its schedule interval, subsequent
				// scheduled firings are silently skipped until the sequence completes.
				// This is intentional — it prevents retry storms from concurrent firings.
				cron.SkipIfStillRunning(cronLog),
			),
		),
		db:     db,
		jobs:   make(map[string]cron.EntryID),
		logger: stdlog,
	}

	// Start Telegram bot if configured (before loading jobs so closures see d.tgBot)
	msgCfg := platform.GetMessengerConfig()
	if msgCfg != nil && msgCfg.Type == "telegram" {
		if err := d.startTelegramBot(db, msgCfg, stdlog); err != nil {
			stdlog.Printf("Warning: Telegram bot failed to start: %v", err)
			slogger.Warn("telegram bot start error", "err", err)
		}
	}

	// Load and register jobs
	if err := d.loadJobs(); err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	// Start file watcher for hot-reload
	watcher, err := NewWatcher(platform.SchedulesDir(), d)
	if err != nil {
		stdlog.Printf("Warning: file watcher failed to start: %v", err)
	} else {
		d.watcher = watcher
		go watcher.Start()
		defer watcher.Stop()
		slogger.Info("file watcher started", "dir", platform.SchedulesDir())
	}

	// Start cron
	d.cron.Start()
	d.logger.Printf("Daemon started (PID %d), %d job(s) loaded", os.Getpid(), len(d.jobs))
	slogger.Info("daemon started", "pid", os.Getpid(), "jobs", len(d.jobs))

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	d.logger.Printf("Received %s, shutting down...", sig)
	slogger.Info("shutdown signal received", "signal", sig)

	// Stop Telegram bot first
	if bot := d.tgBot.Load(); bot != nil {
		bot.Stop()
	}

	ctx := d.cron.Stop()
	<-ctx.Done()
	d.logger.Println("Daemon stopped.")
	slogger.Info("daemon stopped")

	return nil
}

// startTelegramBot initializes and starts the Telegram bot in a goroutine.
func (d *Daemon) startTelegramBot(db *storage.DB, msgCfg *platform.MessengerSettings, stdlog *log.Logger) error {
	tgBot, err := telegram.New(db, msgCfg, stdlog)
	if err != nil {
		return err
	}

	// Set up chat components
	sessionMgr := chat.NewSessionManager(db)
	runner := chat.NewRunner()
	tgBot.SetChatComponents(sessionMgr, runner)

	d.tgBot.Store(tgBot)

	// Start bot in background goroutine
	go tgBot.Start(context.Background())

	stdlog.Println("Telegram bot started")
	return nil
}

func (d *Daemon) loadJobs() error {
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if !job.Enabled {
			d.logger.Printf("Skipping disabled job %q", job.Name)
			slogger.Info("skipping disabled job", "name", job.Name)
			continue
		}

		if err := d.registerJob(job); err != nil {
			d.logger.Printf("Failed to register job %q: %v", job.Name, err)
			slogger.Error("failed to register job", "name", job.Name, "err", err)
		}
	}

	return nil
}

func (d *Daemon) registerJob(job *config.JobConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.registerJobLocked(job)
}

// registerJobLocked registers a job. Caller must hold d.mu.
func (d *Daemon) registerJobLocked(job *config.JobConfig) error {
	// Remove existing entry if re-registering
	if entryID, exists := d.jobs[job.Name]; exists {
		d.cron.Remove(entryID)
		delete(d.jobs, job.Name)
	}

	j := job

	entryID, err := d.cron.AddFunc(j.Schedule, func() {
		d.logger.Printf("Executing job %q", j.Name)
		slogger.Info("executing job", "name", j.Name)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		maxAttempts := 1 + j.MaxRetries
		var result *executor.Result
		var runErr error

		for attempt := 0; attempt < maxAttempts; attempt++ {
			if attempt > 0 {
				delay := retryDelay(j.RetryBackoff, attempt-1)
				d.logger.Printf("Job %q retrying in %s (attempt %d/%d)", j.Name, delay, attempt+1, maxAttempts)
				slogger.Info("job retry scheduled", "name", j.Name, "attempt", attempt+1, "delay", delay)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}

			result, runErr = executor.Run(ctx, d.db, j, "scheduled", attempt)
			if runErr != nil {
				// Setup error (working dir missing, etc.) — non-retryable
				d.logger.Printf("Job %q setup error: %v", j.Name, runErr)
				slogger.Error("job setup error", "name", j.Name, "err", runErr)
				if bot := d.tgBot.Load(); bot != nil {
					bot.NotifyJobComplete(ctx, j.Name, "failed", fmt.Sprintf("infrastructure error: %s", runErr.Error()))
				}
				return
			}

			if result.Status == "success" {
				break
			}

			if attempt < maxAttempts-1 {
				slogger.Warn("job failed, will retry", "name", j.Name,
					"attempt", attempt+1, "maxAttempts", maxAttempts, "status", result.Status)
			}
		}

		d.logger.Printf("Job %q finished: status=%s duration=%s", j.Name, result.Status, result.Duration)
		slogger.Info("job completed", "name", j.Name, "status", result.Status, "duration", result.Duration)

		// Notify via Telegram after all attempts — only on failure or timeout.
		if result.Status != "success" {
			if bot := d.tgBot.Load(); bot != nil {
				// For failed jobs prefer Claude's output; fall back to the exit error.
				// For timeout the status label is sufficient — no fallback needed.
				output := result.Output
				if output == "" && result.Status == "failed" {
					output = result.ErrorMsg
				}
				bot.NotifyJobComplete(ctx, j.Name, result.Status, output)
			}
		}

		// Trigger chained jobs on success
		if result.Status == "success" && len(j.OnSuccess) > 0 {
			d.runChainedJobs(j, map[string]bool{j.Name: true})
		}
	})
	if err != nil {
		return fmt.Errorf("adding cron entry: %w", err)
	}

	d.jobs[j.Name] = entryID
	d.logger.Printf("Registered job %q (%s)", j.Name, j.Schedule)
	slogger.Info("registered job", "name", j.Name, "schedule", j.Schedule)
	return nil
}

// runChainedJobs launches each job listed in parent.OnSuccess in its own goroutine.
// visited tracks job names already in the current chain to prevent infinite loops from cycles.
// Each chained job uses a fresh context so it is independent of the parent's lifecycle.
func (d *Daemon) runChainedJobs(parent *config.JobConfig, visited map[string]bool) {
	for _, childName := range parent.OnSuccess {
		if visited[childName] {
			d.logger.Printf("Chained job %q skipped: cycle detected in chain (visited: %v)", childName, visited)
			slogger.Warn("chain cycle detected, skipping", "child", childName, "parent", parent.Name)
			continue
		}

		childName := childName // capture loop variable

		// Copy visited set for this goroutine so sibling chains don't interfere.
		childVisited := make(map[string]bool, len(visited)+1)
		for k := range visited {
			childVisited[k] = true
		}
		childVisited[childName] = true

		go func() {
			child, err := config.FindJobByName(platform.SchedulesDir(), childName)
			if err != nil {
				d.logger.Printf("Chained job %q not found (triggered by %q): %v", childName, parent.Name, err)
				slogger.Warn("chained job not found", "child", childName, "parent", parent.Name, "err", err)
				return
			}
			if !child.Enabled {
				d.logger.Printf("Chained job %q is disabled, skipping (triggered by %q)", childName, parent.Name)
				slogger.Info("chained job disabled, skipping", "child", childName, "parent", parent.Name)
				return
			}

			d.logger.Printf("Running chained job %q (triggered by %q)", childName, parent.Name)
			slogger.Info("running chained job", "child", childName, "parent", parent.Name)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			result, err := executor.Run(ctx, d.db, child, "on_success", 0)
			if err != nil {
				d.logger.Printf("Chained job %q execution error: %v", childName, err)
				slogger.Error("chained job execution error", "child", childName, "err", err)
				return
			}
			d.logger.Printf("Chained job %q finished: status=%s duration=%s", childName, result.Status, result.Duration)
			slogger.Info("chained job completed", "child", childName, "status", result.Status, "duration", result.Duration)

			if bot := d.tgBot.Load(); bot != nil {
				bot.NotifyJobComplete(ctx, childName, result.Status, result.Output)
			}

			if result.Status == "success" && len(child.OnSuccess) > 0 {
				d.runChainedJobs(child, childVisited)
			}
		}()
	}
}

// Reload reloads all job configurations.
func (d *Daemon) Reload() {
	d.logger.Println("Reloading job configurations...")
	slogger.Info("hot-reload triggered")

	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove all existing entries
	for name, entryID := range d.jobs {
		d.cron.Remove(entryID)
		delete(d.jobs, name)
	}

	// Reload jobs while still holding the lock
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		d.logger.Printf("Error reloading jobs: %v", err)
		slogger.Error("reload error", "err", err)
		return
	}

	for _, job := range jobs {
		if !job.Enabled {
			d.logger.Printf("Skipping disabled job %q", job.Name)
			slogger.Info("skipping disabled job", "name", job.Name)
			continue
		}
		if err := d.registerJobLocked(job); err != nil {
			d.logger.Printf("Failed to register job %q: %v", job.Name, err)
			slogger.Error("failed to register job", "name", job.Name, "err", err)
		}
	}

	d.logger.Printf("Reload complete: %d job(s) loaded", len(d.jobs))
	slogger.Info("hot-reload complete", "jobs", len(d.jobs))
}

// retryDelay returns the wait duration before retry attempt retryIndex (0-based).
// Exponential: 30s, 60s, 120s, 240s … capped at 5 minutes.
// Linear: 30s, 60s, 90s …
// "" is the canonical sentinel for "exponential" (default).
func retryDelay(backoff string, retryIndex int) time.Duration {
	const base = 30 * time.Second
	const maxDelay = 5 * time.Minute
	if backoff == "linear" {
		return base * time.Duration(retryIndex+1)
	}
	// exponential (default): cap the shift index to avoid integer overflow.
	// 2^4 * 30s = 480s already exceeds maxDelay, so no need to compute further.
	if retryIndex > 4 {
		return maxDelay
	}
	delay := base * (1 << uint(retryIndex))
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

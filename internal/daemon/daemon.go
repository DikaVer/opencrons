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

	"github.com/DikaVer/opencron/internal/chat"
	"github.com/DikaVer/opencron/internal/config"
	"github.com/DikaVer/opencron/internal/executor"
	"github.com/DikaVer/opencron/internal/logger"
	"github.com/DikaVer/opencron/internal/messenger/telegram"
	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/storage"
	"github.com/DikaVer/opencron/internal/ui"
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

// cronLogger routes cron library messages to OpenCron logging.
type cronLogger struct {
	stdlog *log.Logger
}

func (l *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	logger.Debug("cron info: msg=%s fields=%v", msg, keysAndValues)
}

func (l *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	if l.stdlog != nil {
		l.stdlog.Printf("Cron error: %s: %v (fields=%v)", msg, err, keysAndValues)
	}
	logger.Debug("cron error: msg=%s err=%v fields=%v", msg, err, keysAndValues)
}

// Run starts the daemon in the foreground.
func Run() error {
	stdlog := log.New(os.Stdout, "[opencron] ", log.LstdFlags)
	cronLog := &cronLogger{stdlog: stdlog}

	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Write PID file
	if err := platform.WritePID(); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer platform.RemovePID()

	// Open database
	db, err := storage.Open(platform.DBPath())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	d := &Daemon{
		cron: cron.New(
			cron.WithParser(ui.CronParser),
			cron.WithChain(
				cron.Recover(cronLog),
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
			logger.Debug("Telegram bot start error: %v", err)
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
	}

	// Start cron
	d.cron.Start()
	d.logger.Printf("Daemon started (PID %d), %d job(s) loaded", os.Getpid(), len(d.jobs))
	logger.Info("Daemon started (PID %d), %d job(s) loaded", os.Getpid(), len(d.jobs))

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	d.logger.Printf("Received %s, shutting down...", sig)
	logger.Info("Received %s, shutting down...", sig)

	// Stop Telegram bot first
	if bot := d.tgBot.Load(); bot != nil {
		bot.Stop()
	}

	ctx := d.cron.Stop()
	<-ctx.Done()
	d.logger.Println("Daemon stopped.")
	logger.Info("Daemon stopped")

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
			continue
		}

		if err := d.registerJob(job); err != nil {
			d.logger.Printf("Failed to register job %q: %v", job.Name, err)
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
		logger.Debug("Daemon executing job %q", j.Name)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result, err := executor.Run(ctx, d.db, j, "scheduled")
		if err != nil {
			d.logger.Printf("Job %q execution error: %v", j.Name, err)
			logger.Debug("Daemon job %q error: %v", j.Name, err)
			return
		}
		d.logger.Printf("Job %q finished: status=%s duration=%s", j.Name, result.Status, result.Duration)

		// Notify via Telegram if bot is running
		if bot := d.tgBot.Load(); bot != nil {
			bot.NotifyJobComplete(ctx, j.Name, result.Status, result.SummaryPath)
		}
	})
	if err != nil {
		return fmt.Errorf("adding cron entry: %w", err)
	}

	d.jobs[j.Name] = entryID
	d.logger.Printf("Registered job %q (%s)", j.Name, j.Schedule)
	return nil
}

// Reload reloads all job configurations.
func (d *Daemon) Reload() {
	d.logger.Println("Reloading job configurations...")
	logger.Debug("Hot-reload triggered")

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
		return
	}

	for _, job := range jobs {
		if !job.Enabled {
			d.logger.Printf("Skipping disabled job %q", job.Name)
			continue
		}
		if err := d.registerJobLocked(job); err != nil {
			d.logger.Printf("Failed to register job %q: %v", job.Name, err)
		}
	}

	d.logger.Printf("Reload complete: %d job(s) loaded", len(d.jobs))
	logger.Debug("Hot-reload complete: %d job(s) loaded", len(d.jobs))
}

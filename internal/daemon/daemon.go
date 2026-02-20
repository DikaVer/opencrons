package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/executor"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/dika-maulidal/cli-scheduler/internal/storage"
	"github.com/robfig/cron/v3"
)

// Daemon manages the cron scheduler lifecycle.
type Daemon struct {
	cron    *cron.Cron
	db      *storage.DB
	watcher *Watcher
	jobs    map[string]cron.EntryID // job name -> cron entry ID
	mu      sync.Mutex
	logger  *log.Logger
}

// Run starts the daemon in the foreground.
func Run() error {
	logger := log.New(os.Stdout, "[scheduler] ", log.LstdFlags)

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
			cron.WithParser(cron.NewParser(cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow)),
			cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
		),
		db:     db,
		jobs:   make(map[string]cron.EntryID),
		logger: logger,
	}

	// Load and register jobs
	if err := d.loadJobs(); err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	// Start file watcher for hot-reload
	watcher, err := NewWatcher(platform.SchedulesDir(), d)
	if err != nil {
		logger.Printf("Warning: file watcher failed to start: %v", err)
	} else {
		d.watcher = watcher
		go watcher.Start()
		defer watcher.Stop()
	}

	// Start cron
	d.cron.Start()
	logger.Printf("Daemon started (PID %d), %d job(s) loaded", os.Getpid(), len(d.jobs))

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Printf("Received %s, shutting down...", sig)
	ctx := d.cron.Stop()
	<-ctx.Done()
	logger.Println("Daemon stopped.")

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

	// Capture job for closure
	j := job

	entryID, err := d.cron.AddFunc(j.Schedule, func() {
		d.logger.Printf("Executing job %q", j.Name)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result, err := executor.Run(ctx, d.db, j, "scheduled")
		if err != nil {
			d.logger.Printf("Job %q execution error: %v", j.Name, err)
			return
		}
		d.logger.Printf("Job %q finished: status=%s duration=%s", j.Name, result.Status, result.Duration)
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
}

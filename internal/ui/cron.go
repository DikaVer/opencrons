// Package ui provides shared formatting, styling, and validation utilities for
// OpenCron's terminal interface.
//
// cron.go exposes CronParser, a shared robfig/cron 5-field parser
// (Minute|Hour|Dom|Month|Dow) used across the application for schedule validation
// and next-run calculation.
package ui

import "github.com/robfig/cron/v3"

// CronParser is the shared 5-field cron parser used across the application.
var CronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

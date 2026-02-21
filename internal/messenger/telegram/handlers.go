// handlers.go implements Telegram bot command and callback query handlers.
//
// Commands: /new (clear session), /stop (cancel running query), /jobs
// (inline keyboard job list), /model (model picker), /effort (effort picker),
// /status (daemon and session info), /help. Callback handlers process job
// actions (select, enable, disable, run, back) and model/effort selection
// from inline keyboards. NotifyJobComplete broadcasts job results to all
// authorized users.
package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/executor"
	"github.com/DikaVer/opencrons/internal/logger"
	"github.com/DikaVer/opencrons/internal/platform"
)

func (b *Bot) handleStopQuery(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	cancelVal, ok := b.cancels.Load(userID)
	if !ok {
		b.SendPlain(ctx, chatID, "No active query to stop.")
		return
	}

	cancelFunc := cancelVal.(context.CancelFunc)
	cancelFunc()
	b.stdlog.Printf("[telegram] User %d stopped active query", userID)
}

func (b *Bot) handleNew(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Deactivate existing sessions via session manager
	if b.sessionMgr != nil {
		if err := b.sessionMgr.ClearSession(userID); err != nil {
			logger.Debug("Error clearing session for user %d: %v", userID, err)
		}
	} else if err := b.db.DeactivateUserSessions(userID); err != nil {
		logger.Debug("Error deactivating sessions for user %d: %v", userID, err)
	}

	b.SendPlain(ctx, chatID, "Session cleared. Send a message to start a fresh conversation with Claude.")
	b.stdlog.Printf("[telegram] User %d started new session", userID)
}

func (b *Bot) handleJobs(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error loading jobs: %v", err))
		return
	}

	if len(jobs) == 0 {
		b.SendPlain(ctx, chatID, "No jobs configured. Use the TUI to add jobs.")
		return
	}

	// Build inline keyboard with jobs
	var rows [][]models.InlineKeyboardButton
	for _, j := range jobs {
		statusIcon := "+"
		if !j.Enabled {
			statusIcon = "-"
		}
		label := fmt.Sprintf("[%s] %s (%s)", statusIcon, j.Name, j.Schedule)
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: label, CallbackData: "job:select:" + j.Name},
		})
	}

	kb := &models.InlineKeyboardMarkup{InlineKeyboard: rows}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Scheduled Jobs:\n(+ enabled, - disabled)",
		ReplyMarkup: kb,
	})
}

func (b *Bot) handleModel(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	// Check current session model
	session, _ := b.db.GetActiveSession(update.Message.From.ID)
	currentModel := "sonnet"
	if session != nil {
		currentModel = session.Model
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: modelLabel("sonnet", currentModel), CallbackData: "model:sonnet"},
				{Text: modelLabel("opus", currentModel), CallbackData: "model:opus"},
				{Text: modelLabel("haiku", currentModel), CallbackData: "model:haiku"},
			},
		},
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Current model: *%s*\nSelect a model:", currentModel),
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (b *Bot) handleEffort(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	session, _ := b.db.GetActiveSession(update.Message.From.ID)
	currentEffort := "high"
	if session != nil {
		currentEffort = session.Effort
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: effortLabel("low", currentEffort), CallbackData: "effort:low"},
				{Text: effortLabel("medium", currentEffort), CallbackData: "effort:medium"},
			},
			{
				{Text: effortLabel("high", currentEffort), CallbackData: "effort:high"},
				{Text: effortLabel("max", currentEffort), CallbackData: "effort:max"},
			},
		},
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Current effort: *%s*\nSelect effort level:", currentEffort),
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (b *Bot) handleStatus(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	// Daemon status
	pid, running := platform.CheckDaemonRunning()
	var statusLines []string
	if running {
		statusLines = append(statusLines, fmt.Sprintf("Daemon: running (PID %d)", pid))
	} else {
		statusLines = append(statusLines, "Daemon: stopped")
	}

	// Job count
	jobs, _ := config.LoadAllJobs(platform.SchedulesDir())
	enabled := 0
	for _, j := range jobs {
		if j.Enabled {
			enabled++
		}
	}
	statusLines = append(statusLines, fmt.Sprintf("Jobs: %d total, %d enabled", len(jobs), enabled))

	// Active session
	session, _ := b.db.GetActiveSession(update.Message.From.ID)
	if session != nil {
		statusLines = append(statusLines, fmt.Sprintf("Session: active (model=%s, effort=%s)", session.Model, session.Effort))
		statusLines = append(statusLines, fmt.Sprintf("Working dir: %s", session.WorkingDir))
	} else {
		statusLines = append(statusLines, "Session: none (send a message to start)")
	}

	b.SendPlain(ctx, chatID, strings.Join(statusLines, "\n"))
}

func (b *Bot) handleHelp(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	help := `OpenCron Bot

Commands:
/new - Start a fresh chat session
/stop - Stop the current running query
/jobs - List and manage scheduled jobs
/model - Change the Claude model
/effort - Change the effort level
/status - Show daemon and session status
/help - Show this help message

Send any text message to chat with Claude.`

	b.SendPlain(ctx, chatID, help)
}

// handleJobCallback processes inline keyboard callbacks for job management.
func (b *Bot) handleJobCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery
	data := cb.Data

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	chatID := cb.Message.Message.Chat.ID

	// Parse callback data: job:<action>:<name>
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return
	}
	action := parts[1]
	jobName := parts[2]

	switch action {
	case "select":
		b.showJobActions(ctx, tgBot, chatID, jobName)
	case "enable":
		b.toggleJob(ctx, chatID, jobName, true)
	case "disable":
		b.toggleJob(ctx, chatID, jobName, false)
	case "run":
		b.runJob(ctx, chatID, jobName)
	case "back":
		b.handleJobs(ctx, tgBot, &models.Update{
			Message: &models.Message{
				Chat: models.Chat{ID: chatID},
				From: &cb.From,
			},
		})
	}
}

// showJobActions displays action buttons for a specific job.
func (b *Bot) showJobActions(ctx context.Context, tgBot *bot.Bot, chatID int64, jobName string) {
	job, err := config.FindJobByName(platform.SchedulesDir(), jobName)
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Job not found: %v", err))
		return
	}

	status := "Enabled"
	if !job.Enabled {
		status = "Disabled"
	}

	text := fmt.Sprintf("Job: %s\nSchedule: %s\nModel: %s\nStatus: %s", job.Name, job.Schedule, job.Model, status)

	var toggleBtn models.InlineKeyboardButton
	if job.Enabled {
		toggleBtn = models.InlineKeyboardButton{Text: "Disable", CallbackData: "job:disable:" + jobName}
	} else {
		toggleBtn = models.InlineKeyboardButton{Text: "Enable", CallbackData: "job:enable:" + jobName}
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				toggleBtn,
				{Text: "Run Now", CallbackData: "job:run:" + jobName},
			},
			{
				{Text: "<< Back to jobs", CallbackData: "job:back:"},
			},
		},
	}

	tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: kb,
	})
}

// toggleJob enables or disables a job.
func (b *Bot) toggleJob(ctx context.Context, chatID int64, jobName string, enable bool) {
	job, err := config.FindJobByName(platform.SchedulesDir(), jobName)
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	job.Enabled = enable
	if err := config.SaveJob(platform.SchedulesDir(), job); err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error saving job: %v", err))
		return
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}
	b.SendPlain(ctx, chatID, fmt.Sprintf("Job %q %s.", jobName, action))
	b.stdlog.Printf("[telegram] Job %q %s via Telegram", jobName, action)
}

// runJob executes a job immediately.
func (b *Bot) runJob(ctx context.Context, chatID int64, jobName string) {
	job, err := config.FindJobByName(platform.SchedulesDir(), jobName)
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	if err := job.ValidatePromptFileExists(platform.PromptsDir()); err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	b.SendPlain(ctx, chatID, fmt.Sprintf("Running job %q...", jobName))

	// Use a background context for execution and result delivery — the
	// callback handler's ctx may expire during long-running jobs.
	execCtx := context.Background()

	result, err := executor.Run(execCtx, b.db, job, "manual")
	if err != nil {
		b.SendPlain(execCtx, chatID, fmt.Sprintf("Execution failed: %v", err))
		return
	}

	logger.Debug("runJob: %q finished status=%s output=%d bytes", jobName, result.Status, len(result.Output))

	// Send job output if available, otherwise fall back to generic status
	if output := strings.TrimSpace(result.Output); output != "" {
		msg := output
		if result.Status != "success" {
			msg = fmt.Sprintf("Job %q (%s):\n\n%s", jobName, result.Status, msg)
		}
		msg = truncateForTelegram(msg, jobName)
		if sendErr := b.Send(execCtx, chatID, msg); sendErr != nil {
			logger.Debug("runJob: HTML send failed for %q: %v", jobName, sendErr)
			if plainErr := b.SendPlain(execCtx, chatID, msg); plainErr != nil {
				logger.Debug("runJob: plain send also failed for %q: %v", jobName, plainErr)
				b.SendPlain(execCtx, chatID, fmt.Sprintf("Job %q completed (%s) but failed to deliver output. Check logs: opencrons logs %s", jobName, result.Status, jobName))
			}
		}
		b.stdlog.Printf("[telegram] Job %q executed: status=%s (output sent)", jobName, result.Status)
		return
	}

	msg := fmt.Sprintf("Job %q finished\nStatus: %s\nDuration: %s", jobName, result.Status, result.Duration.Round(1e9))
	if result.CostUSD > 0 {
		msg += fmt.Sprintf("\nCost: $%.4f", result.CostUSD)
	}
	if result.ErrorMsg != "" {
		msg += fmt.Sprintf("\nError: %s", result.ErrorMsg)
	}

	b.SendPlain(execCtx, chatID, msg)
	b.stdlog.Printf("[telegram] Job %q executed: status=%s", jobName, result.Status)
}

// handleModelCallback processes model selection inline keyboard callbacks.
func (b *Bot) handleModelCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	// Parse callback data: model:<model>
	parts := strings.SplitN(cb.Data, ":", 2)
	if len(parts) < 2 {
		return
	}
	model := parts[1]

	chatID := cb.Message.Message.Chat.ID
	userID := cb.From.ID

	// Update active session
	session, _ := b.db.GetActiveSession(userID)
	if session != nil {
		if err := b.db.UpdateSessionModel(session.ID, model); err != nil {
			logger.Debug("Error updating session model: %v", err)
		}
	}

	b.SendPlain(ctx, chatID, fmt.Sprintf("Model changed to: %s", model))
	b.stdlog.Printf("[telegram] User %d changed model to %s", userID, model)
}

// handleEffortCallback processes effort selection inline keyboard callbacks.
func (b *Bot) handleEffortCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery

	tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	parts := strings.SplitN(cb.Data, ":", 2)
	if len(parts) < 2 {
		return
	}
	effort := parts[1]

	chatID := cb.Message.Message.Chat.ID
	userID := cb.From.ID

	session, _ := b.db.GetActiveSession(userID)
	if session != nil {
		if err := b.db.UpdateSessionEffort(session.ID, effort); err != nil {
			logger.Debug("Error updating session effort: %v", err)
		}
	}

	b.SendPlain(ctx, chatID, fmt.Sprintf("Effort changed to: %s", effort))
	b.stdlog.Printf("[telegram] User %d changed effort to %s", userID, effort)
}

// NotifyJobComplete sends a job completion notification to all authorized chats.
// If output is non-empty, the job's output text is sent directly.
func (b *Bot) NotifyJobComplete(ctx context.Context, jobName, status, output string) {
	// Use job output as the notification message; fall back to generic status
	msg := strings.TrimSpace(output)
	if msg == "" {
		msg = fmt.Sprintf("Job '%s' completed: %s", jobName, status)
		logger.Debug("NotifyJobComplete: no output for %q, sending status-only message", jobName)
	} else {
		// Prepend status header for non-success jobs so the user knows
		if status != "success" {
			msg = fmt.Sprintf("Job '%s' (%s):\n\n%s", jobName, status, msg)
		}
		msg = truncateForTelegram(msg, jobName)
		logger.Debug("NotifyJobComplete: sending output for %q (%d bytes)", jobName, len(msg))
	}

	// Notify all authorized users
	sent := 0
	for userStr, allowed := range b.settings.AllowedUsers {
		if !allowed || strings.HasPrefix(userStr, "__") {
			continue
		}
		var userID int64
		fmt.Sscanf(userStr, "%d", &userID)
		if userID > 0 {
			if err := b.Send(ctx, userID, msg); err != nil {
				logger.Debug("NotifyJobComplete: HTML send failed for user %d: %v, falling back to plain", userID, err)
				if plainErr := b.SendPlain(ctx, userID, msg); plainErr != nil {
					logger.Debug("NotifyJobComplete: plain send also failed for user %d: %v", userID, plainErr)
					// Notify user that output was generated but delivery failed
					failMsg := fmt.Sprintf("Job '%s' completed (%s) but failed to deliver output. Check logs: opencrons logs %s", jobName, status, jobName)
					if assertErr := b.SendPlain(ctx, userID, failMsg); assertErr != nil {
						b.stdlog.Printf("[telegram] NotifyJobComplete: all delivery attempts failed for user %d, job %q: %v", userID, jobName, assertErr)
					}
				} else {
					sent++
				}
			} else {
				sent++
			}
		}
	}
	b.stdlog.Printf("[telegram] Job %q notification sent to %d user(s)", jobName, sent)
}

// truncateForTelegram ensures a message fits within Telegram's 4096-character
// limit, appending a hint to check logs if the output is too long.
func truncateForTelegram(msg, jobName string) string {
	const maxLen = 4000 // leave headroom for HTML formatting overhead
	if len(msg) <= maxLen {
		return msg
	}
	suffix := fmt.Sprintf("\n\n[Output truncated — full output: opencrons logs %s]", jobName)
	return msg[:maxLen-len(suffix)] + suffix
}

func modelLabel(model, current string) string {
	if model == current {
		return model + " *"
	}
	return model
}

func effortLabel(effort, current string) string {
	if effort == current {
		return effort + " *"
	}
	return effort
}

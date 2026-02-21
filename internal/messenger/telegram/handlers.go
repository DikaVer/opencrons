package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/executor"
	"github.com/dika-maulidal/cli-scheduler/internal/logger"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
)

func (b *Bot) handleNew(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Deactivate existing sessions
	if err := b.db.DeactivateUserSessions(userID); err != nil {
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

	help := `CLI Scheduler Bot

Commands:
/new - Start a fresh chat session
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

	result, err := executor.Run(ctx, b.db, job, "manual")
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Execution failed: %v", err))
		return
	}

	msg := fmt.Sprintf("Job %q finished\nStatus: %s\nDuration: %s", jobName, result.Status, result.Duration.Round(1e9))
	if result.CostUSD > 0 {
		msg += fmt.Sprintf("\nCost: $%.4f", result.CostUSD)
	}
	if result.ErrorMsg != "" {
		msg += fmt.Sprintf("\nError: %s", result.ErrorMsg)
	}

	b.SendPlain(ctx, chatID, msg)
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
func (b *Bot) NotifyJobComplete(ctx context.Context, jobName, status string) {
	msg := fmt.Sprintf("Job '%s' completed: %s", jobName, status)

	// Notify all active sessions' chat IDs
	for userStr, allowed := range b.settings.AllowedUsers {
		if !allowed || strings.HasPrefix(userStr, "__") {
			continue
		}
		// We stored user IDs, but we need chat IDs. For private chats, chatID == userID.
		var userID int64
		fmt.Sscanf(userStr, "%d", &userID)
		if userID > 0 {
			b.SendPlain(ctx, userID, msg)
		}
	}
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

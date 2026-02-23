// handlers_callbacks.go implements Telegram inline keyboard callback handlers.
// handleJobCallback dispatches job action callbacks (select, enable, disable, run, back).
// handleModelCallback and handleEffortCallback process model/effort selection callbacks.
// modelLabel and effortLabel are UI helpers that mark the currently active choice.
package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/executor"
	"github.com/DikaVer/opencrons/internal/platform"
)

// handleJobCallback processes inline keyboard callbacks for job management.
func (b *Bot) handleJobCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery
	data := cb.Data

	_, _ = tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Job not found: %v", err))
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

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: kb,
	})
}

// toggleJob enables or disables a job.
func (b *Bot) toggleJob(ctx context.Context, chatID int64, jobName string, enable bool) {
	job, err := config.FindJobByName(platform.SchedulesDir(), jobName)
	if err != nil {
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	job.Enabled = enable
	if err := config.SaveJob(platform.SchedulesDir(), job); err != nil {
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Error saving job: %v", err))
		return
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}
	_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Job %q %s.", jobName, action))
	b.stdlog.Printf("[telegram] Job %q %s via Telegram", jobName, action)
	slogger.Info("job toggled via telegram", "job", jobName, "action", action)
}

// runJob executes a job immediately.
func (b *Bot) runJob(ctx context.Context, chatID int64, jobName string) {
	job, err := config.FindJobByName(platform.SchedulesDir(), jobName)
	if err != nil {
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	if err := job.ValidatePromptFileExists(platform.PromptsDir()); err != nil {
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	slogger.Info("job run requested via telegram", "job", jobName)
	_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Running job %q...", jobName))

	// Use a background context for execution and result delivery — the
	// callback handler's ctx may expire during long-running jobs.
	execCtx := context.Background()

	result, err := executor.Run(execCtx, b.db, job, executor.TriggerManual, 0)
	if err != nil {
		_ = b.SendPlain(execCtx, chatID, fmt.Sprintf("Execution failed: %v", err))
		return
	}

	b.sendJobOutput(execCtx, chatID, jobName, result.Status, result.Output)
	b.stdlog.Printf("[telegram] Job %q executed: status=%s", jobName, result.Status)
}

// handleModelCallback processes model selection inline keyboard callbacks.
func (b *Bot) handleModelCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery

	_, _ = tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
			slogger.Warn("error updating session model", "err", err)
		}
	}

	_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Model changed to: %s", model))
	b.stdlog.Printf("[telegram] User %d changed model to %s", userID, model)
}

// handleEffortCallback processes effort selection inline keyboard callbacks.
func (b *Bot) handleEffortCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery

	_, _ = tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
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
			slogger.Warn("error updating session effort", "err", err)
		}
	}

	_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Effort changed to: %s", effort))
	b.stdlog.Printf("[telegram] User %d changed effort to %s", userID, effort)
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

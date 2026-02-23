// handlers.go contains the job completion notification logic shared across
// scheduled, manual, and chained job executions.
//
// NotifyJobComplete broadcasts a job result to all authorized Telegram users.
// sendJobOutput delivers the result to a single chat with truncation and
// HTML-to-plain fallback. truncateForTelegram enforces the 4096-character limit.
package telegram

import (
	"context"
	"fmt"
	"strings"
)

// NotifyJobComplete sends a job completion notification to all authorized chats.
// If output is non-empty, the job's output text is sent directly.
func (b *Bot) NotifyJobComplete(ctx context.Context, jobName, status, output string) {
	sent := 0
	for userStr, allowed := range b.settings.AllowedUsers {
		if !allowed || strings.HasPrefix(userStr, "__") {
			continue
		}
		var userID int64
		_, _ = fmt.Sscanf(userStr, "%d", &userID)
		if userID > 0 {
			if b.sendJobOutput(ctx, userID, jobName, status, output) {
				sent++
			}
		}
	}
	b.stdlog.Printf("[telegram] Job %q notification sent to %d user(s)", jobName, sent)
}

// sendJobOutput delivers a job's result to a single chat. Used by both
// NotifyJobComplete (scheduled/CLI) and runJob (Telegram Run Now).
// Returns true if the message was delivered successfully.
func (b *Bot) sendJobOutput(ctx context.Context, chatID int64, jobName, status, output string) bool {
	msg := strings.TrimSpace(output)
	if msg == "" {
		msg = fmt.Sprintf("Job '%s' completed: %s", jobName, status)
		slogger.Debug("sendJobOutput: no output, sending status-only", "job", jobName)
	} else {
		if status != "success" {
			msg = fmt.Sprintf("Job '%s' (%s):\n\n%s", jobName, status, msg)
		}
		msg = truncateForTelegram(msg, jobName)
		slogger.Debug("sendJobOutput: sending output", "job", jobName, "chatID", chatID, "bytes", len(msg))
	}

	if err := b.Send(ctx, chatID, msg); err != nil {
		slogger.Warn("sendJobOutput: HTML send failed", "job", jobName, "chatID", chatID, "err", err)
		if plainErr := b.SendPlain(ctx, chatID, msg); plainErr != nil {
			slogger.Error("sendJobOutput: plain send also failed", "job", jobName, "chatID", chatID, "err", plainErr)
			failMsg := fmt.Sprintf("Job '%s' completed (%s) but failed to deliver output. Check logs: opencrons logs %s", jobName, status, jobName)
			if assertErr := b.SendPlain(ctx, chatID, failMsg); assertErr != nil {
				b.stdlog.Printf("[telegram] sendJobOutput: all delivery attempts failed for chat %d, job %q: %v", chatID, jobName, assertErr)
			}
			return false
		}
	}
	return true
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

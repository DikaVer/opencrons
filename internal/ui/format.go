// format.go provides output formatting helpers.
//
// FormatTokens formats integer token counts with K/M suffixes for compact display.
// Truncate shortens strings beyond a maximum length by appending an ellipsis.
package ui

import "fmt"

// FormatTokens formats a token count with K/M suffixes.
func FormatTokens(n int) string {
	if n == 0 {
		return "0"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// Truncate shortens a string to maxLen runes, adding "…" if truncated.
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

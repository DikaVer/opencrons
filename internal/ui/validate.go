// validate.go provides input validation functions for TUI forms.
//
// ValidateJobName ensures names contain only alphanumeric characters, hyphens, and
// underscores. ValidateDirectory checks that a path exists and is a directory.
// ValidateCron verifies a valid 5-field cron expression. ValidateNonEmpty rejects
// blank input.
package ui

import (
	"fmt"
	"os"
	"strings"
)

// ValidateJobName checks that a job name is valid.
func ValidateJobName(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("name is required")
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' {
			return fmt.Errorf("invalid character %q (use alphanumeric, hyphens, underscores)", string(c))
		}
	}
	return nil
}

// ValidateDirectory checks that a path is an existing directory.
func ValidateDirectory(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("directory is required")
	}
	info, err := os.Stat(s)
	if err != nil {
		return fmt.Errorf("directory does not exist")
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	return nil
}

// ValidateCron checks that a cron expression is valid.
func ValidateCron(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("schedule is required")
	}
	if _, err := CronParser.Parse(s); err != nil {
		return fmt.Errorf("invalid cron expression: %v", err)
	}
	return nil
}

// ValidateNonEmpty checks that a string is not empty.
func ValidateNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("this field is required")
	}
	return nil
}

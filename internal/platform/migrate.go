package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// MigrateFromV1Layout migrates the old workspace/ directory layout to the new
// canonical .agents/ + AGENTS.md layout at the BaseDir root.
//
// V1 layout: BaseDir/workspace/.claude/ + BaseDir/workspace/CLAUDE.md
// V2 layout: BaseDir/.agents/ + BaseDir/AGENTS.md (with provider symlinks)
//
// This is a no-op if the old workspace/ doesn't exist or .agents/ already exists.
func MigrateFromV1Layout() error {
	base := BaseDir()
	oldWorkspace := filepath.Join(base, "workspace")
	newAgentsDir := filepath.Join(base, ".agents")

	// Only migrate if old layout exists and new one doesn't
	if !isFileOrDir(oldWorkspace) || isFileOrDir(newAgentsDir) {
		return nil
	}

	// Move workspace/.claude/ → .agents/
	oldClaudeDir := filepath.Join(oldWorkspace, ".claude")
	if isFileOrDir(oldClaudeDir) {
		if err := os.Rename(oldClaudeDir, newAgentsDir); err != nil {
			return err
		}
	}

	// Move workspace/CLAUDE.md → AGENTS.md
	oldClaudeFile := filepath.Join(oldWorkspace, "CLAUDE.md")
	newAgentsFile := filepath.Join(base, "AGENTS.md")
	if isFileOrDir(oldClaudeFile) && !isFileOrDir(newAgentsFile) {
		if err := os.Rename(oldClaudeFile, newAgentsFile); err != nil {
			return err
		}
	}

	// Remove the old workspace/ directory if empty.
	// If it has leftover files, warn on stderr but don't fail.
	if err := os.Remove(oldWorkspace); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Note: old workspace/ directory has extra files and was not removed: %s\n", oldWorkspace)
	}

	return nil
}

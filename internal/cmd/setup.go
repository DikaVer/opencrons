// File setup.go implements the setup command and the runSetupWizard helper.
// It runs the TUI setup wizard, copies .workspace/ files (AGENTS.md to CLAUDE.md,
// .agents/ to .claude/) into the config directory, and saves the resulting settings.
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/tui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Run the setup wizard",
	Long:  "Run (or re-run) the interactive setup wizard to configure providers, messengers, and daemon settings.",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	return runSetupWizard()
}

func runSetupWizard() error {
	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	result, err := tui.RunSetupWizard()
	if err != nil {
		return err
	}

	// Copy workspace files
	if err := copyWorkspace(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not copy workspace files: %v\n", err)
	}

	// Save settings
	s := platform.LoadSettings()
	s.SetupComplete = true
	s.Provider = &platform.ProviderSettings{ID: result.Provider}
	s.DaemonMode = result.DaemonMode

	if result.Messenger != nil {
		s.Messenger = result.Messenger
	}
	if result.Chat != nil {
		s.Chat = result.Chat
	}

	if err := platform.SaveSettings(s); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}

	return nil
}

// copyWorkspace copies .workspace/ from the executable's directory to the config workspace dir.
// In the destination: AGENTS.md -> CLAUDE.md, .agents/ -> .claude/
func copyWorkspace() error {
	// Find the source .workspace directory relative to the executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	// Try a few locations for the .workspace directory
	var srcDir string
	candidates := []string{
		filepath.Join(exeDir, ".workspace"),
		filepath.Join(exeDir, "..", ".workspace"),
		filepath.Join(".", ".workspace"),
	}

	// Also check cwd
	cwd, _ := os.Getwd()
	if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, ".workspace"))
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			srcDir = c
			break
		}
	}

	if srcDir == "" {
		return fmt.Errorf(".workspace directory not found")
	}

	destDir := platform.WorkspaceDir()

	// Walk source and copy, performing renames
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Rename: AGENTS.md -> CLAUDE.md
		relPath = renameWorkspacePath(relPath)

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

// renameWorkspacePath applies workspace-to-config path renames.
func renameWorkspacePath(relPath string) string {
	// AGENTS.md -> CLAUDE.md
	if relPath == "AGENTS.md" {
		return "CLAUDE.md"
	}

	// .agents/ -> .claude/
	if relPath == ".agents" {
		return ".claude"
	}
	if len(relPath) > 8 && relPath[:8] == ".agents"+string(filepath.Separator) {
		return ".claude" + relPath[7:]
	}
	// Also handle forward slashes
	if len(relPath) > 8 && relPath[:8] == ".agents/" {
		return ".claude" + relPath[7:]
	}

	return relPath
}

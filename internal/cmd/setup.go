// File setup.go implements the setup command and the runSetupWizard helper.
// It runs the TUI setup wizard, copies .workspace-example/ files (AGENTS.md, .agents/)
// into the config BaseDir, and saves the resulting settings. Provider-specific symlinks
// (e.g., .claude/ → .agents/) are created by EnsureDirs.
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/tui"
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

	cmdlog.Info("setup wizard completed", "provider", result.Provider, "daemonMode", result.DaemonMode)

	// Re-create provider symlinks now that settings (with provider ID) are saved.
	// EnsureDirs ran before settings were written, so symlinks couldn't be created then.
	if err := platform.EnsureProviderSymlinks(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: could not create provider symlinks: %v\n", err)
	}

	return nil
}

// copyWorkspace copies .workspace-example/ from the executable's directory to the
// config BaseDir. Files are copied with their canonical names (AGENTS.md, .agents/).
// Provider-specific symlinks are handled separately by EnsureDirs/EnsureProviderSymlinks.
func copyWorkspace() error {
	// Find the source .workspace-example directory relative to the executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	// Try a few locations for the .workspace-example directory
	var srcDir string
	candidates := []string{
		filepath.Join(exeDir, ".workspace-example"),
		filepath.Join(exeDir, "..", ".workspace-example"),
		filepath.Join(".", ".workspace-example"),
	}

	// Also check cwd
	cwd, _ := os.Getwd()
	if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, ".workspace-example"))
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			srcDir = c
			break
		}
	}

	if srcDir == "" {
		return fmt.Errorf(".workspace-example directory not found")
	}

	destDir := platform.BaseDir()

	// Walk source and copy using canonical names (no renames needed).
	// Skip files that already exist to preserve user customizations on re-runs.
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == srcDir {
			return nil // skip root entry
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Don't overwrite existing files (user may have customized them)
		if _, statErr := os.Stat(destPath); statErr == nil {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

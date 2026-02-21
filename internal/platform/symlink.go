package platform

import (
	"os"
	"path/filepath"
)

// EnsureSymlink creates a symlink at linkPath pointing to target, idempotently.
// If the link already exists and points to the correct target, it's a no-op.
// If it exists but points elsewhere (or is a regular file/dir), it's removed and recreated.
// Also handles hardlinks (Windows fallback) by checking inode identity.
func EnsureSymlink(target, linkPath string) error {
	// Check if it's a symlink pointing to the correct target
	existing, err := os.Readlink(linkPath)
	if err == nil {
		if existing == target {
			return nil // symlink already correct
		}
		// Symlink exists but points elsewhere — remove and recreate
		if removeErr := os.Remove(linkPath); removeErr != nil {
			return removeErr
		}
		return createSymlink(target, linkPath)
	}

	// os.Readlink failed — not a symlink, or doesn't exist
	if isFileOrDir(linkPath) {
		// Something exists (regular file, dir, hardlink, or junction).
		// Check if it's a hardlink to the same file (Windows fallback creates these).
		linfo, lerr := os.Lstat(linkPath)
		tinfo, terr := os.Stat(target)
		if lerr == nil && terr == nil && os.SameFile(linfo, tinfo) {
			return nil // hardlink already points to the same inode
		}
		// Remove the existing entry
		if removeErr := os.Remove(linkPath); removeErr != nil {
			return removeErr
		}
	}

	return createSymlink(target, linkPath)
}

// EnsureProviderSymlinks reads the provider ID from settings and creates
// provider-specific symlinks at the BaseDir root. For example, with "anthropic"
// it creates .claude/ → .agents/ and CLAUDE.md → AGENTS.md.
func EnsureProviderSymlinks() error {
	s := LoadSettings()
	if s.Provider == nil {
		return nil
	}

	dirAlias, fileAlias := ProviderMapping(s.Provider.ID)
	if dirAlias == "" {
		return nil // unknown provider, no symlinks
	}

	base := BaseDir()

	// .agents/ must exist before we link to it
	agentsDir := filepath.Join(base, ".agents")
	if !isFileOrDir(agentsDir) {
		return nil // nothing to link to yet
	}

	// Create directory symlink: .claude/ → .agents/
	dirLink := filepath.Join(base, dirAlias)
	if err := EnsureSymlink(agentsDir, dirLink); err != nil {
		return err
	}

	// Create file symlink: CLAUDE.md → AGENTS.md
	agentsFile := filepath.Join(base, "AGENTS.md")
	if isFileOrDir(agentsFile) {
		fileLink := filepath.Join(base, fileAlias)
		if err := EnsureSymlink(agentsFile, fileLink); err != nil {
			return err
		}
	}

	return nil
}

// isFileOrDir returns true if path exists (as a file, directory, or symlink).
func isFileOrDir(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

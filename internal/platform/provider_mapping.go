package platform

// ProviderMapping returns the provider-specific directory alias and file alias
// for a given provider ID. These are used to create symlinks from the canonical
// .agents/ and AGENTS.md to provider-specific names (e.g., .claude/ and CLAUDE.md).
// Returns empty strings for unknown providers (no symlinks created).
func ProviderMapping(providerID string) (dirAlias, fileAlias string) {
	switch providerID {
	case "anthropic":
		return ".claude", "CLAUDE.md"
	default:
		return "", ""
	}
}

package provider

// Provider represents an AI provider that can run tasks.
type Provider interface {
	// ID returns the unique identifier for this provider (e.g. "anthropic").
	ID() string
	// Name returns the display name.
	Name() string
	// Detect checks if the provider's CLI tool is available on PATH.
	Detect() bool
	// CheckAuth verifies the provider is authenticated and ready.
	CheckAuth() error
	// Version returns the CLI tool version string, or empty if unavailable.
	Version() string
}

// Registry holds all known providers.
var Registry = map[string]Provider{
	"anthropic": &Anthropic{},
}

// Get returns a provider by ID, or nil if not found.
func Get(id string) Provider {
	return Registry[id]
}

// List returns all registered providers.
func List() []Provider {
	var providers []Provider
	for _, p := range Registry {
		providers = append(providers, p)
	}
	return providers
}

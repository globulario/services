package dnsprovider

import (
	"fmt"
	"sync"
)

// ProviderFactory creates a Provider from configuration.
type ProviderFactory func(cfg Config) (Provider, error)

var (
	mu        sync.RWMutex
	factories = make(map[string]ProviderFactory)
)

// Register registers a provider factory for a given type.
// This should be called during package init() by provider implementations.
//
// Example:
//   func init() {
//       dnsprovider.Register("godaddy", NewGoDaddyProvider)
//   }
func Register(providerType string, factory ProviderFactory) {
	mu.Lock()
	defer mu.Unlock()

	if factory == nil {
		panic("dnsprovider: Register factory is nil")
	}
	if _, exists := factories[providerType]; exists {
		panic("dnsprovider: Register called twice for provider " + providerType)
	}
	factories[providerType] = factory
}

// NewProvider creates a Provider instance from configuration.
// Returns an error if the provider type is unknown or initialization fails.
//
// Example:
//   cfg := dnsprovider.Config{
//       Type: "godaddy",
//       Zone: "globular.cloud",
//       Credentials: map[string]string{
//           "api_key": os.Getenv("GODADDY_API_KEY"),
//           "api_secret": os.Getenv("GODADDY_API_SECRET"),
//       },
//       DefaultTTL: 600,
//   }
//   provider, err := dnsprovider.NewProvider(cfg)
func NewProvider(cfg Config) (Provider, error) {
	mu.RLock()
	factory, exists := factories[cfg.Type]
	mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("dnsprovider: unknown provider type %q", cfg.Type)
	}

	// Validate common fields
	if cfg.Zone == "" {
		return nil, fmt.Errorf("dnsprovider: zone is required")
	}

	// Apply defaults
	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = 600 // 10 minutes default
	}

	return factory(cfg)
}

// ListProviders returns a list of registered provider types.
func ListProviders() []string {
	mu.RLock()
	defer mu.RUnlock()

	providers := make([]string, 0, len(factories))
	for name := range factories {
		providers = append(providers, name)
	}
	return providers
}

// IsRegistered checks if a provider type is registered.
func IsRegistered(providerType string) bool {
	mu.RLock()
	defer mu.RUnlock()

	_, exists := factories[providerType]
	return exists
}

package email

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register makes a provider available under name. It is intended to be
// called from a provider file's init function:
//
//	func init() { email.Register("resend", newResendFromEnv) }
//
// Register panics if name is empty, factory is nil, or name is taken.
func Register(name string, factory Factory) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || factory == nil {
		panic("email: Register requires a name and a factory")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic("email: provider registered twice: " + name)
	}
	registry[name] = factory
}

// Providers returns the sorted names of all registered providers.
func Providers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// NewEmailProvider builds the provider registered under name (the value of
// EMAIL_PROVIDER). Switching providers is a configuration change only.
func NewEmailProvider(ctx context.Context, name string, opts Options) (EmailProvider, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	registryMu.RLock()
	factory, ok := registry[key]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("email: unknown provider %q (registered: %s)", name, strings.Join(Providers(), ", "))
	}
	if opts.Lookup == nil {
		return nil, errors.New("email: Options.Lookup is required")
	}
	if strings.TrimSpace(opts.To) == "" {
		return nil, errors.New("email: SALES_NOTIFICATION_EMAIL is required")
	}
	provider, err := factory(ctx, opts)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// requireEnv returns an error naming every key that is unset.
func requireEnv(lookup func(string) string, keys ...string) error {
	var missing []string
	for _, key := range keys {
		if strings.TrimSpace(lookup(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

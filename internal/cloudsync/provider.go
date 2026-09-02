package cloudsync

import "fmt"

// CloudProvider abstracts a remote sync provider.
// Each implementation knows how to validate credentials,
// create a CloudBackend, and ping the remote service.
// The underlying database is always SQLite-compatible (e.g. libSQL/Turso).
type CloudProvider interface {
	Name() string
	NewBackend(cfg Config) (CloudBackend, error)
	Ping(cfg Config) error
}

var registeredProviders = map[string]CloudProvider{
	"turso": TursoProvider{},
}

// LookupProvider returns the CloudProvider for name.
// An empty name resolves to the default provider ("turso").
func LookupProvider(name string) (CloudProvider, error) {
	if name == "" {
		name = "turso"
	}
	p, ok := registeredProviders[name]
	if !ok {
		return nil, fmt.Errorf("unknown cloud provider %q (available: turso)", name)
	}
	return p, nil
}

// TursoProvider implements CloudProvider for Turso/libSQL databases.
type TursoProvider struct{}

func (TursoProvider) Name() string { return "turso" }

func (TursoProvider) NewBackend(cfg Config) (CloudBackend, error) {
	b, err := NewTursoBackend(cfg)
	if err != nil {
		return nil, err
	}
	if err := b.Migrate(); err != nil {
		return nil, fmt.Errorf("turso migrate: %w", err)
	}
	return b, nil
}

// Ping verifies the cloud credentials with a minimal round-trip (SELECT 1).
func (TursoProvider) Ping(cfg Config) error {
	b, err := NewTursoBackend(cfg)
	if err != nil {
		return err
	}
	_, err = b.pipeline([]hranaStmt{{SQL: "SELECT 1"}})
	return err
}

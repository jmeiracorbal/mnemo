package turso

import (
	"github.com/jmeiracorbal/mnemo/internal/cloudsync"
)

// Provider implements cloudsync.CloudProvider for Turso/libSQL databases.
type Provider struct{}

func (Provider) Name() string { return "turso" }

func (Provider) NewBackend(cfg cloudsync.Config) (cloudsync.CloudBackend, error) {
	b, err := New(cfg)
	if err != nil {
		return nil, err
	}
	if err := b.Migrate(); err != nil {
		return nil, err
	}
	return b, nil
}

// Ping verifies cloud credentials with a minimal round-trip (SELECT 1).
func (Provider) Ping(cfg cloudsync.Config) error {
	b, err := New(cfg)
	if err != nil {
		return err
	}
	_, err = b.pipeline([]hranaStmt{{SQL: "SELECT 1"}})
	return err
}

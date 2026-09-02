package cloudsync

// CloudProvider abstracts a remote sync provider.
// Each implementation knows how to validate credentials,
// create a CloudBackend, and ping the remote service.
// The underlying database is always SQLite-compatible (e.g. libSQL/Turso).
type CloudProvider interface {
	Name() string
	NewBackend(cfg Config) (CloudBackend, error)
	Ping(cfg Config) error
}

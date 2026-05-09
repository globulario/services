package coordination

import (
	"database/sql"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// Store is the persistence layer for agent coordination memory.
type Store struct {
	db *sql.DB
}

// New returns a Store backed by the awareness graph.
func New(g *graph.Graph) *Store {
	return &Store{db: g.DB()}
}

// DB returns the underlying *sql.DB. Use sparingly — prefer Store methods.
// Exposed for tests that need direct DB access.
func (s *Store) DB() *sql.DB { return s.db }

// joinPipe encodes a []string as a pipe-separated string.
func joinPipe(ss []string) string { return strings.Join(ss, "|") }

// splitPipe decodes a pipe-separated string into a []string.
func splitPipe(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "|")
}

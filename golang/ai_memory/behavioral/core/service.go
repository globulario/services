// Package core implements the behavioral-memory kernel (api.Core).
//
// PR-2 implements the ingestion half (ingestion.go); PR-3 the governance half
// (governance.go); PR-4 the runtime half (runtime.go). As of PR-4 the kernel
// implements all 12 Core operations.
//
// Generic-kernel rule: this package depends only on behavioral/api,
// behavioral/store, and behavioral/domain. It must never import cluster systems
// (etcd, ScyllaDB, MinIO, Envoy, DNS) or Globular cluster packages — the store
// adapter (store/scylla_store.go) is the sole holder of the Scylla dependency.
package core

import (
	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// Service is the kernel implementation. It holds the persistence port and the
// domain registry.
type Service struct {
	store    store.Store
	registry *domain.Registry
}

// Compile-time assertion that Service satisfies the Core boundary.
var _ api.Core = (*Service)(nil)

// New constructs the kernel with its persistence port and domain registry.
func New(st store.Store, reg *domain.Registry) *Service {
	if reg == nil {
		reg = domain.NewRegistry()
	}
	return &Service{store: st, registry: reg}
}

// Ingestion RPCs → ingestion.go (PR-2); governance RPCs → governance.go (PR-3);
// runtime RPCs (ResolveGovernedContext, CheckAction, RecordOutcome) → runtime.go
// (PR-4). With PR-4 the kernel implements all 12 Core operations.

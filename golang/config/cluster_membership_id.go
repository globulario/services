package config

import (
	"context"
	"errors"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ClusterMembershipIDKey is the canonical etcd key holding the cluster's
// MEMBERSHIP IDENTITY — an opaque, immutable UUID minted once by the
// cluster-controller at Day-0.
//
// This is deliberately DISTINCT from the cluster DNS/storage domain
// (config.GetDomain()). The domain is a mutable attribute used for FQDNs,
// DNS, and the ScyllaDB/workflow partition namespace; it must never again be
// used as the cluster's security/membership identity. See the identity
// meta-principle (ai-memory dda8d669) and
// docs/design/cluster-id-minted-uuid-migration.md.
//
// Contract for /globular/system/cluster/id:
//   - The cluster-controller is the SOLE writer. It mints once and NEVER
//     overwrites; any attempt to replace an existing value with a different one
//     is a bug and must be refused.
//   - Readers MUST NOT synthesize, derive, coerce, or default this value. Where
//     identity is required, absence/corruption is fail-closed — NEVER the domain,
//     NEVER "globular.internal".
const ClusterMembershipIDKey = "/globular/system/cluster/id"

// ErrClusterMembershipIDAbsent is returned when the membership id has not been
// minted yet, or is empty/corrupt. Callers that require identity MUST fail
// closed on this — they must not fall back to the domain or a default.
var ErrClusterMembershipIDAbsent = errors.New(
	"cluster membership id absent: " + ClusterMembershipIDKey + " not minted")

// membershipKVGetter is the minimal read surface needed to resolve the
// membership id. *clientv3.Client satisfies it; tests supply a fake.
type membershipKVGetter interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
}

// ReadClusterMembershipID reads the canonical cluster membership UUID from etcd.
// Fail-closed: returns ErrClusterMembershipIDAbsent when the key is missing or
// empty, and the transport error otherwise. It NEVER derives the value from the
// domain and NEVER defaults — a consumer that needs identity and gets an error
// here must surface it, not invent one.
func ReadClusterMembershipID(ctx context.Context) (string, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return "", err
	}
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return readClusterMembershipID(rctx, cli)
}

// readClusterMembershipID is the testable core: it performs one Get and applies
// the fail-closed contract. No fallback, no derivation.
func readClusterMembershipID(ctx context.Context, g membershipKVGetter) (string, error) {
	resp, err := g.Get(ctx, ClusterMembershipIDKey)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Kvs) == 0 {
		return "", ErrClusterMembershipIDAbsent
	}
	id := strings.TrimSpace(string(resp.Kvs[0].Value))
	if id == "" {
		return "", ErrClusterMembershipIDAbsent
	}
	return id, nil
}

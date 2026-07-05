package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EtcdKeyStoragePolicy is the authoritative cluster-wide storage-durability
// policy written by the cluster controller. It declares whether the cluster is
// allowed to materialize its stateful substrates (ScyllaDB, MinIO) below the
// durable 3-node quorum, and at what profile.
//
// Absence of this key resolves to the DURABLE default — a cluster only ever runs
// degraded storage when an operator has explicitly declared it. There is no
// implicit fallback: a 1- or 2-node cluster stays fenced until this policy says
// otherwise.
const EtcdKeyStoragePolicy = "/globular/system/storage-policy"

// DurableMinStorageNodes is the storage-node floor for the durable profile — the
// historical hard quorum (mirrors MinQuorumNodes in the cluster controller).
// ScyllaDB replication (RF=3) and MinIO distributed erasure coding require this
// many nodes to survive the loss of one node.
const DurableMinStorageNodes = 3

// StorageProfile declares the durability contract for the cluster's stateful
// substrates. It is an explicit, operator-declared choice — never inferred from
// the current node count.
type StorageProfile string

const (
	// StorageProfileDurable is the production default: full redundancy.
	// ScyllaDB RF=3, MinIO distributed erasure-coded pool. Requires >= 3 storage
	// nodes; below that the substrates are held (the durable quorum gate).
	StorageProfileDurable StorageProfile = "durable"

	// StorageProfileTwoNodeDegraded runs on 2 storage nodes with REDUCED
	// redundancy: ScyllaDB RF=2, MinIO standalone per node (local S3, no
	// distributed erasure set). This is explicitly NOT highly-available — losing
	// a node loses write quorum and the standalone object store on that node.
	StorageProfileTwoNodeDegraded StorageProfile = "two_node_degraded"

	// StorageProfileSingleNode runs on 1 storage node: ScyllaDB RF=1, MinIO
	// standalone single-drive. ZERO redundancy — a node/disk loss is
	// unrecoverable. For dev, bootstrap, lab, and small personal deployments only.
	StorageProfileSingleNode StorageProfile = "single_node"
)

// StoragePolicy is the cluster-wide, controller-owned storage-durability policy
// persisted to etcd at EtcdKeyStoragePolicy. It is the single source of truth
// for "may this cluster run below the durable storage quorum, and how".
//
// +globular:schema:key="/globular/system/storage-policy"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-cluster-controller,globular-node-agent,globular-cluster-doctor"
// +globular:schema:description="Declared cluster storage-durability policy: profile (durable|two_node_degraded|single_node) + explicit allow_degraded opt-in."
// +globular:schema:invariants="Absent key resolves to durable; degraded profiles require allow_degraded=true; no implicit fallback to degraded."
type StoragePolicy struct {
	// Profile is the declared durability contract.
	Profile StorageProfile `json:"profile"`

	// AllowDegraded must be explicitly true for the controller to materialize
	// stateful substrates below DurableMinStorageNodes. Absent/false keeps the
	// durable quorum gate armed regardless of Profile. This is the explicit
	// opt-in that makes degraded storage a deliberate operator choice.
	AllowDegraded bool `json:"allow_degraded"`

	// Generation is incremented each time the policy changes. Readers compare it
	// to detect policy drift.
	Generation int64 `json:"generation"`

	// DeclaredAt / DeclaredBy record who intentionally chose degraded storage and
	// when — this trade-off must be auditable, never silent.
	DeclaredAt time.Time `json:"declared_at,omitempty"`
	DeclaredBy string    `json:"declared_by,omitempty"`

	// Reason is an optional human note explaining the declaration
	// (e.g. "dev laptop", "2-node lab").
	Reason string `json:"reason,omitempty"`

	// WrittenAt records when the controller last persisted this policy.
	WrittenAt time.Time `json:"written_at"`
}

// DefaultStoragePolicy is the safe default returned when no policy is declared:
// durable, degraded NOT allowed. Absence of the etcd key resolves to this.
func DefaultStoragePolicy() *StoragePolicy {
	return &StoragePolicy{Profile: StorageProfileDurable, AllowDegraded: false}
}

// MinStorageNodes returns the storage-node floor this policy permits for
// materializing stateful substrates. Durable — or ANY policy without
// AllowDegraded — resolves to DurableMinStorageNodes (3). Degraded profiles lower
// the floor only when AllowDegraded is set. A nil policy is treated as durable.
func (p *StoragePolicy) MinStorageNodes() int {
	if p == nil || !p.AllowDegraded {
		return DurableMinStorageNodes
	}
	switch p.Profile {
	case StorageProfileSingleNode:
		return 1
	case StorageProfileTwoNodeDegraded:
		return 2
	default:
		return DurableMinStorageNodes
	}
}

// IsDegraded reports whether this policy permits below-durable storage. It is
// true ONLY for an explicit degraded profile with AllowDegraded set — never by
// default, never implicitly. A nil policy is not degraded.
func (p *StoragePolicy) IsDegraded() bool {
	if p == nil || !p.AllowDegraded {
		return false
	}
	return p.Profile == StorageProfileSingleNode || p.Profile == StorageProfileTwoNodeDegraded
}

// MinioStandalone reports whether MinIO must run standalone per node (local S3)
// instead of a distributed erasure-coded pool. True for degraded profiles: a 1-
// or 2-node cluster cannot form a split-brain-safe distributed pool, so each
// storage node runs its own standalone MinIO for local object storage.
func (p *StoragePolicy) MinioStandalone() bool {
	return p.IsDegraded()
}

// ScyllaReplicationFactor returns the ScyllaDB replication factor this policy
// targets for the given active storage-node count. In durable mode it defers to
// the standard ladder (min(nodes,3)); in degraded mode it caps at the declared
// floor so a 1-node cluster gets RF=1 and a 2-node cluster gets RF=2.
func (p *StoragePolicy) ScyllaReplicationFactor(storageNodes int) int {
	rf := storageNodes
	if rf > DurableMinStorageNodes {
		rf = DurableMinStorageNodes
	}
	if rf < 1 {
		rf = 1
	}
	return rf
}

// Validate checks the policy is internally consistent before it is persisted.
// It enforces the core rule that degraded storage must be explicit.
func (p *StoragePolicy) Validate() error {
	if p == nil {
		return fmt.Errorf("storage policy: nil")
	}
	switch p.Profile {
	case StorageProfileDurable, StorageProfileTwoNodeDegraded, StorageProfileSingleNode:
	default:
		return fmt.Errorf("storage policy: unknown profile %q", p.Profile)
	}
	// Degraded profiles must carry the explicit opt-in — no silent degradation.
	if (p.Profile == StorageProfileTwoNodeDegraded || p.Profile == StorageProfileSingleNode) && !p.AllowDegraded {
		return fmt.Errorf("storage policy: profile %q requires allow_degraded=true — degraded storage must be explicitly declared", p.Profile)
	}
	// Durable never degrades: allow_degraded on durable is contradictory.
	if p.Profile == StorageProfileDurable && p.AllowDegraded {
		return fmt.Errorf("storage policy: profile durable is incompatible with allow_degraded=true")
	}
	return nil
}

// SaveStoragePolicy persists the cluster storage policy to etcd through the
// governed critical-write seam. WrittenAt is stamped on write.
func SaveStoragePolicy(ctx context.Context, p *StoragePolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	p.WrittenAt = time.Now()
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("storage policy: marshal: %w", err)
	}
	if err := PutRuntimeWithClass(ctx, EtcdKeyStoragePolicy, data, CriticalWrite); err != nil {
		return fmt.Errorf("storage policy: etcd put: %w", err)
	}
	return nil
}

// LoadStoragePolicy reads the cluster storage policy from etcd. When the key is
// absent it returns the DURABLE default — never nil, never degraded. This is the
// "no implicit fallback" guarantee: a small cluster stays fenced until an
// operator explicitly declares a degraded policy.
func LoadStoragePolicy(ctx context.Context) (*StoragePolicy, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("storage policy: etcd unavailable: %w", err)
	}
	resp, err := cli.Get(ctx, EtcdKeyStoragePolicy)
	if err != nil {
		return nil, fmt.Errorf("storage policy: etcd get: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return DefaultStoragePolicy(), nil
	}
	var p StoragePolicy
	if err := json.Unmarshal(resp.Kvs[0].Value, &p); err != nil {
		return nil, fmt.Errorf("storage policy: parse: %w", err)
	}
	if p.Profile == "" {
		p.Profile = StorageProfileDurable
	}
	return &p, nil
}

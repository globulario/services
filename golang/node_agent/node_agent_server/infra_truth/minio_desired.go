package infra_truth

import (
	"fmt"
	"strings"
)

// MinioDesiredInputs carries the raw cluster facts the node-agent gathered (from
// local identity + the controller-published ObjectStoreDesiredState) into the
// pure desired-state builder. The builder stays free of etcd/config dependencies
// so it is unit-testable.
type MinioDesiredInputs struct {
	NodeID    string
	ClusterID string
	LocalIP   string // this node's cluster-facing address (StableIP, never the VIP)

	Mode          string   // "standalone" | "distributed"; derived from pool size when empty
	Nodes         []string // ordered pool node IPs (may include self)
	DrivesPerNode int

	SourceVersion string // optional generation of the ObjectStoreDesiredState
	Now           int64  // unix seconds; injected so tests are deterministic
}

// MinioDesired is the MinIO desired state: the generic InfraDesiredState plus the
// MinIO-specific topology facts (mode, drives-per-node) that attestation needs.
type MinioDesired struct {
	InfraDesiredState
	Mode          string
	DrivesPerNode int
}

// BuildMinioDesiredState derives the intended MinIO state for this node with
// explicit provenance. It returns an error when the minimum facts (node id and
// local IP) are missing — the caller turns that into an explicit
// infra.desired_state_unavailable violation rather than silently skipping.
func BuildMinioDesiredState(in MinioDesiredInputs) (*MinioDesired, error) {
	if strings.TrimSpace(in.NodeID) == "" {
		return nil, fmt.Errorf("cannot build minio desired state: node id is empty")
	}
	if strings.TrimSpace(in.LocalIP) == "" {
		return nil, fmt.Errorf("cannot build minio desired state: local IP is empty")
	}

	pool := dedupExcludingEmpty(in.Nodes)
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = MinioModeStandalone
		if len(pool) > 1 {
			mode = MinioModeDistributed
		}
	}

	bootstrap := BootstrapFirstNode
	if hasNonSelf(pool, in.LocalIP) {
		bootstrap = BootstrapJoining
	}

	return &MinioDesired{
		InfraDesiredState: InfraDesiredState{
			Component:                  ComponentMinio,
			NodeID:                     in.NodeID,
			ClusterID:                  in.ClusterID,
			Source:                     SourceComputedFromMembership,
			SourceVersion:              in.SourceVersion,
			GeneratedAt:                in.Now,
			ExpectedListenAddresses:    []string{in.LocalIP},
			ExpectedAdvertiseAddresses: []string{in.LocalIP},
			ExpectedPeers:              pool,
			ExpectedSeeds:              pool,
			BootstrapIntent:            bootstrap,
		},
		Mode:          mode,
		DrivesPerNode: in.DrivesPerNode,
	}, nil
}

// effectiveDrives is the per-node drive count, floored at 1 (drives<2 means a
// single drive per node).
func (d *MinioDesired) effectiveDrives() int {
	if d.DrivesPerNode < 1 {
		return 1
	}
	return d.DrivesPerNode
}

// ExpectedVolumeCount is the total MINIO_VOLUMES entry count the rendered env
// must list: every pool node contributes effectiveDrives volumes (the env on
// every node lists the full pool). A rendered count that disagrees means the
// node would format a different drive topology than the pool — the format.json
// blast-radius risk.
func (d *MinioDesired) ExpectedVolumeCount() int {
	return len(d.ExpectedPeers) * d.effectiveDrives()
}

// minioDesiredMap projects the desired state into the InfraProbeResult.desired
// map, adding the MinIO-specific topology keys on top of the generic projection.
func (d *MinioDesired) minioDesiredMap() map[string]string {
	m := d.desiredMap()
	m["mode"] = d.Mode
	m["drives_per_node"] = fmt.Sprintf("%d", d.effectiveDrives())
	m["expected_volume_count"] = fmt.Sprintf("%d", d.ExpectedVolumeCount())
	return m
}

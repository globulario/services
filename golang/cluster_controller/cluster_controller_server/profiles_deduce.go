package main

import (
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

const (
	_GB = uint64(1024 * 1024 * 1024)

	// Minimum free disk to suggest the storage profile.
	_storageMinDiskFreeBytes = 50 * _GB

	// Minimum RAM + CPU to suggest the control-plane profile (etcd needs headroom).
	_controlPlaneMinRAM  = 2 * _GB
	_controlPlaneMinCPUs = uint32(2)

	// Minimum RAM to suggest the gateway profile (Envoy is memory-hungry).
	_gatewayMinRAM = 1 * _GB

	nodeProfileSourceMetadataKey = "globular_profile_source"
	nodeProfileSourceRequested   = "requested"
	nodeProfileSourceDeduced     = "deduced"
	nodeProfileSourceDefault     = "default"
)

// deduceProfiles returns the profiles that best match the given hardware
// capabilities. "core" is always included as the baseline. Additional profiles
// are added when the node meets the minimum thresholds.
// The optional storageNodeCount argument is accepted for older call sites but
// no longer changes profile selection; quorum is reported from current
// membership, not enforced as an admission floor.
func deduceProfiles(caps *cluster_controllerpb.NodeCapabilities, storageNodeCount ...int) []string {
	suggested := []string{"core"}

	if caps == nil {
		return suggested
	}

	// control-plane: needs RAM for etcd and enough CPUs to stay responsive.
	if caps.GetRamBytes() >= _controlPlaneMinRAM && caps.GetCpuCount() >= _controlPlaneMinCPUs {
		suggested = append(suggested, "control-plane")
	}

	// storage: needs substantial free disk for MinIO object storage.
	if caps.GetDiskFreeBytes() >= _storageMinDiskFreeBytes {
		suggested = append(suggested, "storage")
	}

	// gateway: needs enough RAM to run Envoy comfortably.
	if caps.GetRamBytes() >= _gatewayMinRAM {
		suggested = append(suggested, "gateway")
	}

	return normalizeProfiles(suggested)
}

func profileSourceMetadata(labels map[string]string, source string) map[string]string {
	meta := copyLabels(labels)
	if source == "" {
		return meta
	}
	if meta == nil {
		meta = make(map[string]string, 1)
	}
	meta[nodeProfileSourceMetadataKey] = source
	return meta
}

// refineNodeProfilesFromCapabilities expands a just-joining core-only node from
// measured hardware when the original profile assignment was controller-derived
// rather than operator-requested. It is only allowed before a signed JoinPlan is
// issued; signed profile assignments are immutable admission evidence.
func refineNodeProfilesFromCapabilities(node *nodeState, caps *cluster_controllerpb.NodeCapabilities, signedPlanIssued bool) bool {
	if node == nil || caps == nil {
		return false
	}
	if signedPlanIssued {
		return false
	}
	if node.Metadata != nil && node.Metadata[nodeProfileSourceMetadataKey] == nodeProfileSourceRequested {
		return false
	}
	switch node.JoinLifecyclePhase {
	case JoinPhaseAuthorized, JoinPhaseBootstrapping, JoinPhaseNodeAgentRegistered, JoinPhaseAdmissionPending:
	default:
		return false
	}
	if !profilesAreCoreOnly(node.Profiles) {
		return false
	}
	deduced := deduceProfiles(caps)
	if profilesSameSet(node.Profiles, deduced) {
		return false
	}
	node.Profiles = deduced
	if node.Metadata == nil {
		node.Metadata = make(map[string]string, 1)
	}
	node.Metadata[nodeProfileSourceMetadataKey] = nodeProfileSourceDeduced
	node.EtcdMemberIntent = initialEtcdIntentForProfiles(deduced)
	node.ScyllaIntent = initialScyllaIntentForProfiles(deduced)
	node.ObjectStoreIntent = initialObjectStoreIntentForProfiles(deduced)
	return true
}

func (srv *server) nodeHasSignedJoinPlanLocked(nodeID string) bool {
	if srv == nil || srv.state == nil {
		return false
	}
	for _, jr := range srv.state.JoinRequests {
		if jr == nil || jr.AssignedNodeID != nodeID {
			continue
		}
		if len(jr.JoinPlanJSON) > 0 {
			return true
		}
	}
	return false
}

func profilesAreCoreOnly(profiles []string) bool {
	normalized := normalizeProfiles(profiles)
	return len(normalized) == 1 && normalized[0] == "core"
}

func profilesSameSet(a, b []string) bool {
	aa := normalizeProfiles(a)
	bb := normalizeProfiles(b)
	if len(aa) != len(bb) {
		return false
	}
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

// capsToStored converts a proto NodeCapabilities to the JSON-serializable struct.
func capsToStored(caps *cluster_controllerpb.NodeCapabilities) *storedCapabilities {
	if caps == nil {
		return nil
	}
	return &storedCapabilities{
		CPUCount:           caps.GetCpuCount(),
		RAMBytes:           caps.GetRamBytes(),
		DiskBytes:          caps.GetDiskBytes(),
		DiskFreeBytes:      caps.GetDiskFreeBytes(),
		CanApplyPrivileged: caps.GetCanApplyPrivileged(),
		PrivilegeReason:    caps.GetPrivilegeReason(),
	}
}

// storedToProtoCapabilities converts stored capabilities back to proto.
func storedToProtoCapabilities(c *storedCapabilities) *cluster_controllerpb.NodeCapabilities {
	if c == nil {
		return nil
	}
	return &cluster_controllerpb.NodeCapabilities{
		CpuCount:           c.CPUCount,
		RamBytes:           c.RAMBytes,
		DiskBytes:          c.DiskBytes,
		DiskFreeBytes:      c.DiskFreeBytes,
		CanApplyPrivileged: c.CanApplyPrivileged,
		PrivilegeReason:    c.PrivilegeReason,
	}
}

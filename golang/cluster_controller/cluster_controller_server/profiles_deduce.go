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
)

// deduceProfiles returns the profiles that best match the given hardware
// capabilities. "core" is always included as the baseline. Additional profiles
// are added when the node meets the minimum thresholds.
//
// The returned slice is normalized (sorted, deduplicated, lowercase).
func deduceProfiles(caps *cluster_controllerpb.NodeCapabilities) []string {
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
	}
}

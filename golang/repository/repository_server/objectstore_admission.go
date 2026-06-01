package main

// objectstore_admission.go — topology-aware MinIO mirror admission gate.
//
// Before the repository uses MinIO as a mirror it must pass two checks:
//
//  1. Topology safety — reads ObjectStoreDesiredState and the local node's
//     DiskCandidates from etcd. Rejects if:
//       - the configured MinIO path is on a network-mounted filesystem
//         (IsNetworkMount=true in the node's DiskCandidate)
//       - NFS/CIFS mounts are not explicitly allowed (AllowNetworkMounts=false)
//
//  2. Canary — the dep_health watchdog runs PUT/GET/DELETE every health cycle.
//     If the canary fails, mirrorOK is set to false, which prevents the
//     dep_health watchdog from using the mirror.
//
// These checks enforce the three-state model:
//   AVAILABLE     — topology safe + canary passed
//   DEGRADED      — topology safe + canary failed (last cycle)
//   INVALID       — topology unsafe (network mount or overlap detected)
//
// The repository ALWAYS continues from the local POSIX store; none of these
// states affect RequireHealthy() or block repository RPCs.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/config"
)

// MirrorAdmissionStatus describes the result of the MinIO mirror topology check.
type MirrorAdmissionStatus int

const (
	MirrorAdmissionAvailable MirrorAdmissionStatus = iota // topology safe, canary passing
	MirrorAdmissionDegraded                                // topology safe, canary failed recently
	MirrorAdmissionInvalid                                 // topology unsafe — do not use as mirror
)

func (s MirrorAdmissionStatus) String() string {
	switch s {
	case MirrorAdmissionAvailable:
		return "AVAILABLE"
	case MirrorAdmissionDegraded:
		return "DEGRADED"
	case MirrorAdmissionInvalid:
		return "INVALID"
	default:
		return "UNKNOWN"
	}
}

// checkMinioTopologySafe verifies the configured MinIO path on this node is
// not on a network-mounted filesystem. Returns (AVAILABLE, "") when safe, or
// (INVALID, reason) when topology is unsafe.
//
// This is a lightweight pre-flight check: it reads from etcd but does not
// run cross-node duplicate detection (that belongs to cluster_doctor).
func checkMinioTopologySafe(ctx context.Context, nodeID, nodeIP string) (MirrorAdmissionStatus, string) {
	// Read ObjectStoreDesiredState to find this node's configured MinIO path.
	desired, err := config.LoadObjectStoreDesiredState(ctx)
	if err != nil {
		// etcd unavailable — can't validate topology. Optimistic: allow mirror.
		slog.Warn("objectstore-admission: cannot read desired state from etcd — skipping topology check", "err", err)
		return MirrorAdmissionAvailable, ""
	}
	if desired == nil {
		// No desired state yet — cluster not yet formed. Allow mirror optimistically.
		return MirrorAdmissionAvailable, ""
	}

	// Find this node's configured MinIO path.
	configuredPath := desired.NodePaths[nodeIP]
	if configuredPath == "" {
		configuredPath = "/var/lib/globular/minio"
	}

	// Read this node's disk candidates to find the filesystem serving that path.
	candidates, err := config.LoadDiskCandidates(ctx, nodeID)
	if err != nil || len(candidates) == 0 {
		// Cannot read candidates — skip topology check, allow optimistically.
		return MirrorAdmissionAvailable, ""
	}

	// Find the best-matching candidate for the configured path.
	var best *config.DiskCandidate
	for _, dc := range candidates {
		mp := dc.MountPath
		if mp == configuredPath || strings.HasPrefix(configuredPath, mp+"/") {
			if best == nil || len(mp) > len(best.MountPath) {
				best = dc
			}
		}
	}
	if best == nil {
		// No matching candidate — can't determine topology. Allow optimistically.
		return MirrorAdmissionAvailable, ""
	}

	if best.IsNetworkMount {
		reason := fmt.Sprintf(
			"MinIO path %q is on a network-mounted filesystem (fs=%s, source=%s). "+
				"Network-backed MinIO paths are blocked by default. "+
				"Two pool nodes sharing the same NFS export corrupt each other's format.json. "+
				"Configure a locally-attached block device for MinIO data. "+
				"Doctor finding: objectstore.network_mount_used, objectstore.duplicate_physical_path.",
			configuredPath, best.FSType, best.MountSource)
		slog.Error("objectstore-admission: MinIO mirror INVALID — network mount detected",
			"path", configuredPath,
			"fs_type", best.FSType,
			"mount_source", best.MountSource,
		)
		return MirrorAdmissionInvalid, reason
	}

	return MirrorAdmissionAvailable, ""
}

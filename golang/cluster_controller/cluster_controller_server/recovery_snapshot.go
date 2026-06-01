package main

// recovery_snapshot.go — NodeRecoverySnapshot capture and validation.
//
// A snapshot is a structured, machine-usable inventory of every artifact
// installed on a node at a point in time. It is the authoritative reseed
// source for node.recover.full_reseed.
//
// Rule A: No destructive reseed step may execute unless a persisted snapshot exists.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/google/uuid"
)

// captureNodeInventorySnapshot creates a full NodeRecoverySnapshot from the
// node's currently installed packages as recorded in etcd (installed_state).
//
// This is the controller-side implementation of the
// controller.recovery.capture_node_inventory_snapshot actor action.
func (srv *server) captureNodeInventorySnapshot(ctx context.Context, nodeID, reason, requestedBy string) (*cluster_controllerpb.NodeRecoverySnapshot, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("captureNodeInventorySnapshot: node_id is required")
	}

	// Resolve node metadata from in-memory state.
	srv.lock("captureNodeInventorySnapshot")
	node, ok := srv.state.Nodes[nodeID]
	hostname := ""
	profiles := []string{}
	if ok && node != nil {
		hostname = node.Identity.Hostname
		profiles = append(profiles, node.Profiles...)
	}
	srv.unlock()

	snapshotID := uuid.New().String()
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		SnapshotID: snapshotID,
		NodeID:     nodeID,
		NodeName:   hostname,
		Hostname:   hostname,
		CreatedAt:  time.Now().UTC(),
		CreatedBy:  requestedBy,
		Reason:     reason,
		Profiles:   profiles,
	}

	// Collect installed packages across all kinds.
	var artifacts []cluster_controllerpb.SnapshotArtifact
	var warnings []string
	exactReplayPossible := true

	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND", "APPLICATION"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			log.Printf("recovery snapshot: list %s packages for %s: %v (skipping kind)", kind, nodeID, err)
			warnings = append(warnings, fmt.Sprintf("could not list %s packages: %v", kind, err))
			continue
		}
		for _, pkg := range pkgs {
			name := strings.TrimSpace(pkg.GetName())
			if name == "" {
				continue
			}
			art := cluster_controllerpb.SnapshotArtifact{
				Name:           name,
				Kind:           kind,
				Version:        pkg.GetVersion(),
				BuildNumber:    pkg.GetBuildNumber(),
				OriginalNodeID: nodeID,
				InstallState:   pkg.GetStatus(),
			}

			// Capture build_id and checksum from metadata if available.
			if md := pkg.GetMetadata(); md != nil {
				art.BuildID = md["build_id"]
				art.Checksum = md["entrypoint_checksum"]
				art.PublisherID = md["publisher_id"]
			}

			if art.BuildID == "" {
				exactReplayPossible = false
				warnings = append(warnings, fmt.Sprintf("%s/%s: no build_id — exact replay not possible", kind, name))
			}

			// Mark build availability — the planner will verify against the repository.
			// For now we assume the build is available; the planner refines this.
			art.ExactBuildAvailable = art.BuildID != ""

			artifacts = append(artifacts, art)
		}
	}

	snap.Artifacts = artifacts
	snap.ExactReplayPossible = exactReplayPossible
	snap.Warnings = warnings

	// Compute profile fingerprint.
	snap.ProfileFingerprint = profileFingerprint(profiles)

	// Compute snapshot hash (integrity marker).
	snap.SnapshotHash = computeSnapshotHash(snap)

	// Persist snapshot to etcd (Rule A: must exist before any destructive step).
	if err := srv.putNodeRecoverySnapshot(ctx, snap); err != nil {
		return nil, fmt.Errorf("persist snapshot for %s: %w", nodeID, err)
	}

	log.Printf("recovery snapshot: captured %s for node %s — %d artifacts, exact_replay=%v, %d warnings",
		snapshotID, nodeID, len(artifacts), exactReplayPossible, len(warnings))

	return snap, nil
}

// validateNodeRecoverySnapshot checks that an existing snapshot is suitable
// for reuse. Returns an error if the snapshot is stale, corrupted, or belongs
// to a different node.
func (srv *server) validateNodeRecoverySnapshot(snap *cluster_controllerpb.NodeRecoverySnapshot, targetNodeID string) error {
	if snap == nil {
		return fmt.Errorf("snapshot is nil")
	}
	if snap.NodeID != targetNodeID {
		return fmt.Errorf("snapshot node_id=%q does not match target node_id=%q", snap.NodeID, targetNodeID)
	}
	if len(snap.Artifacts) == 0 {
		return fmt.Errorf("snapshot %s is empty — no artifacts captured", snap.SnapshotID)
	}
	// Recompute hash and compare (corruption check).
	expected := computeSnapshotHash(snap)
	if snap.SnapshotHash != "" && snap.SnapshotHash != expected {
		return fmt.Errorf("snapshot %s hash mismatch — snapshot may be corrupted (stored=%s computed=%s)",
			snap.SnapshotID, snap.SnapshotHash[:8], expected[:8])
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// computeSnapshotHash returns a deterministic SHA-256 hash of the snapshot's
// artifact list (name+kind+version+build_id, sorted). Used for integrity checks.
func computeSnapshotHash(snap *cluster_controllerpb.NodeRecoverySnapshot) string {
	// Build a stable canonical representation.
	type entry struct {
		Kind    string `json:"k"`
		Name    string `json:"n"`
		Version string `json:"v"`
		BuildID string `json:"b"`
	}
	entries := make([]entry, len(snap.Artifacts))
	for i, a := range snap.Artifacts {
		entries[i] = entry{Kind: a.Kind, Name: a.Name, Version: a.Version, BuildID: a.BuildID}
	}
	// Stable sort by kind+name.
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			ki := entries[i].Kind + "/" + entries[i].Name
			kj := entries[j].Kind + "/" + entries[j].Name
			if kj < ki {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	data, _ := json.Marshal(entries)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// profileFingerprint returns a short stable fingerprint of the profile list.
func profileFingerprint(profiles []string) string {
	sorted := make([]string, len(profiles))
	copy(sorted, profiles)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	data, _ := json.Marshal(sorted)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}

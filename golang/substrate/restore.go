package substrate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Restore statuses recorded in the marker at RestoreMarkerKey.
const (
	// StatusRestoredUnverified — desired state was restored from backup
	// evidence and has NOT been reconciled against live observation yet.
	// Controllers must not take destructive convergence actions from
	// restored-only evidence while this status stands (controller gate: P2).
	StatusRestoredUnverified = "RESTORED_UNVERIFIED"
	// StatusRestoredVerified — an operator (or a verification pass) attested
	// that restored state has been reconciled against observed reality.
	StatusRestoredVerified = "RESTORED_VERIFIED"
)

// RestoreMarker is the durable receipt of a recovery operation.
type RestoreMarker struct {
	Status           string         `json:"status"`
	Mode             string         `json:"mode"` // from-dump | from-survivor-force-new-cluster
	RestoredAt       string         `json:"restored_at"`
	DumpCreatedAt    string         `json:"dump_created_at,omitempty"`
	DumpClusterUID   string         `json:"dump_cluster_uid,omitempty"`
	DumpDesiredEpoch int64          `json:"dump_desired_epoch,omitempty"`
	DumpSHA256       string         `json:"dump_sha256,omitempty"`
	Counts           map[string]int `json:"counts,omitempty"`
	VerifiedAt       string         `json:"verified_at,omitempty"`
	Note             string         `json:"note,omitempty"`
}

// RestoreOptions controls RestoreDump.
type RestoreOptions struct {
	// Force overrides the cluster-UID guard and overwrites keys that already
	// exist live. Without it, live keys always win: restored state is
	// evidence, not authority.
	Force bool
	// DryRun classifies and reports without writing anything.
	DryRun bool
	// Note is recorded in the restore marker.
	Note string
}

// RestoreResult reports what a restore did (or would do, under DryRun).
type RestoreResult struct {
	Restored        map[RestorePolicy]int
	SkippedPolicy   map[RestorePolicy]int
	SkippedExisting int
	SkippedLease    int
	UnknownPrefixes []string
}

// RestoreDump applies a dump to the store under the classification contract:
//
//   - RESTORE_AUTHORITATIVE and RESTORE_AS_UNVERIFIED keys are written;
//   - REBUILD_FROM_OBSERVATION and DISCARD keys are skipped;
//   - lease-bound keys are skipped — they were ephemeral by construction;
//   - keys that already exist live are skipped unless Force: newer live
//     evidence must never be overwritten by older desired state;
//   - the restore marker is written with status RESTORED_UNVERIFIED.
//
// The target is typically a fresh (or freshly re-bootstrapped) etcd; the
// cluster-UID guard refuses to import a dump from a different cluster.
func RestoreDump(ctx context.Context, kv KV, d *Dump, opts RestoreOptions) (*RestoreResult, error) {
	liveUID, _, err := kv.Get(ctx, ClusterIDKey)
	if err != nil {
		return nil, fmt.Errorf("read live cluster id: %w", err)
	}
	if liveUID != nil && d.Manifest.ClusterUID != "" {
		live := strings.TrimSpace(string(liveUID.Value))
		if live != d.Manifest.ClusterUID && !opts.Force {
			return nil, fmt.Errorf("REFUSED: dump is from cluster %s but the live store belongs to cluster %s — importing it would graft one cluster's desired state onto another (override requires --force)",
				d.Manifest.ClusterUID, live)
		}
	}

	res := &RestoreResult{
		Restored:      map[RestorePolicy]int{},
		SkippedPolicy: map[RestorePolicy]int{},
	}
	unknown := map[string]struct{}{}

	for i := range d.Entries {
		e := &d.Entries[i]
		if e.Key == RestoreMarkerKey {
			continue // never resurrect a prior restore marker
		}
		c := Classify(e.Key)
		if !c.Known {
			unknown[unknownPrefixOf(e.Key)] = struct{}{}
		}
		switch c.Policy {
		case RebuildFromObservation, Discard:
			res.SkippedPolicy[c.Policy]++
			continue
		}
		if e.Lease {
			res.SkippedLease++
			continue
		}
		existing, _, err := kv.Get(ctx, e.Key)
		if err != nil {
			return nil, fmt.Errorf("read live %s: %w", e.Key, err)
		}
		if existing != nil && !opts.Force {
			res.SkippedExisting++
			continue
		}
		if !opts.DryRun {
			if err := kv.Put(ctx, e.Key, e.Value); err != nil {
				return nil, fmt.Errorf("restore %s: %w", e.Key, err)
			}
		}
		res.Restored[c.Policy]++
	}

	for p := range unknown {
		res.UnknownPrefixes = append(res.UnknownPrefixes, p)
	}
	sort.Strings(res.UnknownPrefixes)

	if !opts.DryRun {
		marker := RestoreMarker{
			Status:           StatusRestoredUnverified,
			Mode:             "from-dump",
			RestoredAt:       time.Now().UTC().Format(time.RFC3339),
			DumpCreatedAt:    d.Manifest.CreatedAt,
			DumpClusterUID:   d.Manifest.ClusterUID,
			DumpDesiredEpoch: d.Manifest.DesiredEpoch,
			DumpSHA256:       d.Manifest.PayloadSHA256,
			Note:             opts.Note,
			Counts: map[string]int{
				"restored_authoritative": res.Restored[RestoreAuthoritative],
				"restored_unverified":    res.Restored[RestoreAsUnverified],
				"skipped_existing":       res.SkippedExisting,
				"skipped_lease":          res.SkippedLease,
				"skipped_rebuild":        res.SkippedPolicy[RebuildFromObservation],
				"skipped_discard":        res.SkippedPolicy[Discard],
			},
		}
		if err := WriteMarker(ctx, kv, marker); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// WriteMarker persists a recovery receipt at RestoreMarkerKey.
func WriteMarker(ctx context.Context, kv KV, m RestoreMarker) error {
	data, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return fmt.Errorf("marshal restore marker: %w", err)
	}
	if err := kv.Put(ctx, RestoreMarkerKey, data); err != nil {
		return fmt.Errorf("write restore marker: %w", err)
	}
	return nil
}

// ReadMarker returns the current recovery marker, or nil when none exists.
func ReadMarker(ctx context.Context, kv KV) (*RestoreMarker, error) {
	kvp, _, err := kv.Get(ctx, RestoreMarkerKey)
	if err != nil {
		return nil, err
	}
	if kvp == nil {
		return nil, nil
	}
	var m RestoreMarker
	if err := json.Unmarshal(kvp.Value, &m); err != nil {
		return nil, fmt.Errorf("parse restore marker: %w", err)
	}
	return &m, nil
}

// MarkVerified flips the marker to RESTORED_VERIFIED — the operator's (or a
// future verification pass's) attestation that restored desired state has
// been reconciled against observed reality. It refuses when no marker exists:
// there is nothing to attest.
func MarkVerified(ctx context.Context, kv KV, note string) (*RestoreMarker, error) {
	m, err := ReadMarker(ctx, kv)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("no restore marker at %s — nothing to verify", RestoreMarkerKey)
	}
	m.Status = StatusRestoredVerified
	m.VerifiedAt = time.Now().UTC().Format(time.RFC3339)
	if note != "" {
		m.Note = note
	}
	if err := WriteMarker(ctx, kv, *m); err != nil {
		return nil, err
	}
	return m, nil
}

// unknownPrefixOf reduces a key to a reportable prefix (first three path
// segments) so unknown-prefix reports group by subtree instead of listing
// every key.
func unknownPrefixOf(key string) string {
	parts := strings.SplitN(strings.TrimPrefix(key, "/"), "/", 4)
	if len(parts) >= 3 {
		return "/" + strings.Join(parts[:3], "/") + "/"
	}
	return key
}

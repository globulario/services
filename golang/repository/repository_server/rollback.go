package main

// rollback.go — Phase CLI-C installed-revision history + rollback candidates.
//
// The repository owns the canonical install-history table. Node-agents (and
// the controller on their behalf) call RecordInstalledRevision after every
// successful install / upgrade / rollback. ListInstalledRevisions returns the
// time-ordered history; ListRollbackCandidates filters that history into the
// set of viable rollback targets, applying:
//
//   - artifact must be PUBLISHED (publish_state + pipeline_state)
//   - blob must be present + size match (verifyArtifactIntegrity gate)
//   - signature must pass policy (Phase CLI-B verifyArtifactSignature)
//   - revision must not be REVOKED
//   - (optional, deferred) target must not introduce a downgrade — caller
//     opts in via --allow-downgrade in the CLI / workflow.
//
// The repository-side scaffolding is complete here. The node-agent's actual
// rollback execution (snapshot configs, stop service, install target, verify
// runtime health) lives in the node_agent service and will be wired in a
// separate pass — search for "TODO(rollback-exec)" markers.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Scylla CRUD: installed_revisions ──────────────────────────────────────

func (s *scyllaStore) putInstalledRevision(ctx context.Context, rev *repopb.InstalledPackageRevision) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}
	return sess.Query(`INSERT INTO installed_revisions
		(publisher_id, name, platform, installed_at_unix, revision_id,
		 kind, version, build_id, build_number, checksum, node_id,
		 previous_revision_id, config_snapshot_id,
		 service_status_before, service_status_after, workflow_run_id, action)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rev.GetPublisherId(), rev.GetName(), rev.GetPlatform(),
		rev.GetInstalledAtUnix(), rev.GetRevisionId(),
		rev.GetKind().String(), rev.GetVersion(), rev.GetBuildId(),
		rev.GetBuildNumber(), rev.GetChecksum(), rev.GetNodeId(),
		rev.GetPreviousRevisionId(), rev.GetConfigSnapshotId(),
		rev.GetServiceStatusBefore(), rev.GetServiceStatusAfter(),
		rev.GetInstalledByWorkflowRunId(), rev.GetAction(),
	).WithContext(ctx).Exec()
}

func (s *scyllaStore) listInstalledRevisions(ctx context.Context, publisherID, name, platform, nodeID string, limit int) ([]*repopb.InstalledPackageRevision, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}
	q := sess.Query(`SELECT installed_at_unix, revision_id, kind, version, build_id,
		build_number, checksum, node_id, previous_revision_id, config_snapshot_id,
		service_status_before, service_status_after, workflow_run_id, action
		FROM installed_revisions WHERE publisher_id = ? AND name = ? AND platform = ?`,
		publisherID, name, platform).WithContext(ctx)
	iter := q.Iter()
	var (
		installedAt int64
		revID, kindStr, version, buildID, checksum, node, prev, cfgSnap, ssb, ssa, runID, action string
		buildNumber int64
	)
	var out []*repopb.InstalledPackageRevision
	for iter.Scan(&installedAt, &revID, &kindStr, &version, &buildID,
		&buildNumber, &checksum, &node, &prev, &cfgSnap, &ssb, &ssa, &runID, &action) {
		if nodeID != "" && node != nodeID {
			continue
		}
		k := repopb.ArtifactKind_SERVICE
		if v, ok := repopb.ArtifactKind_value[kindStr]; ok {
			k = repopb.ArtifactKind(v)
		}
		out = append(out, &repopb.InstalledPackageRevision{
			PublisherId: publisherID, Name: name, Platform: platform,
			Kind: k, Version: version, BuildId: buildID,
			BuildNumber: buildNumber, Checksum: checksum,
			InstalledAtUnix: installedAt, RevisionId: revID, NodeId: node,
			PreviousRevisionId:  prev,
			ConfigSnapshotId:    cfgSnap,
			ServiceStatusBefore: ssb, ServiceStatusAfter: ssa,
			InstalledByWorkflowRunId: runID, Action: action,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// ── In-memory fallback for tests (no Scylla session) ─────────────────────

type revisionCache struct {
	rows []*repopb.InstalledPackageRevision
}

func (srv *server) initRevisionCache() {
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if srv.revisions == nil {
		srv.revisions = &revisionCache{}
	}
}

func (srv *server) cacheRevision(rev *repopb.InstalledPackageRevision) {
	srv.initRevisionCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	srv.revisions.rows = append(srv.revisions.rows, rev)
}

func (srv *server) listCachedRevisions(publisherID, name, platform, nodeID string, limit int) []*repopb.InstalledPackageRevision {
	srv.initRevisionCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	var out []*repopb.InstalledPackageRevision
	for _, r := range srv.revisions.rows {
		if r.GetPublisherId() != publisherID || r.GetName() != name || r.GetPlatform() != platform {
			continue
		}
		if nodeID != "" && r.GetNodeId() != nodeID {
			continue
		}
		out = append(out, r)
	}
	// Newest first.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].GetInstalledAtUnix() > out[j].GetInstalledAtUnix()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ── High-level helpers ───────────────────────────────────────────────────

// generateRevisionID returns a 16-byte hex string suitable for
// InstalledPackageRevision.revision_id (UUID-ish without external dep).
func generateRevisionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: timestamp + counter — never empty.
		return fmt.Sprintf("rev-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (srv *server) saveInstalledRevision(ctx context.Context, rev *repopb.InstalledPackageRevision) error {
	if rev.GetRevisionId() == "" {
		rev.RevisionId = generateRevisionID()
	}
	if rev.GetInstalledAtUnix() == 0 {
		rev.InstalledAtUnix = time.Now().Unix()
	}
	srv.cacheRevision(rev)
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			return ss.putInstalledRevision(ctx, rev)
		}
	}
	return nil
}

func (srv *server) loadInstalledRevisions(ctx context.Context, publisherID, name, platform, nodeID string, limit int) []*repopb.InstalledPackageRevision {
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			if rows, err := ss.listInstalledRevisions(ctx, publisherID, name, platform, nodeID, limit); err == nil && len(rows) > 0 {
				return rows
			}
		}
	}
	return srv.listCachedRevisions(publisherID, name, platform, nodeID, limit)
}

// ── Rollback candidate evaluation ────────────────────────────────────────

// evaluateRollbackCandidate runs the same gates the resolver / DownloadArtifact
// use, plus a signature policy check. Returns RollbackEligibility.
func (srv *server) evaluateRollbackCandidate(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) *repopb.RollbackEligibility {
	v, err := srv.verifyArtifactIntegrity(ctx, ref, buildNumber)
	if err != nil || v == nil {
		return &repopb.RollbackEligibility{
			Eligible:     false,
			Reason:       "verify failed",
			VerifyStatus: repopb.ArtifactVerifyStatus_ARTIFACT_VERIFY_INCONCLUSIVE,
		}
	}
	verifyStatus := mapVerifyStatus(v.Status)
	key := artifactKeyWithBuild(ref, buildNumber)
	pipelineState := srv.readArtifactState(ctx, key)
	if pipelineState == PipelineRevoked {
		return &repopb.RollbackEligibility{
			Eligible: false, Reason: "REVOKED — terminal", VerifyStatus: verifyStatus,
		}
	}
	if pipelineState == PipelineQuarantined {
		return &repopb.RollbackEligibility{
			Eligible: false, Reason: "QUARANTINED — not a rollback target", VerifyStatus: verifyStatus,
		}
	}
	if v.Status != VerifyOK {
		return &repopb.RollbackEligibility{
			Eligible: false, Reason: v.Reason, VerifyStatus: verifyStatus,
		}
	}

	// Phase F signature policy: central decision (Required vs Allowed) so
	// rollback eligibility matches the same rules resolver / DownloadArtifact
	// use. Required + missing → not eligible; revoked key → never eligible.
	expectedDigest := v.ExpectedSHA
	sigDec := srv.signaturePolicyDecision(ctx, ref, key, expectedDigest, "")
	if !sigDec.Allowed {
		return &repopb.RollbackEligibility{
			Eligible: false, Reason: sigDec.Reason,
			VerifyStatus: verifyStatus, SignatureStatus: sigDec.Status,
		}
	}
	return &repopb.RollbackEligibility{
		Eligible: true, Reason: "ok",
		VerifyStatus: verifyStatus, SignatureStatus: sigDec.Status,
	}
}

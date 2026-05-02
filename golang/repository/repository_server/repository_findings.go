package main

// repository_findings.go — Phase F Part 4 ListRepositoryFindings RPC and the
// scan loop that produces them.
//
// The cluster_doctor pulls these on each snapshot tick. Doctor renders them
// as Findings with operator-readable remediation hints. The scan logic lives
// here in the repository service so the service that owns the truth also
// owns the integrity checks; the doctor never re-implements verification.

import (
	"context"
	"fmt"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ListRepositoryFindings scans the manifest catalog and emits one finding
// per integrity issue. Heavy hashing is deliberately avoided — only Stat-only
// checks plus signature lookups (which are O(1) per artifact).
func (srv *server) ListRepositoryFindings(ctx context.Context, req *repopb.ListRepositoryFindingsRequest) (*repopb.ListRepositoryFindingsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 200
	}
	kindFilter := req.GetKindFilter()

	resp := &repopb.ListRepositoryFindingsResponse{GeneratedAtUnix: time.Now().Unix()}

	if srv.scylla == nil {
		return resp, nil
	}
	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		return resp, nil // best-effort; caller treats absence as "no findings"
	}

	policy := srv.ensureSignaturePolicy().CurrentPolicy(ctx)
	now := time.Now().Unix()

	for i := range rows {
		if len(resp.Findings) >= limit {
			break
		}
		row := rows[i]
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			// Non-PUBLISHED rows can still be REVOKED/QUARANTINED but
			// "installable" doctor invariants only fire when something
			// claims to be installable. Skip — these are healthy from
			// the integrity-of-PUBLISHED perspective.
			//
			// Exception: revoked / quarantined rows that the resolver can
			// still see (artifact_state stale).
			if row.PublishState == repopb.PublishState_REVOKED.String() ||
				row.PublishState == repopb.PublishState_QUARANTINED.String() {
				if shouldEmit(kindFilter, repopb.RepositoryFindingKind_REPO_FIND_REVOKED_INSTALLABLE,
					repopb.RepositoryFindingKind_REPO_FIND_QUARANTINED_INSTALLABLE) {
					if f := srv.evalLifecycleCoherence(ctx, &row, now); f != nil {
						if matchKindFilter(kindFilter, f.GetKind()) {
							resp.Findings = append(resp.Findings, f)
						}
					}
				}
			}
			continue
		}
		ref := &repopb.ArtifactRef{
			PublisherId: row.PublisherID, Name: row.Name,
			Version: row.Version, Platform: row.Platform,
		}

		// 1) Blob present + size match.
		present, reason := srv.artifactBlobStatus(ctx, ref, row.BuildNumber, row.SizeBytes)
		if !present {
			kind := repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_MISSING_BLOB
			if reason == "size_mismatch" {
				kind = repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH
			}
			if matchKindFilter(kindFilter, kind) {
				resp.Findings = append(resp.Findings,
					srv.buildBlobFinding(&row, ref, kind, reason, now))
			}
			continue
		}

		// 2) Signature policy: required + missing/invalid.
		dec := srv.signaturePolicyDecision(ctx, ref, row.ArtifactKey, row.Checksum, "")
		if dec.Required && !dec.Allowed {
			kind := repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED
			if matchKindFilter(kindFilter, kind) {
				resp.Findings = append(resp.Findings,
					srv.buildSignatureFinding(&row, ref, dec, now))
			}
		}

		// 3) Lifecycle coherence: PUBLISHED row with non-PUBLISHED pipeline state.
		if f := srv.evalLifecycleCoherence(ctx, &row, now); f != nil {
			if matchKindFilter(kindFilter, f.GetKind()) {
				resp.Findings = append(resp.Findings, f)
			}
		}
		_ = policy // currently unused; reserved for future per-publisher rules
	}
	return resp, nil
}

// shouldEmit returns true if at least one of the given kinds passes the filter.
func shouldEmit(filter repopb.RepositoryFindingKind, kinds ...repopb.RepositoryFindingKind) bool {
	if filter == repopb.RepositoryFindingKind_REPOSITORY_FINDING_UNSPECIFIED {
		return true
	}
	for _, k := range kinds {
		if filter == k {
			return true
		}
	}
	return false
}

func matchKindFilter(filter, k repopb.RepositoryFindingKind) bool {
	return filter == repopb.RepositoryFindingKind_REPOSITORY_FINDING_UNSPECIFIED || filter == k
}

func (srv *server) buildBlobFinding(row *manifestRow, ref *repopb.ArtifactRef, kind repopb.RepositoryFindingKind, reason string, now int64) *repopb.RepositoryFinding {
	return &repopb.RepositoryFinding{
		Kind:               kind,
		Severity:           repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL,
		ArtifactKey:        row.ArtifactKey,
		Ref:                ref,
		CurrentState:       string(srv.readArtifactState(context.Background(), row.ArtifactKey)),
		ExpectedState:      string(PipelinePublished) + " + blob present",
		Reason:             fmt.Sprintf("PUBLISHED row but %s", reason),
		RecommendedCommand: fmt.Sprintf("globular repository repair %s/%s %s --platform %s",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()),
		Evidence: map[string]string{
			"checksum":    row.Checksum,
			"size_bytes":  fmt.Sprintf("%d", row.SizeBytes),
			"blob_status": reason,
		},
		ObservedAtUnix: now,
	}
}

func (srv *server) buildSignatureFinding(row *manifestRow, ref *repopb.ArtifactRef, dec SignaturePolicyDecision, now int64) *repopb.RepositoryFinding {
	severity := repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL
	switch dec.Status {
	case repopb.SignatureStatus_SIGNATURE_REVOKED_KEY,
		repopb.SignatureStatus_SIGNATURE_INVALID,
		repopb.SignatureStatus_SIGNATURE_DIGEST_MISMATCH:
		severity = repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL
	default:
		severity = repopb.RepositoryFindingSeverity_REPO_FIND_ERROR
	}
	return &repopb.RepositoryFinding{
		Kind:               repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED,
		Severity:           severity,
		ArtifactKey:        row.ArtifactKey,
		Ref:                ref,
		CurrentState:       string(srv.readArtifactState(context.Background(), row.ArtifactKey)),
		ExpectedState:      "PUBLISHED + valid trusted signature",
		Reason:             dec.Reason,
		RecommendedCommand: fmt.Sprintf("globular repository signature verify %s/%s %s --platform %s",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()),
		Evidence: map[string]string{
			"signature_status": dec.Status.String(),
		},
		ObservedAtUnix: now,
	}
}

// evalLifecycleCoherence detects rows where publish_state and artifact_state
// disagree in dangerous ways: e.g. publish_state=PUBLISHED but artifact_state
// is REVOKED — the resolver could leak access. Or vice versa.
func (srv *server) evalLifecycleCoherence(ctx context.Context, row *manifestRow, now int64) *repopb.RepositoryFinding {
	pipeline := srv.readArtifactState(ctx, row.ArtifactKey)
	publish := row.PublishState
	ref := &repopb.ArtifactRef{
		PublisherId: row.PublisherID, Name: row.Name,
		Version: row.Version, Platform: row.Platform,
	}
	switch {
	case publish == repopb.PublishState_PUBLISHED.String() && pipeline == PipelineRevoked:
		return &repopb.RepositoryFinding{
			Kind:           repopb.RepositoryFindingKind_REPO_FIND_REVOKED_INSTALLABLE,
			Severity:       repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL,
			ArtifactKey:    row.ArtifactKey,
			Ref:            ref,
			CurrentState:   "publish_state=PUBLISHED, artifact_state=REVOKED",
			ExpectedState:  "both REVOKED",
			Reason:         "REVOKED artifact has stale publish_state=PUBLISHED",
			RecommendedCommand: fmt.Sprintf("globular repository artifact revoke %s/%s %s --platform %s",
				ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()),
			ObservedAtUnix: now,
		}
	case publish == repopb.PublishState_PUBLISHED.String() && pipeline == PipelineQuarantined:
		return &repopb.RepositoryFinding{
			Kind:           repopb.RepositoryFindingKind_REPO_FIND_QUARANTINED_INSTALLABLE,
			Severity:       repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL,
			ArtifactKey:    row.ArtifactKey,
			Ref:            ref,
			CurrentState:   "publish_state=PUBLISHED, artifact_state=QUARANTINED",
			ExpectedState:  "both QUARANTINED",
			Reason:         "QUARANTINED artifact has stale publish_state=PUBLISHED",
			RecommendedCommand: fmt.Sprintf("globular repository artifact quarantine %s/%s %s --platform %s",
				ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()),
			ObservedAtUnix: now,
		}
	case publish == repopb.PublishState_REVOKED.String() && pipeline != PipelineRevoked && pipeline != PipelineUnspecified:
		return &repopb.RepositoryFinding{
			Kind:           repopb.RepositoryFindingKind_REPO_FIND_REVOKED_INSTALLABLE,
			Severity:       repopb.RepositoryFindingSeverity_REPO_FIND_ERROR,
			ArtifactKey:    row.ArtifactKey,
			Ref:            ref,
			CurrentState:   fmt.Sprintf("publish_state=REVOKED, artifact_state=%s", pipeline),
			ExpectedState:  "both REVOKED",
			Reason:         "publish_state=REVOKED but pipeline_state has not been moved",
			RecommendedCommand: fmt.Sprintf("globular repository artifact revoke %s/%s %s --platform %s",
				ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()),
			ObservedAtUnix: now,
		}
	}
	return nil
}

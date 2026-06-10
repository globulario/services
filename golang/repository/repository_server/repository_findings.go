package main

// repository_findings.go — Phase F Part 4 ListRepositoryFindings RPC and the
// scan loop that produces them.
//
// The cluster_doctor pulls these on each snapshot tick. Doctor renders them
// as Findings with operator-readable remediation hints. The scan logic lives
// here in the repository service so the service that owns the truth also
// owns the integrity checks; the doctor never re-implements verification.
//
// Also contains GetRepositoryStatus — the operational mode RPC that must
// answer even when ScyllaDB is down (never calls requireHealthy).

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/operational"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ListRepositoryFindings scans the manifest catalog and emits one finding
// per integrity issue. Heavy hashing is deliberately avoided — only Stat-only
// checks plus signature lookups (which are O(1) per artifact).
func (srv *server) ListRepositoryFindings(ctx context.Context, req *repopb.ListRepositoryFindingsRequest) (*repopb.ListRepositoryFindingsResponse, error) {
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

	// Emit meta-invariant findings about watchdog/mode coherence.
	for _, f := range srv.evalDependencyModeCoherence(now) {
		if len(resp.Findings) >= limit {
			break
		}
		if matchKindFilter(kindFilter, f.GetKind()) {
			resp.Findings = append(resp.Findings, f)
		}
	}
	// Emit identity findings for duplicate checksum rows missing alias linkage.
	for _, f := range srv.evalDuplicateChecksumAliasGaps(ctx, rows, now) {
		if len(resp.Findings) >= limit {
			break
		}
		if matchKindFilter(kindFilter, f.GetKind()) {
			resp.Findings = append(resp.Findings, f)
		}
	}
	for _, f := range srv.evalBuildIDChecksumConflicts(rows, now) {
		if len(resp.Findings) >= limit {
			break
		}
		if matchKindFilter(kindFilter, f.GetKind()) {
			resp.Findings = append(resp.Findings, f)
		}
	}
	for _, f := range srv.evalVersionResolutionAmbiguity(rows, now) {
		if len(resp.Findings) >= limit {
			break
		}
		if matchKindFilter(kindFilter, f.GetKind()) {
			resp.Findings = append(resp.Findings, f)
		}
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

func (srv *server) evalDuplicateChecksumAliasGaps(ctx context.Context, rows []manifestRow, now int64) []*repopb.RepositoryFinding {
	type groupKey struct {
		Publisher string
		Name      string
		Version   string
		Platform  string
		Checksum  string
	}
	groups := make(map[groupKey][]manifestRow)
	for _, row := range rows {
		if row.PublishState != repopb.PublishState_PUBLISHED.String() || strings.TrimSpace(row.Checksum) == "" {
			continue
		}
		k := groupKey{
			Publisher: row.PublisherID,
			Name:      row.Name,
			Version:   row.Version,
			Platform:  row.Platform,
			Checksum:  row.Checksum,
		}
		groups[k] = append(groups[k], row)
	}

	var out []*repopb.RepositoryFinding
	for k, group := range groups {
		if len(group) < 2 {
			continue
		}
		seenBID := map[string]bool{}
		for _, r := range group {
			m, _, err := manifestFromRow(r)
			if err != nil || m == nil || strings.TrimSpace(m.GetBuildId()) == "" {
				continue
			}
			if bid := strings.TrimSpace(m.GetBuildId()); bid != "" {
				seenBID[bid] = true
			}
		}
		if len(seenBID) < 2 {
			continue
		}
		for _, row := range group {
			manifest, _, err := manifestFromRow(row)
			if err != nil || manifest == nil || manifest.GetUpstreamImport() == nil {
				out = append(out, &repopb.RepositoryFinding{
					Kind:           repopb.RepositoryFindingKind_REPO_FIND_CONFIG_CONFLICT,
					Severity:       repopb.RepositoryFindingSeverity_REPO_FIND_WARN,
					ArtifactKey:    row.ArtifactKey,
					Ref:            &repopb.ArtifactRef{PublisherId: row.PublisherID, Name: row.Name, Version: row.Version, Platform: row.Platform},
					CurrentState:   "duplicate checksum group without alias metadata",
					ExpectedState:  "release/build alias must map to canonical build_id",
					Reason:         "repository.identity.duplicate_checksum_without_alias: missing upstream import metadata for alias check",
					ObservedAtUnix: now,
					Evidence: map[string]string{
						"publisher": k.Publisher,
						"name":      k.Name,
						"version":   k.Version,
						"platform":  k.Platform,
						"checksum":  k.Checksum,
					},
				})
				continue
			}
			ui := manifest.GetUpstreamImport()
			ref := manifest.GetRef()
			alias, _ := srv.loadReleaseBuildAlias(ctx, ref, ui.GetReleaseTag(), ui.GetBuildNumber())
			if alias == nil || strings.TrimSpace(alias.CanonicalBuildID) == "" {
				out = append(out, &repopb.RepositoryFinding{
					Kind:               repopb.RepositoryFindingKind_REPO_FIND_CONFIG_CONFLICT,
					Severity:           repopb.RepositoryFindingSeverity_REPO_FIND_WARN,
					ArtifactKey:        row.ArtifactKey,
					Ref:                ref,
					CurrentState:       "duplicate checksum group without alias",
					ExpectedState:      "release/build alias must map to canonical build_id",
					Reason:             "repository.identity.duplicate_checksum_without_alias: alias record missing",
					RecommendedCommand: fmt.Sprintf("globular repository sync --source %s --tag %s", ui.GetSourceName(), ui.GetReleaseTag()),
					ObservedAtUnix:     now,
					Evidence: map[string]string{
						"checksum":      row.Checksum,
						"build_id":      manifest.GetBuildId(),
						"build_number":  fmt.Sprintf("%d", row.BuildNumber),
						"release_tag":   ui.GetReleaseTag(),
						"source_name":   ui.GetSourceName(),
						"identity_rule": "duplicate_checksum_requires_alias",
					},
				})
			}
		}
	}
	return out
}

func (srv *server) evalBuildIDChecksumConflicts(rows []manifestRow, now int64) []*repopb.RepositoryFinding {
	type bidState struct {
		manifest *repopb.ArtifactManifest
		row      manifestRow
	}
	seen := make(map[string]map[string]bidState) // build_id -> checksum -> sample
	for _, row := range rows {
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			continue
		}
		m, _, err := manifestFromRow(row)
		if err != nil || m == nil {
			continue
		}
		bid := strings.TrimSpace(m.GetBuildId())
		sum := strings.TrimSpace(m.GetChecksum())
		if bid == "" || sum == "" {
			continue
		}
		if _, ok := seen[bid]; !ok {
			seen[bid] = make(map[string]bidState)
		}
		seen[bid][sum] = bidState{manifest: m, row: row}
	}

	var out []*repopb.RepositoryFinding
	for bid, sums := range seen {
		if len(sums) < 2 {
			continue
		}
		for checksum, state := range sums {
			ref := state.manifest.GetRef()
			out = append(out, &repopb.RepositoryFinding{
				Kind:               repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH,
				Severity:           repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL,
				ArtifactKey:        state.row.ArtifactKey,
				Ref:                ref,
				CurrentState:       "same build_id appears with multiple checksums",
				ExpectedState:      "build_id maps to exactly one checksum",
				Reason:             "repository.identity.build_id_checksum_conflict",
				RecommendedCommand: "globular repository doctor identity",
				ObservedAtUnix:     now,
				Evidence: map[string]string{
					"build_id":  bid,
					"checksum":  checksum,
					"conflicts": fmt.Sprintf("%d", len(sums)),
					"invariant": "build_id->single_checksum",
				},
			})
		}
	}
	return out
}

func (srv *server) evalVersionResolutionAmbiguity(rows []manifestRow, now int64) []*repopb.RepositoryFinding {
	type key struct {
		Publisher string
		Name      string
		Version   string
		Platform  string
	}
	groups := make(map[key]map[string]manifestRow) // identity -> build_id -> sample row
	for _, row := range rows {
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			continue
		}
		m, _, err := manifestFromRow(row)
		if err != nil || m == nil {
			continue
		}
		bid := strings.TrimSpace(m.GetBuildId())
		if bid == "" {
			continue
		}
		k := key{
			Publisher: row.PublisherID,
			Name:      row.Name,
			Version:   row.Version,
			Platform:  row.Platform,
		}
		if _, ok := groups[k]; !ok {
			groups[k] = make(map[string]manifestRow)
		}
		groups[k][bid] = row
	}

	var out []*repopb.RepositoryFinding
	for k, builds := range groups {
		if len(builds) < 2 {
			continue
		}
		var sample manifestRow
		for _, row := range builds {
			sample = row
			break
		}
		out = append(out, &repopb.RepositoryFinding{
			Kind:               repopb.RepositoryFindingKind_REPO_FIND_CONFIG_CONFLICT,
			Severity:           repopb.RepositoryFindingSeverity_REPO_FIND_ERROR,
			ArtifactKey:        sample.ArtifactKey,
			Ref:                &repopb.ArtifactRef{PublisherId: k.Publisher, Name: k.Name, Version: k.Version, Platform: k.Platform},
			CurrentState:       "multiple published build_ids exist for same package version/platform",
			ExpectedState:      "desired-state and installs must pin build_id for deterministic resolution",
			Reason:             "repository.identity.version_resolution_ambiguous",
			RecommendedCommand: "globular repository doctor identity",
			ObservedAtUnix:     now,
			Evidence: map[string]string{
				"publisher":       k.Publisher,
				"name":            k.Name,
				"version":         k.Version,
				"platform":        k.Platform,
				"published_builds": fmt.Sprintf("%d", len(builds)),
				"invariant":       "version_resolution_requires_build_id_pin",
			},
		})
	}
	return out
}

// blobFindingSeverity returns WARNING when an upstream repair source is known,
// CRITICAL when no source can provide the blob (unrecoverable without manual intervention).
// This implements the repository.metadata_without_verified_artifact invariant:
//   - WARNING → "repair source exists, run globular repository repair"
//   - CRITICAL → "no source, manual re-import or re-publish required"
func (srv *server) blobFindingSeverity(row *manifestRow) repopb.RepositoryFindingSeverity {
	manifest, _, parseErr := manifestFromRow(*row)
	if parseErr == nil && manifest != nil {
		if ui := manifest.GetUpstreamImport(); ui != nil && ui.GetSourceName() != "" {
			return repopb.RepositoryFindingSeverity_REPO_FIND_WARN
		}
	}
	return repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL
}

func (srv *server) buildBlobFinding(row *manifestRow, ref *repopb.ArtifactRef, kind repopb.RepositoryFindingKind, reason string, now int64) *repopb.RepositoryFinding {
	reasonCode := fmt.Sprintf("repository.identity.blob_integrity: %s", reason)
	switch kind {
	case repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_MISSING_BLOB:
		reasonCode = "repository.identity.missing_blob_for_published_manifest"
	case repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_CHECKSUM_MISMATCH:
		reasonCode = "repository.identity.checksum_mismatch"
	}

	severity := srv.blobFindingSeverity(row)
	recommended := fmt.Sprintf("globular repository repair %s/%s %s --platform %s",
		ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())

	// Every blob finding is a LOCAL POSIX CAS observation on THIS repository
	// instance — stamp the scope so a negative never reads as a cluster-wide
	// assertion (meta.assertions_must_carry_their_scope).
	evidence := map[string]string{
		"checksum":    row.Checksum,
		"size_bytes":  fmt.Sprintf("%d", row.SizeBytes),
		"blob_status": reason,
		"invariant":   "repository.metadata_without_verified_artifact",
		"blob_scope":  "local_posix_cas",
		"instance":    srv.GetAddress(),
	}

	// Replication-lag disambiguation. A PUBLISHED manifest is cluster-wide
	// (shared ScyllaDB index), but blobs are per-instance local CAS replicated
	// asynchronously via the MinIO mirror. A blob absent from THIS instance's
	// local CAS but PRESENT in the shared mirror is replication lag on this
	// instance — NOT data loss — and must not read as CRITICAL. This is the
	// false-positive storm seen during multi-node joins, when a freshly
	// converged repository instance has the full manifest index but an
	// incomplete local CAS. Reserve CRITICAL/WARN for blobs missing locally
	// AND from the mirror (true loss). Mirror presence is consulted for
	// REPORTING ONLY; installability authority stays local (artifactBlobStatus).
	if kind == repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_MISSING_BLOB &&
		reason == "missing_blob" &&
		srv.artifactBlobInMirror(context.Background(), ref, row.BuildNumber) {
		severity = repopb.RepositoryFindingSeverity_REPO_FIND_INFO
		reasonCode = "repository.identity.blob_absent_local_present_mirror"
		evidence["blob_status"] = "missing_local_present_mirror"
		evidence["remediation_hint"] = "blob present in shared mirror; awaiting local CAS replication on this instance — not data loss"
		recommended = "" // no operator action — local replication is automatic
	}

	return &repopb.RepositoryFinding{
		Kind:               kind,
		Severity:           severity,
		ArtifactKey:        row.ArtifactKey,
		Ref:                ref,
		CurrentState:       string(srv.readArtifactState(context.Background(), row.ArtifactKey)),
		ExpectedState:      string(PipelinePublished) + " + local blob present + checksum verified",
		Reason:             reasonCode,
		RecommendedCommand: recommended,
		Evidence:           evidence,
		ObservedAtUnix:     now,
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

// GetRepositoryStatus returns the live operational mode and per-capability
// status of this repository instance. It intentionally never calls
// requireHealthy or requireCapability — it must answer even when Scylla is
// down, because its primary use is to diagnose degraded states.
func (srv *server) GetRepositoryStatus(_ context.Context, _ *repopb.GetRepositoryStatusRequest) (*repopb.GetRepositoryStatusResponse, error) {
	if srv.depHealth == nil {
		// No watchdog — cannot prove dependency state; report DEGRADED with UNKNOWN capabilities.
		return &repopb.GetRepositoryStatusResponse{
			Service: "repository.PackageRepository",
			Mode:    string(operational.ModeDegraded),
			Reason:  "dependency_watchdog_not_initialized",
			Capabilities: []*repopb.CapabilityHealthProto{
				{Name: CapRepoWrite, Status: string(operational.CapUnknown)},
				{Name: CapRepoQuery, Status: string(operational.CapUnknown)},
				{Name: CapRepoRead, Status: string(operational.CapUnknown)},
				{Name: CapRepoMirror, Status: string(operational.CapUnknown)},
			},
			ObservedAtUnix: time.Now().Unix(),
		}, nil
	}
	s := srv.depHealth.OperationalStatus()

	resp := &repopb.GetRepositoryStatusResponse{
		Service:        s.Service,
		Mode:           string(s.Mode),
		Reason:         s.Reason,
		ObservedAtUnix: s.ObservedAtUnix,
	}
	for _, d := range s.Dependencies {
		resp.Dependencies = append(resp.Dependencies, &repopb.DependencyHealthProto{
			Name:                d.Name,
			Kind:                string(d.Kind),
			Status:              string(d.Status),
			Reason:              d.Reason,
			AffectsCapabilities: d.AffectsCapabilities,
		})
	}
	for _, c := range s.Capabilities {
		resp.Capabilities = append(resp.Capabilities, &repopb.CapabilityHealthProto{
			Name:   c.Name,
			Status: string(c.Status),
			Mode:   string(c.Mode),
			Reason: c.Reason,
		})
	}
	return resp, nil
}

// evalDependencyModeCoherence emits a finding when the reported service mode
// is inconsistent with the actual dependency state. This is a meta-invariant:
// it detects bugs in the watchdog itself, not in artifact state.
//
//   - REPO_FIND_SCYLLA_DOWN_MODE_INCONSISTENT: Scylla is down but service
//     reports mode=FULL (should be READ_ONLY or LOCAL_ONLY).
//   - REPO_FIND_MINIO_BLOCKS_REPOSITORY: MinIO is unavailable and the service
//     is reporting a non-mirror capability as blocked by the mirror (optional
//     deps must never block required capabilities).
func (srv *server) evalDependencyModeCoherence(now int64) []*repopb.RepositoryFinding {
	if srv.depHealth == nil {
		return nil
	}
	s := srv.depHealth.OperationalStatus()
	var findings []*repopb.RepositoryFinding

	// Invariant 1: if Scylla is reported unavailable, mode must not be FULL.
	scyllaDown := false
	for _, d := range s.Dependencies {
		if d.Name == "scylladb" && d.Status == operational.DepUnavailable {
			scyllaDown = true
			break
		}
	}
	if scyllaDown && s.Mode == operational.ModeFull {
		findings = append(findings, &repopb.RepositoryFinding{
			Kind:           repopb.RepositoryFindingKind_REPO_FIND_SCYLLA_DOWN_MODE_INCONSISTENT,
			Severity:       repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL,
			Reason:         "scylladb dependency is UNAVAILABLE but service mode is FULL — watchdog inconsistency",
			ExpectedState:  "mode=READ_ONLY or mode=LOCAL_ONLY when scylladb is unavailable",
			CurrentState:   fmt.Sprintf("mode=%s, scylladb=UNAVAILABLE", s.Mode),
			ObservedAtUnix: now,
		})
	}

	// Invariant 2: MinIO mirror unavailability must only block CapRepoMirror,
	// never CapRepoWrite, CapRepoQuery, or CapRepoRead.
	minioDown := false
	for _, d := range s.Dependencies {
		if d.Name == "minio_mirror" && d.Status == operational.DepUnavailable {
			minioDown = true
			break
		}
	}
	if minioDown {
		for _, c := range s.Capabilities {
			if c.Name == CapRepoMirror {
				continue // mirror being blocked by mirror-down is correct
			}
			if c.Status == operational.CapBlocked {
				findings = append(findings, &repopb.RepositoryFinding{
					Kind:     repopb.RepositoryFindingKind_REPO_FIND_MINIO_BLOCKS_REPOSITORY,
					Severity: repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL,
					Reason: fmt.Sprintf(
						"optional MinIO mirror is blocking capability %q — mirror must never block non-mirror capabilities",
						c.Name),
					ExpectedState:  fmt.Sprintf("capability %s=AVAILABLE when only mirror is down", c.Name),
					CurrentState:   fmt.Sprintf("capability %s=BLOCKED, minio_mirror=UNAVAILABLE", c.Name),
					ObservedAtUnix: now,
				})
			}
		}
	}
	return findings
}

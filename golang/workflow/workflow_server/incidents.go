package main

// Incident model — operator surface for a self-correcting control plane.
//
// See docs/incidents-design.md for the full design. This file implements:
//   - the scanner (joins telemetry → incidents per §10.2)
//   - ListIncidents / GetIncident / ApplyIncidentAction / SubmitProposedFix RPCs
//   - severity derivation (§3.5), headline generation (§3.4), resolution (§4.3)

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	incidentCategoryWorkflowFailure   = "workflow_failure"
	incidentCategoryDriftStuck        = "drift_stuck"
	incidentCategoryServiceUnhealthy  = "service_unhealthy"
	incidentCategoryNodeUnreachable   = "node_unreachable"
	incidentScanInterval              = 60 * time.Second
	incidentHeadlineMaxLen            = 80
)

// Per-category resolution N (§4.3).
var incidentResolutionN = map[string]int{
	incidentCategoryWorkflowFailure:   3,
	incidentCategoryDriftStuck:        2,
	incidentCategoryServiceUnhealthy:  3,
	incidentCategoryNodeUnreachable:   5,
	"auth_denied":                     1,
	"phantom_gossip":                  10,
}

// Category base severity (§3.5 Step 1).
var incidentCategoryBaseSeverity = map[string]workflowpb.IncidentSeverity{
	incidentCategoryWorkflowFailure:  workflowpb.IncidentSeverity_INCIDENT_SEVERITY_WARN,
	incidentCategoryDriftStuck:       workflowpb.IncidentSeverity_INCIDENT_SEVERITY_WARN,
	incidentCategoryServiceUnhealthy: workflowpb.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
	incidentCategoryNodeUnreachable:  workflowpb.IncidentSeverity_INCIDENT_SEVERITY_ERROR,
	"auth_denied":                    workflowpb.IncidentSeverity_INCIDENT_SEVERITY_WARN,
	"phantom_gossip":                 workflowpb.IncidentSeverity_INCIDENT_SEVERITY_WARN,
}

// ---------------------------------------------------------------------------
// Scanner
// ---------------------------------------------------------------------------

// runIncidentScanner aggregates telemetry signals into incidents every minute.
// Idempotent — reads telemetry tables, upserts incidents, marks absent-for-N
// incidents as RESOLVED.
func (srv *server) runIncidentScanner() {
	ticker := time.NewTicker(incidentScanInterval)
	defer ticker.Stop()
	for range ticker.C {
		srv.scanOnce()
	}
}

func (srv *server) scanOnce() {
	// Get any cluster_id present in the telemetry tables.
	var clusterID string
	if err := srv.session.Query(
		`SELECT cluster_id FROM workflow_run_summaries LIMIT 1`,
	).Scan(&clusterID); err != nil || clusterID == "" {
		return
	}

	// Track which incidents were present in this scan; any OPEN incident
	// not in this set gets its absent_scans counter incremented.
	presentIDs := make(map[string]bool)

	srv.scanWorkflowFailures(clusterID, presentIDs)
	srv.scanDriftStuck(clusterID, presentIDs)

	// For every OPEN incident NOT present this scan, increment absent_scans;
	// transition to RESOLVED if absent_scans >= N for its category.
	srv.resolveAbsent(clusterID, presentIDs)
}

// scanWorkflowFailures opens/updates incidents for steps with failures.
func (srv *server) scanWorkflowFailures(clusterID string, present map[string]bool) {
	iter := srv.session.Query(
		`SELECT workflow_name, step_id, total_executions, failure_count,
			success_count, last_error_message, last_finished_at
		 FROM workflow_step_outcomes WHERE cluster_id=?`, clusterID,
	).Iter()
	defer iter.Close()
	for {
		var (
			wf, step, lastErr       string
			total, fail, succ       int64
			lastFinished            time.Time
		)
		if !iter.Scan(&wf, &step, &total, &fail, &succ, &lastErr, &lastFinished) {
			break
		}
		if fail == 0 || total < 5 {
			continue
		}
		// failure rate >= 10%
		if float64(fail)/float64(total) < 0.10 {
			continue
		}
		signature := wf + "/" + step
		entityRef := signature
		id := incidentID(clusterID, incidentCategoryWorkflowFailure, signature)
		present[id] = true

		headline := clampHeadline(fmt.Sprintf("%s step failed · %s", wf, step))
		ev := []*workflowpb.EvidenceItem{{
			Id:         uuid.NewString(),
			Provenance: workflowpb.Provenance_PROVENANCE_OBSERVED,
			Source:     "workflow.step_outcomes",
			Summary: fmt.Sprintf("Step %s/%s failed %d of %d executions",
				wf, step, fail, total),
			Facts: map[string]string{
				"workflow_name":    wf,
				"step_id":          step,
				"total_executions": fmt.Sprintf("%d", total),
				"failure_count":    fmt.Sprintf("%d", fail),
				"success_count":    fmt.Sprintf("%d", succ),
				"last_error":       lastErr,
			},
			ObservedAt: maybeTimestamp(lastFinished),
		}}
		srv.upsertIncident(clusterID, id, incidentCategoryWorkflowFailure, signature,
			entityRef, "step", headline, int32(fail), ev)
	}
}

// scanDriftStuck opens/updates incidents for drift items observed ≥3 cycles.
func (srv *server) scanDriftStuck(clusterID string, present map[string]bool) {
	iter := srv.session.Query(
		`SELECT drift_type, entity_ref, consecutive_cycles, first_observed_at,
			last_observed_at, chosen_workflow
		 FROM drift_unresolved WHERE cluster_id=?`, clusterID,
	).Iter()
	defer iter.Close()
	for {
		var (
			dType, eRef, chosen        string
			cycles                     int
			firstObserved, lastObserved time.Time
		)
		if !iter.Scan(&dType, &eRef, &cycles, &firstObserved, &lastObserved, &chosen) {
			break
		}
		if cycles < 3 {
			continue
		}
		signature := dType + "/" + eRef
		id := incidentID(clusterID, incidentCategoryDriftStuck, signature)
		present[id] = true

		headline := clampHeadline(fmt.Sprintf("Drift %s unresolved %d cycles · %s",
			dType, cycles, eRef))
		ev := []*workflowpb.EvidenceItem{{
			Id:         uuid.NewString(),
			Provenance: workflowpb.Provenance_PROVENANCE_OBSERVED,
			Source:     "workflow.drift_unresolved",
			Summary: fmt.Sprintf("%s on %s unresolved for %d consecutive reconcile cycles",
				dType, eRef, cycles),
			Facts: map[string]string{
				"drift_type":         dType,
				"entity_ref":         eRef,
				"consecutive_cycles": fmt.Sprintf("%d", cycles),
				"chosen_workflow":    chosen,
				"first_observed_at":  firstObserved.UTC().Format(time.RFC3339),
			},
			ObservedAt: maybeTimestamp(lastObserved),
		}}
		entityType := driftEntityType(dType)
		srv.upsertIncident(clusterID, id, incidentCategoryDriftStuck, signature,
			eRef, entityType, headline, int32(cycles), ev)
	}
}

// driftEntityType infers entity_type from drift_type (best-effort).
func driftEntityType(driftType string) string {
	switch driftType {
	case "missing_package", "wrong_version", "unmanaged_package":
		return "package"
	case "node_degraded":
		return "node"
	case "infra_unhealthy":
		return "infra"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Incident upsert (core write path)
// ---------------------------------------------------------------------------

func (srv *server) upsertIncident(clusterID, id, category, signature, entityRef, entityType,
	headline string, occurrenceCount int32, newEvidence []*workflowpb.EvidenceItem) {

	// Load existing incident (if any) to preserve operator state + lifecycle.
	existing, _ := srv.loadIncident(clusterID, id)
	now := time.Now()

	var incident *workflowpb.Incident
	if existing == nil {
		// New incident — OPEN.
		incident = &workflowpb.Incident{
			Id:              id,
			ClusterId:       clusterID,
			Category:        category,
			Signature:       signature,
			Status:          workflowpb.IncidentStatus_INCIDENT_STATUS_OPEN,
			Headline:        headline,
			OccurrenceCount: occurrenceCount,
			FirstSeenAt:     timestamppb.New(now),
			LastSeenAt:      timestamppb.New(now),
			EntityRef:       entityRef,
			EntityType:      entityType,
			Evidence:        newEvidence,
		}
	} else {
		incident = existing
		// Re-open if previously RESOLVED.
		if incident.Status == workflowpb.IncidentStatus_INCIDENT_STATUS_RESOLVED {
			incident.Status = workflowpb.IncidentStatus_INCIDENT_STATUS_OPEN
		}
		incident.Headline = headline // keep fresh
		incident.OccurrenceCount = occurrenceCount
		incident.LastSeenAt = timestamppb.New(now)
		// Replace evidence with the latest snapshot (we could merge, but
		// "newest first" is easier to reason about — §4.4).
		incident.Evidence = newEvidence
	}

	incident.Severity = deriveSeverity(incident)

	srv.saveIncident(incident, 0 /* reset absent_scans on present */)
}

// resolveAbsent increments absent_scans for incidents NOT in the present set.
// Transitions OPEN → RESOLVED when absent_scans >= N for the category.
func (srv *server) resolveAbsent(clusterID string, present map[string]bool) {
	iter := srv.session.Query(
		`SELECT id, category, absent_scans, status FROM incidents WHERE cluster_id=?`,
		clusterID,
	).Iter()
	var toBump []struct {
		id       string
		category string
		absent   int
		status   int
	}
	for {
		var id, category string
		var absent, status int
		if !iter.Scan(&id, &category, &absent, &status) {
			break
		}
		if present[id] {
			continue // will be reset on its own upsert
		}
		if status != int(workflowpb.IncidentStatus_INCIDENT_STATUS_OPEN) &&
			status != int(workflowpb.IncidentStatus_INCIDENT_STATUS_ACKED) &&
			status != int(workflowpb.IncidentStatus_INCIDENT_STATUS_RESOLVING) {
			continue
		}
		toBump = append(toBump, struct {
			id       string
			category string
			absent   int
			status   int
		}{id, category, absent, status})
	}
	_ = iter.Close()

	for _, t := range toBump {
		newAbsent := t.absent + 1
		threshold := incidentResolutionN[t.category]
		if threshold == 0 {
			threshold = 3
		}
		if newAbsent >= threshold {
			// Transition to RESOLVED.
			srv.session.Query(
				`UPDATE incidents SET status=?, absent_scans=? WHERE cluster_id=? AND id=?`,
				int(workflowpb.IncidentStatus_INCIDENT_STATUS_RESOLVED), newAbsent, clusterID, t.id,
			).Exec()
		} else {
			srv.session.Query(
				`UPDATE incidents SET absent_scans=? WHERE cluster_id=? AND id=?`,
				newAbsent, clusterID, t.id,
			).Exec()
		}
	}
}

// ---------------------------------------------------------------------------
// Severity derivation (§3.5)
// ---------------------------------------------------------------------------

func deriveSeverity(inc *workflowpb.Incident) workflowpb.IncidentSeverity {
	sev := incidentCategoryBaseSeverity[inc.Category]
	if sev == 0 {
		sev = workflowpb.IncidentSeverity_INCIDENT_SEVERITY_WARN
	}
	// Recurrence bump.
	if inc.OccurrenceCount >= 20 && sev < workflowpb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL {
		sev = workflowpb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
	} else if inc.OccurrenceCount >= 5 && sev < workflowpb.IncidentSeverity_INCIDENT_SEVERITY_ERROR {
		sev = workflowpb.IncidentSeverity_INCIDENT_SEVERITY_ERROR
	}
	// Diagnosis upgrade.
	for _, d := range inc.Diagnoses {
		if d.Severity == workflowpb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL {
			sev = workflowpb.IncidentSeverity_INCIDENT_SEVERITY_CRITICAL
			break
		}
	}
	return sev
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func incidentID(clusterID, category, signature string) string {
	sum := sha256.Sum256([]byte(clusterID + "|" + category + "|" + signature))
	return hex.EncodeToString(sum[:8])
}

func clampHeadline(s string) string {
	if len(s) <= incidentHeadlineMaxLen {
		return s
	}
	return s[:incidentHeadlineMaxLen-1] + "…"
}

// ---------------------------------------------------------------------------
// Storage (load / save)
// ---------------------------------------------------------------------------

func (srv *server) loadIncident(clusterID, id string) (*workflowpb.Incident, error) {
	var (
		category, signature, headline, entityRef, entityType string
		acknowledgedBy, assignedTo                           string
		status, severity                                     int
		occurrenceCount                                      int
		firstSeen, lastSeen, acknowledgedAt                  time.Time
		acknowledged                                         bool
		evidenceJSON, diagnosesJSON, fixesJSON               string
	)
	err := srv.session.Query(
		`SELECT category, signature, status, severity, headline,
			occurrence_count, first_seen_at, last_seen_at,
			entity_ref, entity_type,
			acknowledged, acknowledged_by, acknowledged_at, assigned_to,
			evidence_json, diagnoses_json, proposed_fixes_json
		 FROM incidents WHERE cluster_id=? AND id=?`,
		clusterID, id,
	).Scan(&category, &signature, &status, &severity, &headline,
		&occurrenceCount, &firstSeen, &lastSeen,
		&entityRef, &entityType,
		&acknowledged, &acknowledgedBy, &acknowledgedAt, &assignedTo,
		&evidenceJSON, &diagnosesJSON, &fixesJSON)
	if err != nil {
		return nil, err
	}
	inc := &workflowpb.Incident{
		Id:              id,
		ClusterId:       clusterID,
		Category:        category,
		Signature:       signature,
		Status:          workflowpb.IncidentStatus(status),
		Severity:        workflowpb.IncidentSeverity(severity),
		Headline:        headline,
		OccurrenceCount: int32(occurrenceCount),
		FirstSeenAt:     maybeTimestamp(firstSeen),
		LastSeenAt:      maybeTimestamp(lastSeen),
		EntityRef:       entityRef,
		EntityType:      entityType,
		Acknowledged:    acknowledged,
		AcknowledgedBy:  acknowledgedBy,
		AcknowledgedAt:  maybeTimestamp(acknowledgedAt),
		AssignedTo:      assignedTo,
	}
	if evidenceJSON != "" {
		_ = json.Unmarshal([]byte(evidenceJSON), &inc.Evidence)
	}
	if diagnosesJSON != "" {
		_ = json.Unmarshal([]byte(diagnosesJSON), &inc.Diagnoses)
	}
	if fixesJSON != "" {
		_ = json.Unmarshal([]byte(fixesJSON), &inc.ProposedFixes)
	}
	return inc, nil
}

func (srv *server) saveIncident(inc *workflowpb.Incident, absentScans int) {
	evidenceJSON, _ := json.Marshal(inc.Evidence)
	diagnosesJSON, _ := json.Marshal(inc.Diagnoses)
	fixesJSON, _ := json.Marshal(inc.ProposedFixes)

	srv.session.Query(
		`INSERT INTO incidents (
			cluster_id, id, category, signature, status, severity, headline,
			occurrence_count, first_seen_at, last_seen_at,
			entity_ref, entity_type,
			acknowledged, acknowledged_by, acknowledged_at, assigned_to,
			evidence_json, diagnoses_json, proposed_fixes_json, absent_scans
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inc.ClusterId, inc.Id, inc.Category, inc.Signature,
		int(inc.Status), int(inc.Severity), inc.Headline,
		int(inc.OccurrenceCount), tsToTime(inc.FirstSeenAt), tsToTime(inc.LastSeenAt),
		inc.EntityRef, inc.EntityType,
		inc.Acknowledged, inc.AcknowledgedBy, tsToTime(inc.AcknowledgedAt), inc.AssignedTo,
		string(evidenceJSON), string(diagnosesJSON), string(fixesJSON), absentScans,
	).Exec()
}

// ---------------------------------------------------------------------------
// RPC handlers
// ---------------------------------------------------------------------------

// ListIncidents returns incidents for a cluster, optionally filtered by status.
// Applies ordering rules from §4.4.
func (srv *server) ListIncidents(_ context.Context, req *workflowpb.ListIncidentsRequest) (*workflowpb.ListIncidentsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	iter := srv.session.Query(
		`SELECT id FROM incidents WHERE cluster_id=?`, req.ClusterId,
	).Iter()
	var ids []string
	for {
		var id string
		if !iter.Scan(&id) {
			break
		}
		ids = append(ids, id)
	}
	_ = iter.Close()

	var out []*workflowpb.Incident
	for _, id := range ids {
		inc, err := srv.loadIncident(req.ClusterId, id)
		if err != nil {
			continue
		}
		if req.Status != 0 && inc.Status != req.Status {
			continue
		}
		sortIncidentChildren(inc)
		out = append(out, inc)
	}

	// Order incidents: OPEN first, then by severity DESC, then last_seen DESC.
	sort.Slice(out, func(i, j int) bool {
		oi, oj := out[i], out[j]
		if (oi.Status == workflowpb.IncidentStatus_INCIDENT_STATUS_OPEN) !=
			(oj.Status == workflowpb.IncidentStatus_INCIDENT_STATUS_OPEN) {
			return oi.Status == workflowpb.IncidentStatus_INCIDENT_STATUS_OPEN
		}
		if oi.Severity != oj.Severity {
			return oi.Severity > oj.Severity
		}
		return oi.GetLastSeenAt().AsTime().After(oj.GetLastSeenAt().AsTime())
	})

	limit := int(req.Limit)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return &workflowpb.ListIncidentsResponse{Incidents: out}, nil
}

func (srv *server) GetIncident(_ context.Context, req *workflowpb.GetIncidentRequest) (*workflowpb.Incident, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.IncidentId == "" {
		return nil, fmt.Errorf("cluster_id and incident_id are required")
	}
	inc, err := srv.loadIncident(req.ClusterId, req.IncidentId)
	if err != nil {
		return nil, err
	}
	sortIncidentChildren(inc)
	return inc, nil
}

// ApplyIncidentAction records an operator action and updates incident state.
// Actions are append-only; state updates are derived from the action verb.
func (srv *server) ApplyIncidentAction(_ context.Context, req *workflowpb.IncidentAction) (*emptypb.Empty, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.IncidentId == "" || req.Action == "" {
		return &emptypb.Empty{}, fmt.Errorf("incident_id and action are required")
	}
	// We need cluster_id; caller may not have it. Look up the incident.
	var clusterID string
	if err := srv.session.Query(
		`SELECT cluster_id FROM incidents WHERE id=? LIMIT 1 ALLOW FILTERING`,
		req.IncidentId,
	).Scan(&clusterID); err != nil {
		return &emptypb.Empty{}, fmt.Errorf("incident not found: %w", err)
	}

	// Append to action log.
	now := time.Now()
	actionID := uuid.NewString()
	srv.session.Query(
		`INSERT INTO incident_actions (
			cluster_id, incident_id, action_at, action_id, action, actor, fix_id, comment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		clusterID, req.IncidentId, now, actionID,
		req.Action, req.Actor, req.FixId, req.Comment,
	).Exec()

	// Update incident state based on action.
	switch req.Action {
	case "ack":
		srv.session.Query(
			`UPDATE incidents SET status=?, acknowledged=true, acknowledged_by=?, acknowledged_at=?
			 WHERE cluster_id=? AND id=?`,
			int(workflowpb.IncidentStatus_INCIDENT_STATUS_ACKED),
			req.Actor, now, clusterID, req.IncidentId,
		).Exec()
	case "apply_fix":
		srv.session.Query(
			`UPDATE incidents SET status=? WHERE cluster_id=? AND id=?`,
			int(workflowpb.IncidentStatus_INCIDENT_STATUS_RESOLVING),
			clusterID, req.IncidentId,
		).Exec()
	}
	return &emptypb.Empty{}, nil
}

// SubmitProposedFix attaches an AI-proposed fix to an incident. Validates
// that the fix cites evidence or diagnosis (§4 citation rule).
func (srv *server) SubmitProposedFix(_ context.Context, req *workflowpb.SubmitProposedFixRequest) (*workflowpb.ProposedFix, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	if req == nil || req.ClusterId == "" || req.IncidentId == "" || req.Fix == nil {
		return nil, fmt.Errorf("cluster_id, incident_id, fix are required")
	}
	fix := req.Fix
	if len(fix.CitedEvidenceIds) == 0 && len(fix.CitedDiagnosisIds) == 0 {
		return nil, fmt.Errorf("proposed fix must cite at least one evidence or diagnosis")
	}
	if fix.Id == "" {
		fix.Id = uuid.NewString()
	}
	fix.ProposedAt = timestamppb.Now()
	fix.TargetIncidentId = req.IncidentId
	if fix.Status == 0 {
		fix.Status = workflowpb.FixStatus_FIX_STATUS_PROPOSED
	}

	inc, err := srv.loadIncident(req.ClusterId, req.IncidentId)
	if err != nil {
		return nil, fmt.Errorf("incident not found: %w", err)
	}
	inc.ProposedFixes = append(inc.ProposedFixes, fix)
	srv.saveIncident(inc, 0)
	return fix, nil
}

// ---------------------------------------------------------------------------
// Ordering helpers (§4.4)
// ---------------------------------------------------------------------------

// sortIncidentChildren applies the ordering rules from §4.4 to an incident's
// evidence, diagnoses, and proposed_fixes arrays.
func sortIncidentChildren(inc *workflowpb.Incident) {
	// Evidence: newest observed_at first.
	sort.SliceStable(inc.Evidence, func(i, j int) bool {
		return inc.Evidence[i].GetObservedAt().AsTime().After(inc.Evidence[j].GetObservedAt().AsTime())
	})
	// Diagnoses: highest severity first, then newest.
	sort.SliceStable(inc.Diagnoses, func(i, j int) bool {
		if inc.Diagnoses[i].Severity != inc.Diagnoses[j].Severity {
			return inc.Diagnoses[i].Severity > inc.Diagnoses[j].Severity
		}
		return inc.Diagnoses[i].GetDiagnosedAt().AsTime().After(inc.Diagnoses[j].GetDiagnosedAt().AsTime())
	})
	// Proposed fixes: confidence DESC, then newest.
	confRank := func(c string) int {
		switch c {
		case "high":
			return 3
		case "medium":
			return 2
		case "low":
			return 1
		}
		return 0
	}
	sort.SliceStable(inc.ProposedFixes, func(i, j int) bool {
		ci, cj := confRank(inc.ProposedFixes[i].Confidence), confRank(inc.ProposedFixes[j].Confidence)
		if ci != cj {
			return ci > cj
		}
		return inc.ProposedFixes[i].GetProposedAt().AsTime().After(inc.ProposedFixes[j].GetProposedAt().AsTime())
	})
}

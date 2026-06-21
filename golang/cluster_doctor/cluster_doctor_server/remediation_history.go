// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.remediation_history
// @awareness file_role=in_memory_remediation_audit_ring_for_failure_rate_gate
// @awareness implements=globular.platform:intent.audit.every_authority_change_is_explainable
// @awareness implements=globular.platform:intent.remediation.failure_rate_policy
// @awareness risk=medium
package main

// This file powers the cross-attempt failure-rate gate inside
// ExecuteRemediation. cluster-doctor is observer-only and must NEVER write to
// etcd (invariant:cluster_doctor.observer_only_never_writes_etcd), so the gate
// reads recent remediation audits from a bounded IN-PROCESS ring populated by
// auditRemediation — not from the old /globular/cluster_doctor/audit/ prefix,
// whose writer was made a no-op in v1.2.166 (which left this gate dead, always
// counting 0 failures). The ring is per-process; cross-restart durability via
// ai-memory remains a tracked follow-up. Read paths must NOT weaken the gate.

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// remediationAuditRing is a bounded, in-process ring of recent RemediationAudit
// records. It is the live data source for the failure-rate breaker now that the
// etcd audit writer is a no-op (observer-only). Per-process lifetime only.
type remediationAuditRing struct {
	mu      sync.Mutex
	buf     []RemediationAudit
	maxSize int
}

// remediationAudits holds recent per-action audits for the failure-rate gate.
// 500 matches the limit ExecuteRemediation passes to countRecentFailedActionAttempts.
var remediationAudits = &remediationAuditRing{maxSize: 500}

func (r *remediationAuditRing) push(a RemediationAudit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, a)
	if len(r.buf) > r.maxSize {
		r.buf = r.buf[len(r.buf)-r.maxSize:]
	}
}

// list returns up to the most recent `limit` audits (all of them if limit<=0).
func (r *remediationAuditRing) list(limit int) []RemediationAudit {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.buf)
	start := 0
	if limit > 0 && limit < n {
		start = n - limit
	}
	out := make([]RemediationAudit, n-start)
	copy(out, r.buf[start:])
	return out
}

// listRemediationAuditsFn is the seam the failure-rate/history readers use.
// Default reads the in-process ring; tests inject fixed histories.
var listRemediationAuditsFn = listRemediationAuditsFromRing

func listRemediationAuditsFromRing(_ context.Context, limit int) ([]RemediationAudit, error) {
	return remediationAudits.list(limit), nil
}

type remediationActionStat struct {
	ActionType string
	Successes  int
	LastTs     int64
}

func summarizeHistoricalSuccessfulActions(ctx context.Context, invariantID, evidenceDigest string, limit int) []remediationActionStat {
	if invariantID == "" || evidenceDigest == "" {
		return nil
	}
	audits, err := listRemediationAuditsFn(ctx, limit)
	if err != nil {
		return nil
	}
	stats := map[string]remediationActionStat{}
	for _, a := range audits {
		if a.InvariantID != invariantID || a.EvidenceDigest != evidenceDigest {
			continue
		}
		if a.Rejected || !a.Executed {
			continue
		}
		current := stats[a.ActionType]
		current.ActionType = a.ActionType
		current.Successes++
		if a.Timestamp > current.LastTs {
			current.LastTs = a.Timestamp
		}
		stats[a.ActionType] = current
	}
	out := make([]remediationActionStat, 0, len(stats))
	for _, s := range stats {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Successes == out[j].Successes {
			return out[i].LastTs > out[j].LastTs
		}
		return out[i].Successes > out[j].Successes
	})
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func historicalActionsHint(stats []remediationActionStat) string {
	if len(stats) == 0 {
		return ""
	}
	hint := "Historical successful actions for similar evidence:"
	for _, s := range stats {
		hint += fmt.Sprintf(" %s(success=%d,last=%s);", s.ActionType, s.Successes, time.Unix(s.LastTs, 0).UTC().Format(time.RFC3339))
	}
	return hint
}

func countRecentFailedActionAttempts(ctx context.Context, invariantID, evidenceDigest, actionType string, since time.Time, limit int) int {
	if invariantID == "" || evidenceDigest == "" || actionType == "" {
		return 0
	}
	audits, err := listRemediationAuditsFn(ctx, limit)
	if err != nil {
		return 0
	}
	sinceUnix := since.Unix()
	count := 0
	for _, a := range audits {
		if a.InvariantID != invariantID || a.EvidenceDigest != evidenceDigest || a.ActionType != actionType {
			continue
		}
		if a.Timestamp < sinceUnix || a.DryRun {
			continue
		}
		if !a.Executed && a.Reason != "" {
			count++
		}
	}
	return count
}

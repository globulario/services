// incident_projection.go stores operational incidents to ai-memory when
// workflow runs complete as FAILED or BLOCKED (AL-1).
//
// Incidents are the learning dataset for the AI advisory layer. They use
// the existing ai-memory service — no new tables or services needed.
//
// Safety: incidents are read-only from the workflow engine's perspective.
// They inform doctor findings but never bypass verification or approval.
//
// See docs/architecture/learn-by-error-implementation.md.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// incidentEligibleStatus pins the incident policy on the authoritative enum.
//
// ONLY a terminal failure produces a failure incident. DEFERRED and BLOCKED are
// non-terminal — the run is waiting, not broken — and SUCCEEDED is not a
// failure. Any future enum value is excluded by default: a status nobody has
// classified must not silently become a failure incident.
func incidentEligibleStatus(status workflowpb.RunStatus) bool {
	return status == workflowpb.RunStatus_RUN_STATUS_FAILED
}

// incidentStatusLabel renders the authoritative enum for incident text without
// routing through the lossy legacy mapper.
func incidentStatusLabel(status workflowpb.RunStatus) string {
	return strings.TrimPrefix(status.String(), "RUN_STATUS_")
}

// ── Incident creation ────────────────────────────────────────────────────────

// projectIncident stores an operational incident to ai-memory when a
// workflow run finishes as FAILED or BLOCKED. Called from ExecuteWorkflow
// after FinishRun.
//
// Deduplication: incidents are keyed by (cluster_id, workflow_name, step_id,
// error_signature, package_name). Duplicate incidents within a short window
// are not stored.
// projectIncident takes the AUTHORITATIVE RunStatus enum, never the legacy
// response string.
//
// resp.Status is built by legacyRunStatusString, which collapses the whole enum
// onto "SUCCEEDED" or "FAILED" for ExecuteWorkflowResponse's documented
// contract. Classifying incidents from that string made every DEFERRED and
// BLOCKED run arrive labelled "FAILED", so a run that was waiting on a
// prerequisite was recorded as a terminal failure in the AI learning dataset.
//
// The old guard `resp.Status != "FAILED" && resp.Status != "BLOCKED"` shows the
// intent was already to treat BLOCKED separately — but legacyRunStatusString can
// never emit "BLOCKED", so that branch was unreachable and blocked runs fell
// through the FAILED path. The lossy mapping stays where it belongs: the
// external API boundary.
func (srv *server) projectIncident(ctx context.Context, req *workflowpb.ExecuteWorkflowRequest, resp *workflowpb.ExecuteWorkflowResponse, status workflowpb.RunStatus) {
	if resp == nil {
		return
	}
	if !incidentEligibleStatus(status) {
		return
	}
	// Must have a clear error to store — avoid noisy empty incidents.
	if resp.Error == "" && status == workflowpb.RunStatus_RUN_STATUS_FAILED {
		return
	}

	errSig := normalizeErrorSignature(resp.Error)
	dedupeKey := fmt.Sprintf("%s/%s/%s", req.ClusterId, req.WorkflowName, errSig)

	// Check dedupe: skip if we stored this exact incident recently.
	if srv.recentIncidentSeen(dedupeKey) {
		return
	}

	statusLabel := incidentStatusLabel(status)
	title := fmt.Sprintf("%s %s: %s", req.WorkflowName, statusLabel, truncate(resp.Error, 80))

	tags := []string{
		req.WorkflowName,
		strings.ToLower(statusLabel),
	}

	metadata := map[string]string{
		"cluster_id":      req.ClusterId,
		"workflow_name":   req.WorkflowName,
		"run_id":          resp.RunId,
		"run_status":      statusLabel,
		"error_signature": errSig,
		"correlation_id":  req.CorrelationId,
		"created_at":      fmt.Sprintf("%d", time.Now().Unix()),
	}

	// Store via the ai-memory MCP tool pattern — direct gRPC to ai-memory
	// if available, otherwise log and skip (fire-and-forget).
	srv.storeIncidentToMemory(ctx, title, strings.Join(tags, ","), resp.Error, metadata)
	srv.markRecentIncident(dedupeKey)
}

// ── Error normalization ──────────────────────────────────────────────────────

var (
	// Patterns to strip from error messages for normalization.
	uuidPattern  = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	ipPattern    = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?`)
	timestampPat = regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}[T ]\d{2}:\d{2}:\d{2}`)
	unixMsPat    = regexp.MustCompile(`\b1[67]\d{11}\b`) // unix millis (2020-2030)
)

// normalizeErrorSignature strips volatile parts (IPs, UUIDs, timestamps)
// from an error message to produce a stable signature for deduplication
// and similarity matching.
func normalizeErrorSignature(errMsg string) string {
	s := errMsg
	s = uuidPattern.ReplaceAllString(s, "<UUID>")
	s = ipPattern.ReplaceAllString(s, "<IP>")
	s = timestampPat.ReplaceAllString(s, "<TS>")
	s = unixMsPat.ReplaceAllString(s, "<UNIX>")
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200]
	}
	// Hash for compact dedup key.
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// ── Deduplication ────────────────────────────────────────────────────────────

const incidentDedupeWindow = 5 * time.Minute

// recentIncidentSeen checks if we stored this exact incident recently.
func (srv *server) recentIncidentSeen(key string) bool {
	srv.incidentDedupeMu.RLock()
	defer srv.incidentDedupeMu.RUnlock()
	if t, ok := srv.incidentDedupeMap[key]; ok {
		return time.Since(t) < incidentDedupeWindow
	}
	return false
}

func (srv *server) markRecentIncident(key string) {
	srv.incidentDedupeMu.Lock()
	defer srv.incidentDedupeMu.Unlock()
	if srv.incidentDedupeMap == nil {
		srv.incidentDedupeMap = make(map[string]time.Time)
	}
	srv.incidentDedupeMap[key] = time.Now()
	// Lazy cleanup: remove old entries.
	for k, t := range srv.incidentDedupeMap {
		if time.Since(t) > incidentDedupeWindow*2 {
			delete(srv.incidentDedupeMap, k)
		}
	}
}

// ── ai-memory storage ────────────────────────────────────────────────────────

func (srv *server) storeIncidentToMemory(ctx context.Context, title, tags, content string, metadata map[string]string) {
	if srv.aiMemoryClient == nil {
		slog.Debug("incident projection: ai-memory not configured, skipping", "title", title)
		return
	}

	storeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Add "incident" tag so we can filter incidents from other debug memories.
	tagList := append(strings.Split(tags, ","), "incident")

	_, err := srv.aiMemoryClient.Store(storeCtx, &ai_memorypb.StoreRqst{
		Memory: &ai_memorypb.Memory{
			Project:  "globular-services",
			Type:     ai_memorypb.MemoryType_DEBUG,
			Title:    title,
			Content:  content,
			Tags:     tagList,
			Metadata: metadata,
		},
	})
	if err != nil {
		slog.Warn("incident projection: store failed", "title", title, "err", err)
		return
	}
	slog.Info("incident projection: stored", "title", title, "tags", tags)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

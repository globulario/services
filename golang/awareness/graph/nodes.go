package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Node types supported in V1.
const (
	NodeTypeSourceFile          = "source_file"
	NodeTypeSymbol              = "symbol"
	NodeTypeGoPackage           = "go_package"
	NodeTypeProtoService        = "proto_service"
	NodeTypeProtoMessage        = "proto_message"
	NodeTypeRPCMethod           = "rpc_method"
	NodeTypeGlobularService     = "globular_service"
	NodeTypePackage             = "package"
	NodeTypeWorkflow            = "workflow"
	NodeTypeWorkflowStep        = "workflow_step"
	NodeTypeSystemdUnit         = "systemd_unit"
	NodeTypeEtcdKey             = "etcd_key"
	NodeTypeScyllaTable         = "scylla_table"
	NodeTypeMinioBucket         = "minio_bucket"
	NodeTypeEventType           = "event_type"
	NodeTypeDoctorFinding       = "doctor_finding"
	NodeTypeInvariant           = "invariant"
	NodeTypeFailureMode         = "failure_mode"
	NodeTypeForbiddenFix        = "forbidden_fix"
	// NodeTypeIncidentPattern indexes hand-authored entries from
	// docs/awareness/incident_patterns.yaml. Distinct from NodeTypeIncident
	// (the runtime-recorded incident store) — incident_pattern is the
	// stay-fixed pattern an incident exemplifies, written into the YAML at
	// build time. The graph node carries only the compact identity fields
	// (id, summary, severity, failure_mode reference); the long narrative
	// (root_cause, lesson, edit_shapes) stays in the YAML to keep graph
	// JSON small. See knowledge.IncidentPattern for the full struct.
	NodeTypeIncidentPattern = "incident_pattern"
	NodeTypeTest                = "test"
	NodeTypeRuntimeState        = "runtime_state"
	NodeTypeRemediationWorkflow = "remediation_workflow"
	NodeTypeDependencyPhase     = "dependency_phase"

	// Learning node types (Task 3).
	NodeTypeIncident          = "incident"
	NodeTypeIncidentBundle    = "incident_bundle"
	NodeTypeAwarenessProposal = "awareness_proposal"
	NodeTypeProposalPatch     = "proposal_patch"
	NodeTypeContextAlias      = "context_alias"
	NodeTypeLearningRule      = "learning_rule"
	NodeTypeEvidence          = "evidence"
	NodeTypeManualRepair      = "manual_repair"

	// Experience ledger node types.
	NodeTypeExperience     = "experience"
	NodeTypeGoalPattern    = "goal_pattern"
	NodeTypeStrategy       = "strategy"
	NodeTypeAttempt        = "attempt"
	NodeTypeObservation    = "observation"
	NodeTypeLesson         = "lesson"
	NodeTypeNextTimeHint   = "next_time_hint"
	NodeTypeEvidenceBundle = "evidence_bundle"
	NodeTypeScorecard      = "scorecard"

	// Fix ledger node types (Task 4).
	NodeTypeFixCase              = "fix_case"
	NodeTypeGuardrail            = "guardrail"
	NodeTypeImplementationStatus = "implementation_status"
	NodeTypeRemainingGap         = "remaining_gap"
	NodeTypeTestObligation       = "test_obligation"
	NodeTypeRegressionRisk       = "regression_risk"
	NodeTypeCodeFix              = "code_fix"
	NodeTypeAuditItem            = "audit_item"

	// Protocol annotation node types (Task 8).
	NodeTypeHashSchema       = "hash_schema"
	NodeTypeStateTransition  = "state_transition"
	NodeTypeRiskSurface      = "risk_surface"
	NodeTypeProtocolContract = "protocol_contract"

	// Pattern node type (Task 15).
	NodeTypePattern = "pattern"

	// Design pattern layer node types.
	NodeTypeDesignPattern = "design_pattern"
	NodeTypeAntiPattern   = "anti_pattern"
	NodeTypeCodeSmell     = "code_smell"

	// Design / documentation node types (Task 12).
	NodeTypeArchitectureDecision = "architecture_decision"
	NodeTypeDesignRule           = "design_rule"
	NodeTypeRationale            = "rationale"
	NodeTypeTradeoff             = "tradeoff"
	NodeTypeOperationalPrinciple = "operational_principle"
	NodeTypeRunbook              = "runbook"
	NodeTypeDebugPlaybook        = "debug_playbook"
	NodeTypeDocumentationSection = "documentation_section"
	NodeTypeErrorReport          = "error_report"
	NodeTypeIncidentReport       = "incident_report"
	NodeTypeCommitFix            = "commit_fix"

	// Service design graph node types (Phase 2-8).
	NodeTypeAuthzAnnotation = "authz_annotation" // proto RPC authz option
	NodeTypeStreamingMode   = "streaming_mode"   // client_streaming / server_streaming / bidirectional

	// Runtime bridge node types (Task 6).
	NodeTypeRuntimeSnapshot      = "runtime_snapshot"
	NodeTypeRuntimeServiceStatus = "runtime_service_status"
	NodeTypeWorkflowReceipt      = "workflow_receipt"
	NodeTypeStateDelta           = "state_delta"
	NodeTypeRepositoryStatus     = "repository_status"
	NodeTypeObjectstoreStatus    = "objectstore_status"
	NodeTypeXDSStatus            = "xds_status"
	NodeTypeSystemdStatus        = "systemd_status"
	NodeTypeDoctorEvidence       = "doctor_evidence"

	// Live etcd cluster state node types.
	NodeTypeEtcdSnapshot          = "etcd_snapshot"
	NodeTypeDesiredService        = "desired_service"
	NodeTypeDesiredInfrastructure = "desired_infrastructure"
	NodeTypeServiceRelease        = "service_release"
	NodeTypeInfrastructureRelease = "infrastructure_release"
	NodeTypeNodeHeartbeat         = "node_heartbeat"
	NodeTypeNodeConvergenceState  = "node_convergence_state"
	NodeTypeNodeInstalledPackage  = "node_installed_package"
	NodeTypeObjectstoreDesired    = "objectstore_desired_state"
	NodeTypeServiceRuntimeConfig  = "service_runtime_config"
	NodeTypeClusterSystemConfig   = "cluster_system_config"

	// Convergence delta node types.
	NodeTypeConvergenceRecord    = "convergence_record"
	NodeTypeDesiredStateRecord   = "desired_state_record"
	NodeTypeInstalledStateRecord = "installed_state_record"
	NodeTypeRuntimeStateRecord   = "runtime_state_record"
	NodeTypeDriftRecord          = "drift_record"
	NodeTypeReleaseAction        = "release_action"
	NodeTypeVerificationRecord   = "verification_record"

	// Metrics node types.
	NodeTypeMetricQuery       = "metric_query"
	NodeTypeMetricThreshold   = "metric_threshold"
	NodeTypeMetricWarningRule = "metric_warning_rule"
	NodeTypeMetricSample      = "metric_sample"
	NodeTypeMetricWarning     = "metric_warning"

	// PKI / certificate node types.
	NodeTypeCertificate          = "certificate"
	NodeTypeCertificateAuthority = "certificate_authority"
	NodeTypeCertSAN              = "certificate_san"
	NodeTypeCertExpiryWarning    = "certificate_expiry_warning"

	// RBAC policy node types.
	NodeTypeRBACPolicyFile  = "rbac_policy_file"
	NodeTypeRBACRole        = "rbac_role"
	NodeTypeRBACPermission  = "rbac_permission"
	NodeTypeRBACSubject     = "rbac_subject"
	NodeTypeRBACBinding     = "rbac_binding"
	NodeTypeServiceIdentity = "service_identity"

	// Workflow execution node types.
	NodeTypeWorkflowRun                = "workflow_run"
	NodeTypeWorkflowStepRun            = "workflow_step_run"
	NodeTypeWorkflowBlockedReason      = "workflow_blocked_reason"
	NodeTypeWorkflowRetryRecord        = "workflow_retry_record"
	NodeTypeWorkflowError              = "workflow_error"
	NodeTypeWorkflowStepEffect         = "workflow_step_effect"
	NodeTypeWorkflowVerificationRecord = "workflow_verification_record"
	NodeTypeWorkflowIntegrityFinding   = "workflow_integrity_finding"

	// DNS / network node types.
	NodeTypeDNSZone         = "dns_zone"
	NodeTypeDNSRecord       = "dns_record"
	NodeTypeServiceEndpoint = "service_endpoint"
	NodeTypeDomainSpec      = "domain_spec"
)

// Node is a vertex in the awareness graph.
type Node struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Path      string         `json:"path,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt int64          `json:"created_at,omitempty"`
	UpdatedAt int64          `json:"updated_at,omitempty"`
}

// AddNode upserts a node by ID. Existing nodes are updated in-place.
//
// AddNode is destructive: the upsert replaces every field including metadata.
// Callers that only want to STUB a node must use EnsureNode instead.
func (g *Graph) AddNode(ctx context.Context, n Node) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("AddNode %s: graph is read-only", n.ID)
	}
	now := time.Now().Unix()
	if n.CreatedAt == 0 {
		n.CreatedAt = now
	}
	n.UpdatedAt = now

	g.mu.Lock()
	n2 := n
	g.indexNode(&n2)
	g.mu.Unlock()
	return nil
}

// EnsureNode inserts a node only if no node with the same ID exists.
// If the node already exists, EnsureNode is a no-op — the existing node's
// metadata, summary, type, and name are preserved.
func (g *Graph) EnsureNode(ctx context.Context, n Node) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("EnsureNode %s: graph is read-only", n.ID)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.nodes[n.ID]; exists {
		return nil // no-op: preserve existing node
	}
	now := time.Now().Unix()
	if n.CreatedAt == 0 {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	n2 := n
	g.indexNode(&n2)
	return nil
}

// DeleteNode removes a node and all its edges from the graph.
func (g *Graph) DeleteNode(ctx context.Context, id string) error {
	if g.readOnly || g.staticReadOnly {
		return fmt.Errorf("DeleteNode %s: graph is read-only", id)
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	n, ok := g.nodes[id]
	if !ok {
		return nil
	}
	g.removeNodeFromIndexes(n)
	delete(g.nodes, id)

	// Remove all edges involving this node.
	var kept []*Edge
	for _, e := range g.edges {
		if e.Src == id || e.Dst == id {
			k := edgeKey{Src: e.Src, Kind: e.Kind, Dst: e.Dst, Phase: e.Phase}
			delete(g.edgeKeys, k)
			// Remove from indexes.
			g.bySrc[e.Src] = removeEdgeFromSlice(g.bySrc[e.Src], e)
			g.byDst[e.Dst] = removeEdgeFromSlice(g.byDst[e.Dst], e)
			g.byKind[e.Kind] = removeEdgeFromSlice(g.byKind[e.Kind], e)
			g.byClass[e.Class] = removeEdgeFromSlice(g.byClass[e.Class], e)
		} else {
			kept = append(kept, e)
		}
	}
	// Rebuild edgeKeys indexes for kept edges.
	g.edges = kept
	g.edgeKeys = make(map[edgeKey]int)
	for i, e := range g.edges {
		g.edgeKeys[edgeKey{Src: e.Src, Kind: e.Kind, Dst: e.Dst, Phase: e.Phase}] = i
	}
	return nil
}

// PruneStaleSourceFileNodes removes source_file nodes whose paths no longer
// exist on disk. srcDir is the repo root used to resolve repo-relative paths.
func (g *Graph) PruneStaleSourceFileNodes(ctx context.Context, srcDir string) (int, error) {
	nodes, err := g.FindNodesByType(ctx, NodeTypeSourceFile)
	if err != nil {
		return 0, fmt.Errorf("PruneStale: list nodes: %w", err)
	}
	pruned := 0
	for _, n := range nodes {
		if n.Path == "" {
			continue
		}
		absPath := filepath.Join(srcDir, n.Path)
		if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
			if err := g.DeleteNode(ctx, n.ID); err != nil {
				return pruned, err
			}
			pruned++
		}
	}
	return pruned, nil
}

// copyMetadata deep-copies a metadata map.
func copyMetadata(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// metaToJSON serialises metadata to a JSON string for compatibility with
// callers that expect the old SQL backend's string representation.
func metaToJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}

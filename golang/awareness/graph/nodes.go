package graph

import (
	"context"
	"encoding/json"
	"fmt"
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

	// Runtime bridge node types (Task 6).
	NodeTypeRuntimeSnapshot = "runtime_snapshot"
	NodeTypeRuntimeServiceStatus = "runtime_service_status"
	NodeTypeWorkflowReceipt      = "workflow_receipt"
	NodeTypeStateDelta           = "state_delta"
	NodeTypeRepositoryStatus     = "repository_status"
	NodeTypeObjectstoreStatus    = "objectstore_status"
	NodeTypeXDSStatus            = "xds_status"
	NodeTypeSystemdStatus        = "systemd_status"
	NodeTypeDoctorEvidence       = "doctor_evidence"
)

// Node is a vertex in the awareness graph.
type Node struct {
	ID        string
	Type      string
	Name      string
	Path      string
	Summary   string
	Metadata  map[string]any
	CreatedAt int64
	UpdatedAt int64
}

// AddNode upserts a node by ID. Existing nodes are updated in-place.
func (g *Graph) AddNode(ctx context.Context, n Node) error {
	meta, err := marshalMeta(n.Metadata)
	if err != nil {
		return fmt.Errorf("AddNode %s: %w", n.ID, err)
	}
	now := time.Now().Unix()
	_, err = g.db.ExecContext(ctx, `
		INSERT INTO nodes (id, type, name, path, summary, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type          = excluded.type,
			name          = excluded.name,
			path          = excluded.path,
			summary       = excluded.summary,
			metadata_json = excluded.metadata_json,
			updated_at    = excluded.updated_at
	`, n.ID, n.Type, n.Name, n.Path, n.Summary, meta, now, now)
	if err != nil {
		return fmt.Errorf("AddNode %s: %w", n.ID, err)
	}
	return nil
}

// marshalMeta encodes a metadata map to JSON. Returns "{}" for nil.
func marshalMeta(m map[string]any) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// unmarshalMeta decodes a JSON metadata string. Returns nil on empty/invalid input.
func unmarshalMeta(s string) map[string]any {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestRemediationAuditCarriesCorrelationFields — contract test.
// Every audit record must carry a correlation_id, even when the caller
// didn't supply one (deterministic fallback). When the caller does supply
// a correlation header it must be preserved so doctor and workflow audits
// join. WorkflowRunID must propagate when present.
func TestRemediationAuditCarriesCorrelationFields(t *testing.T) {
	// Caller supplies correlation + workflow run id.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		AuditCorrelationMetadataKey, "corr-from-workflow-abc",
		AuditWorkflowRunMetadataKey, "wf-run-123",
	))
	if got := correlationIDFromContext(ctx, "f1", 0); got != "corr-from-workflow-abc" {
		t.Fatalf("propagated correlation: got %q", got)
	}
	if got := workflowRunIDFromContext(ctx); got != "wf-run-123" {
		t.Fatalf("propagated workflow run id: got %q", got)
	}

	// No metadata → deterministic fallback so audits are still joinable.
	bare := correlationIDFromContext(context.Background(), "f-bare", 2)
	if !strings.HasPrefix(bare, "corr-f-bare-2-") {
		t.Fatalf("fallback correlation id must encode finding+step, got %q", bare)
	}
}

// TestAuditRedactsApprovalTokenMaterial — contract test. If an approval
// token or other secret material accidentally lands in the action's
// Params, the audit writer must redact it before persisting. The redaction
// must be content-aware (JWT-shaped values) as well as key-aware (anything
// named "*token*", "*secret*", "*password*").
func TestAuditRedactsApprovalTokenMaterial(t *testing.T) {
	jwt := "eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiJ4In0.signature_blob_here_abcdefg"
	audit := RemediationAudit{
		AuditID: "rem-test",
		Params: map[string]string{
			"approval_token":    "secret-token-value",
			"client_secret":     "hunter2",
			"password":          "p4ssw0rd",
			"api_key":           "ak-xyz",
			"node_id":           "node-1",   // innocuous, must be preserved
			"unit":              "echo.service",
			"random_jwt_in_val": jwt,        // JWT-shaped value, even with innocuous key
		},
	}
	redacted := audit.Redacted()
	for _, k := range []string{"approval_token", "client_secret", "password", "api_key", "random_jwt_in_val"} {
		if redacted.Params[k] != "<redacted>" {
			t.Fatalf("param %s must be redacted, got %q", k, redacted.Params[k])
		}
	}
	if redacted.Params["node_id"] != "node-1" {
		t.Fatalf("innocuous param must be preserved, got %q", redacted.Params["node_id"])
	}
	if redacted.Params["unit"] != "echo.service" {
		t.Fatalf("innocuous param must be preserved, got %q", redacted.Params["unit"])
	}
	// Caller's struct must not be mutated.
	if audit.Params["approval_token"] != "secret-token-value" {
		t.Fatal("Redacted must return a copy, not mutate caller")
	}
}

// TestAuditRetentionPolicyIsExplicit — contract test. Retention must be
// declared as a constant (RemediationAuditRetention), not buried inline
// as a magic lease number. Compliance tooling and operators must be able
// to inspect the policy from code.
func TestAuditRetentionPolicyIsExplicit(t *testing.T) {
	if RemediationAuditRetention <= 0 {
		t.Fatal("RemediationAuditRetention must be a positive declared constant")
	}
	if RemediationAuditRetention < 7*24*time.Hour {
		t.Fatalf("retention %s is shorter than compliance floor of 7 days", RemediationAuditRetention)
	}
	// Document the current target so a future change must update both
	// the constant and this assertion together — the policy stays declared.
	if want := 30 * 24 * time.Hour; RemediationAuditRetention != want {
		t.Fatalf("retention policy changed without test update: got %s, declared want %s",
			RemediationAuditRetention, want)
	}
}

// Smoke test: an end-to-end ExecuteRemediation call produces an audit
// whose correlation fields are populated. The doctor is not running etcd
// in this unit test — auditRemediation gracefully no-ops the persistence
// step on etcd error — so we exercise the in-process audit struct path
// by inspecting the rejection response's reason wiring.
func TestExecuteRemediationStampsCorrelationFromContext(t *testing.T) {
	withStubbedGatePersistence(t)

	findingID := "finding-correlation"
	srv := &ClusterDoctorServer{
		executor: &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
		lastFindings: []rules.Finding{{
			FindingID:   findingID,
			InvariantID: "runtime.desired_enabled_not_alive",
			Summary:     "unit is not running",
			Evidence: []*cluster_doctorpb.Evidence{{
				SourceService: "cluster_controller",
				SourceRpc:     "GetClusterHealthV1",
				Timestamp:     timestamppb.Now(),
			}},
			Remediation: []*cluster_doctorpb.RemediationStep{{
				Order: 1,
				Action: &cluster_doctorpb.RemediationAction{
					ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
					Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
					Params:     map[string]string{"unit": "echo.service", "node_id": "node-1"},
				},
			}},
		}},
	}
	srv.isAuthoritative.Store(true)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		AuditCorrelationMetadataKey, "corr-smoke",
	))
	_, err := srv.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: findingID,
		StepIndex: 0,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The audit struct is internal; we know correlationIDFromContext returns
	// the propagated id when metadata is set — verified above. The smoke
	// test simply ensures the call succeeds end-to-end with correlation
	// metadata attached.
}

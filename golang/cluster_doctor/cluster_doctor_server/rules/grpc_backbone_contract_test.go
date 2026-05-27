package rules

import (
	"errors"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestGrpcBackboneContract_NoDataErrorsNoFindings(t *testing.T) {
	inv := grpcBackboneContract{}
	got := inv.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %d", len(got))
	}
}

func TestGrpcBackboneContract_ClusterIDViolation(t *testing.T) {
	inv := grpcBackboneContract{}
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{
				Service: "workflow",
				RPC:     "GetRun",
				Err:     errors.New("rpc error: code = PermissionDenied desc = cluster_id required after cluster initialization"),
			},
		},
	}
	got := inv.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("severity=%v want ERROR", got[0].Severity)
	}
	if got[0].InvariantID != "grpc.backbone.contract" {
		t.Fatalf("unexpected invariant: %s", got[0].InvariantID)
	}
}

func TestGrpcBackboneContract_CallDepthViolation(t *testing.T) {
	inv := grpcBackboneContract{}
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{
				Service: "repository",
				RPC:     "Publish",
				Err:     errors.New("rpc error: code = ResourceExhausted desc = call depth 10 exceeds maximum 10 — probable circular service call"),
			},
		},
	}
	got := inv.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("severity=%v want ERROR", got[0].Severity)
	}
}

func TestGrpcBackboneContract_PublicProbeDenied(t *testing.T) {
	inv := grpcBackboneContract{}
	snap := &collector.Snapshot{
		DataErrors: []collector.DataError{
			{
				Service: "dns",
				RPC:     "/grpc.health.v1.Health/Check",
				Err:     errors.New("rpc error: code = Unauthenticated desc = authentication required"),
			},
		},
	}
	got := inv.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("severity=%v want WARN", got[0].Severity)
	}
}


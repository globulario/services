package main

import (
	"os"
	"path/filepath"
	"testing"

	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

func TestLoadOperationRequest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "op.json")
	const body = `{
		"id":"op-1","actor":"ACTOR_AGENT","action":"set_desired_version",
		"target":{"resourceId":"echo","owner":"cluster_controller"},
		"execution":{"mode":"APPLY","approvedPath":"OWNER_RPC"}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	req, err := loadOperationRequest(path)
	if err != nil {
		t.Fatalf("loadOperationRequest: %v", err)
	}
	if req.GetId() != "op-1" || req.GetActor() != pb.ActorKind_ACTOR_AGENT {
		t.Errorf("bad parse: id=%q actor=%v", req.GetId(), req.GetActor())
	}
	if req.GetTarget().GetOwner() != "cluster_controller" {
		t.Errorf("owner = %q", req.GetTarget().GetOwner())
	}
}

func TestLoadOperationRequest_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := loadOperationRequest(path); err == nil {
		t.Fatal("expected parse error for malformed JSON")
	}
}

func TestLedgerEntryFromRequest(t *testing.T) {
	req := &pb.OperationRequest{
		Id:     "op-9",
		Actor:  pb.ActorKind_ACTOR_MCP,
		Action: "set_desired_version",
		Target: &pb.OperationTarget{ResourceId: "echo", Owner: "cluster_controller"},
		Evidence: &pb.OperationEvidence{
			BeforeSnapshot:   "sha256:zz",
			AwgInvariants:    []string{"desired.keyed_by_kind_and_name"},
			BehavioralRules:  []string{"forbidden.cluster.raw_owner_owned_state_write"},
			RelatedIncidents: []string{"a399ebea"},
		},
	}
	e := ledgerEntryFromRequest(req, pb.OperationResult_ALLOWED)
	if e.GetOperationId() != "op-9" || e.GetResult() != pb.OperationResult_ALLOWED {
		t.Fatalf("bad entry: id=%q result=%v", e.GetOperationId(), e.GetResult())
	}
	if e.GetActor() != "ACTOR_MCP" || e.GetTargetOwner() != "cluster_controller" || e.GetTargetResource() != "echo" {
		t.Errorf("bad projection: %+v", e)
	}
	if e.GetBeforeStateHash() != "sha256:zz" || len(e.GetAwgInvariants()) != 1 || len(e.GetRelatedAiMemoryEvents()) != 1 {
		t.Errorf("evidence not carried into ledger: %+v", e)
	}
	if e.GetTimestamp() == "" {
		t.Error("timestamp must be stamped")
	}
}

func TestResultByName(t *testing.T) {
	if resultByName["refused"] != pb.OperationResult_REFUSED || resultByName["completed"] != pb.OperationResult_COMPLETED {
		t.Error("result name mapping wrong")
	}
	if _, ok := resultByName["bogus"]; ok {
		t.Error("unknown result name must not map")
	}
}

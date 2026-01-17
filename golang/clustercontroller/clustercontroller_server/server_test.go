package main

import (
	"context"
	"strings"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

func TestRestartUnitsForSpecChanges(t *testing.T) {
	httpSpec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "http",
	}
	httpsSpec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
	}

	httpUnits := restartUnitsForSpec(httpSpec)
	if len(httpUnits) == 0 {
		t.Fatalf("expected restart units for http spec")
	}
	if !containsUnit(httpUnits, "globular-dns.service") {
		t.Fatalf("http restart units missing globular-dns.service")
	}
	httpsUnits := restartUnitsForSpec(httpsSpec)
	if len(httpsUnits) <= len(httpUnits) {
		t.Fatalf("expected https spec to include additional units")
	}
	found := false
	for _, unit := range httpsUnits {
		if unit == "globular-storage.service" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("https restart units missing globular-storage.service")
	}
}

func containsUnit(units []string, target string) bool {
	for _, u := range units {
		if u == target {
			return true
		}
	}
	return false
}

func TestCompleteOperationMarksDone(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	opID := "op-complete"
	nodeID := "node-1"
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, clustercontrollerpb.OperationPhase_OP_RUNNING, "running", 10, false, ""))

	_, err := srv.CompleteOperation(context.Background(), &clustercontrollerpb.CompleteOperationRequest{
		OperationId: opID,
		NodeId:      nodeID,
		Success:     true,
		Message:     "done",
	})
	if err != nil {
		t.Fatalf("CompleteOperation error: %v", err)
	}
	op := srv.operations[opID]
	if op == nil || op.last == nil {
		t.Fatalf("operation state missing")
	}
	if !op.last.GetDone() {
		t.Fatalf("expected done=true, got %+v", op.last)
	}
	if op.last.GetPhase() != clustercontrollerpb.OperationPhase_OP_SUCCEEDED {
		t.Fatalf("expected succeeded phase, got %s", op.last.GetPhase())
	}
}

func TestCleanupTimedOutOperationsFails(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)
	opID := "op-timeout"
	srv.operations[opID] = &operationState{
		created: time.Now().Add(-(operationTimeout + time.Minute)),
		nodeID:  "node-x",
	}
	srv.cleanupTimedOutOperations()
	op := srv.operations[opID]
	if op == nil || op.last == nil {
		t.Fatalf("expected operation event after timeout")
	}
	if !op.last.GetDone() {
		t.Fatalf("expected done true after timeout, got %+v", op.last)
	}
	if op.last.GetPhase() != clustercontrollerpb.OperationPhase_OP_FAILED {
		t.Fatalf("expected failed phase, got %s", op.last.GetPhase())
	}
	if !strings.Contains(op.last.GetMessage(), "timed out") {
		t.Fatalf("expected timeout message, got %q", op.last.GetMessage())
	}
}

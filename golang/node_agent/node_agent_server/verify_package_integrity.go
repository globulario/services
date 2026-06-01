package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// VerifyPackageIntegrity is the gRPC entrypoint for the
// package.verify_integrity action. It wraps the existing action registry
// invocation so operators, doctor rules, and admin UIs can run the
// invariant checks without embedding the workflow engine.
//
// The method is read-only: it inspects etcd installed_state, the local
// artifact cache, and the repository manifest. No files are written.
// The JSON report is returned verbatim in report_json.
func (srv *NodeAgentServer) VerifyPackageIntegrity(ctx context.Context, req *node_agentpb.VerifyPackageIntegrityRequest) (*node_agentpb.VerifyPackageIntegrityResponse, error) {
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		nodeID = srv.nodeID
	}

	handler := actions.Get("package.verify_integrity")
	if handler == nil {
		return &node_agentpb.VerifyPackageIntegrityResponse{
			Ok:          false,
			ErrorDetail: "action package.verify_integrity is not registered",
		}, nil
	}

	args, err := structpb.NewStruct(map[string]any{
		"package_name":    req.GetPackageName(),
		"kind":            req.GetKind(),
		"repository_addr": req.GetRepositoryAddr(),
		"node_id":         nodeID,
	})
	if err != nil {
		return &node_agentpb.VerifyPackageIntegrityResponse{
			Ok:          false,
			ErrorDetail: fmt.Sprintf("build args: %v", err),
		}, nil
	}

	result, err := handler.Apply(ctx, args)
	if err != nil {
		return &node_agentpb.VerifyPackageIntegrityResponse{
			Ok:          false,
			ErrorDetail: err.Error(),
		}, nil
	}

	// Parse the JSON report just enough to populate the convenience
	// summary fields. Callers that need the full structure re-parse
	// report_json themselves.
	var parsed struct {
		Checked    int            `json:"checked"`
		Findings   []any          `json:"findings"`
		Invariants map[string]int `json:"invariants"`
	}
	_ = json.Unmarshal([]byte(result), &parsed)

	total := 0
	for _, n := range parsed.Invariants {
		total += n
	}
	if total == 0 {
		total = len(parsed.Findings)
	}

	return &node_agentpb.VerifyPackageIntegrityResponse{
		Ok:           true,
		ReportJson:   result,
		CheckedCount: int32(parsed.Checked),
		FindingCount: int32(total),
	}, nil
}

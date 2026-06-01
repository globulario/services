// @awareness namespace=globular.platform
// @awareness component=platform_node_agent
// @awareness file_role=actor_service_lifecycle_bridge
// @awareness risk=medium
package main

// actor_service.go — node-agent implementation of WorkflowActorService.
//
// The workflow service's executor dispatches steps to actor services via
// gRPC ExecuteAction calls. For workflows whose steps target actor=node_agent
// (e.g. package.rollback's install_target_package), this server is the
// receiving end. Without this implementation those workflows block forever
// at their first node-agent step.
//
// Actions handled:
//   package.apply               → ApplyPackageRelease (with rollback_mode
//                                  inferred from inputs)
//   service.drain               → systemctl stop unit
//   service.start               → systemctl start unit
//   service.health_probe        → wait-active probe
//   services.verify_integrity   → VerifyPackageIntegrity wrapper
//   package.config.classify     → no-op success (the apply path runs
//                                  applyConfigPolicyPreInstall internally)
//   package.config.apply_policy → no-op success (receipts are emitted by
//                                  the apply path's post-success hook)
//   package.rollback.forward_recover → re-applies the previous revision
//                                  (best-effort; logs and returns ok=false
//                                  when the previous revision is unknown)
//
// Unknown actions return ok=false with a clear message — never silent.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// NodeAgentActorServer implements workflowpb.WorkflowActorServiceServer.
type NodeAgentActorServer struct {
	workflowpb.UnimplementedWorkflowActorServiceServer
	srv *NodeAgentServer
}

// NewNodeAgentActorServer returns an actor server bound to the given
// node-agent. Register with grpc.Server via
// workflowpb.RegisterWorkflowActorServiceServer.
func NewNodeAgentActorServer(srv *NodeAgentServer) *NodeAgentActorServer {
	return &NodeAgentActorServer{srv: srv}
}

func (a *NodeAgentActorServer) ExecuteAction(ctx context.Context, req *workflowpb.ExecuteActionRequest) (*workflowpb.ExecuteActionResponse, error) {
	if req == nil || req.GetAction() == "" {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: "action is required"}, nil
	}
	with, inputs := decodeActionInputs(req)

	log.Printf("actor: %s/%s run=%s step=%s", req.GetActor(), req.GetAction(),
		req.GetRunId(), req.GetStepId())

	switch req.GetAction() {
	case "package.apply":
		return a.handlePackageApply(ctx, req, with, inputs)
	case "service.drain":
		return a.handleServiceDrain(ctx, req, with, inputs)
	case "service.start":
		return a.handleServiceStart(ctx, req, with, inputs)
	case "service.health_probe":
		return a.handleServiceHealthProbe(ctx, req, with, inputs)
	case "services.verify_integrity":
		return a.handleVerifyIntegrity(ctx, req, with, inputs)
	case "package.config.classify",
		"package.config.apply_policy":
		// The apply path performs config classification + receipt emission
		// internally. Returning success here lets the workflow advance
		// without duplicating work.
		return &workflowpb.ExecuteActionResponse{
			Ok:      true,
			Message: req.GetAction() + ": handled by package.apply",
		}, nil
	case "package.rollback.forward_recover":
		return a.handleForwardRecover(ctx, req, with, inputs)
	}

	return &workflowpb.ExecuteActionResponse{
		Ok:      false,
		Message: fmt.Sprintf("node-agent: unknown action %q", req.GetAction()),
	}, nil
}

// ── Action handlers ───────────────────────────────────────────────────────

// handlePackageApply maps a workflow step to ApplyPackageRelease. The step's
// `with:` block is the source of truth for identity (publisher, name,
// version, build_id, etc.); workflow-level inputs are a fallback.
//
// rollback_mode is inferred from with.rollback_mode (set by the
// package.rollback YAML's install_target_package step). preserve_configs
// defaults to true when rollback_mode is set.
func (a *NodeAgentActorServer) handlePackageApply(ctx context.Context, req *workflowpb.ExecuteActionRequest, with, inputs map[string]any) (*workflowpb.ExecuteActionResponse, error) {
	get := pickField(with, inputs)

	rollback := boolField(get, "rollback_mode")
	preserveConfigs := boolField(get, "preserve_configs")
	if rollback && !mapHasKey(with, "preserve_configs") && !mapHasKey(inputs, "preserve_configs") {
		// Default-on for rollback so operator overrides don't get clobbered.
		preserveConfigs = true
	}

	apply := &node_agentpb.ApplyPackageReleaseRequest{
		PackageName:           strField(get, "package_name", "name"),
		PackageKind:           strField(get, "package_kind", "kind"),
		Version:               strField(get, "version", "target_version"),
		Publisher:             strField(get, "publisher", "publisher_id"),
		Platform:              strField(get, "platform"),
		Force:                 boolField(get, "force"),
		ExpectedSha256:        strField(get, "expected_sha256", "checksum"),
		OperationId:           defaultIfEmpty(req.GetRunId(), strField(get, "operation_id")),
		RepositoryAddr:        strField(get, "repository_addr"),
		BuildNumber:           int64Field(get, "build_number"),
		BuildId:               strField(get, "build_id"),
		RollbackMode:          rollback,
		RollbackReason:        strField(get, "rollback_reason", "reason"),
		WorkflowRunId:         req.GetRunId(),
		TargetRevisionId:      strField(get, "target_revision_id"),
		PreserveConfigs:       preserveConfigs,
		RestoreConfigSnapshot: boolField(get, "restore_config_snapshot"),
		AllowDowngrade:        rollback || boolField(get, "allow_downgrade"),
		PreviousRevisionId:    strField(get, "previous_revision_id"),
	}
	if apply.PackageName == "" {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: "package.apply: missing package_name"}, nil
	}

	resp, err := a.srv.ApplyPackageRelease(ctx, apply)
	if err != nil {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("package.apply failed: %v", err),
		}, nil
	}
	out := map[string]any{
		"status":       resp.GetStatus(),
		"package_name": resp.GetPackageName(),
		"version":      resp.GetVersion(),
		"build_id":     resp.GetBuildId(),
		"operation_id": resp.GetOperationId(),
		"checksum":     resp.GetChecksum(),
	}
	outJSON, _ := json.Marshal(out)
	return &workflowpb.ExecuteActionResponse{
		Ok:         resp.GetOk(),
		Message:    resp.GetMessage(),
		OutputJson: string(outJSON),
	}, nil
}

func (a *NodeAgentActorServer) handleServiceDrain(ctx context.Context, _ *workflowpb.ExecuteActionRequest, with, inputs map[string]any) (*workflowpb.ExecuteActionResponse, error) {
	unit := unitFor(pickField(with, inputs))
	if unit == "" {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: "service.drain: missing package_name/unit"}, nil
	}
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := supervisor.Stop(stopCtx, unit); err != nil {
		// Best-effort: a stopped unit is fine.
		log.Printf("actor: service.drain %s: %v (treating as ok if already stopped)", unit, err)
	}
	return &workflowpb.ExecuteActionResponse{Ok: true, Message: "drained " + unit}, nil
}

func (a *NodeAgentActorServer) handleServiceStart(ctx context.Context, _ *workflowpb.ExecuteActionRequest, with, inputs map[string]any) (*workflowpb.ExecuteActionResponse, error) {
	unit := unitFor(pickField(with, inputs))
	if unit == "" {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: "service.start: missing package_name/unit"}, nil
	}
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := supervisor.Restart(startCtx, unit); err != nil {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: fmt.Sprintf("service.start %s: %v", unit, err)}, nil
	}
	return &workflowpb.ExecuteActionResponse{Ok: true, Message: "started " + unit}, nil
}

func (a *NodeAgentActorServer) handleServiceHealthProbe(ctx context.Context, _ *workflowpb.ExecuteActionRequest, with, inputs map[string]any) (*workflowpb.ExecuteActionResponse, error) {
	unit := unitFor(pickField(with, inputs))
	if unit == "" {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: "service.health_probe: missing package_name/unit"}, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := supervisor.WaitActive(probeCtx, unit, 30*time.Second); err != nil {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: fmt.Sprintf("service.health_probe %s: %v", unit, err),
		}, nil
	}
	return &workflowpb.ExecuteActionResponse{Ok: true, Message: unit + " active"}, nil
}

func (a *NodeAgentActorServer) handleVerifyIntegrity(ctx context.Context, _ *workflowpb.ExecuteActionRequest, with, inputs map[string]any) (*workflowpb.ExecuteActionResponse, error) {
	get := pickField(with, inputs)
	pkg := strField(get, "package_name", "name")
	resp, err := a.srv.VerifyPackageIntegrity(ctx, &node_agentpb.VerifyPackageIntegrityRequest{
		PackageName: pkg,
	})
	if err != nil {
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: fmt.Sprintf("verify-integrity: %v", err)}, nil
	}
	return &workflowpb.ExecuteActionResponse{
		Ok:         resp.GetOk(),
		Message:    resp.GetErrorDetail(),
		OutputJson: resp.GetReportJson(),
	}, nil
}

// handleForwardRecover re-applies the previous revision when a rollback
// install fails AFTER the previous package was modified. Best-effort —
// returns ok=false with a clear message when the previous revision can't
// be reconstructed (e.g. no previous_revision_id was supplied).
func (a *NodeAgentActorServer) handleForwardRecover(ctx context.Context, req *workflowpb.ExecuteActionRequest, with, inputs map[string]any) (*workflowpb.ExecuteActionResponse, error) {
	get := pickField(with, inputs)
	prevName := strField(get, "package_name", "name")
	prevVersion := strField(get, "previous_version", "current_version")
	prevBuildID := strField(get, "previous_build_id")
	if prevName == "" || prevVersion == "" {
		return &workflowpb.ExecuteActionResponse{
			Ok:      false,
			Message: "forward_recover: previous revision identity unknown — operator must intervene",
		}, nil
	}
	apply := &node_agentpb.ApplyPackageReleaseRequest{
		PackageName:    prevName,
		PackageKind:    strField(get, "package_kind", "kind"),
		Version:        prevVersion,
		Publisher:      strField(get, "publisher", "publisher_id"),
		Platform:       strField(get, "platform"),
		Force:          true, // recovery may be a downgrade itself
		BuildId:        prevBuildID,
		BuildNumber:    int64Field(get, "previous_build_number"),
		OperationId:    "forward_recover-" + req.GetRunId(),
		WorkflowRunId:  req.GetRunId(),
		AllowDowngrade: true,
	}
	resp, err := a.srv.ApplyPackageRelease(ctx, apply)
	if err != nil || !resp.GetOk() {
		msg := "forward_recover: " + resp.GetMessage()
		if err != nil {
			msg = fmt.Sprintf("forward_recover failed: %v", err)
		}
		return &workflowpb.ExecuteActionResponse{Ok: false, Message: msg}, nil
	}
	return &workflowpb.ExecuteActionResponse{
		Ok:      true,
		Message: fmt.Sprintf("forward_recover: restored %s@%s", prevName, prevVersion),
	}, nil
}

// ── decoding helpers ─────────────────────────────────────────────────────

// decodeActionInputs unmarshals `with:` and workflow inputs into typed maps.
// On parse failure both maps are empty; the action handler reports a clean
// "missing field" error rather than silently doing the wrong thing.
func decodeActionInputs(req *workflowpb.ExecuteActionRequest) (with, inputs map[string]any) {
	with = make(map[string]any)
	inputs = make(map[string]any)
	if s := req.GetWithJson(); s != "" {
		_ = json.Unmarshal([]byte(s), &with)
	}
	if s := req.GetInputsJson(); s != "" {
		_ = json.Unmarshal([]byte(s), &inputs)
	}
	return with, inputs
}

// pickField returns a getter that prefers `with:` then falls back to inputs.
type fieldGetter func(keys ...string) any

func pickField(with, inputs map[string]any) fieldGetter {
	return func(keys ...string) any {
		for _, k := range keys {
			if v, ok := with[k]; ok && v != nil {
				return v
			}
		}
		for _, k := range keys {
			if v, ok := inputs[k]; ok && v != nil {
				return v
			}
		}
		return nil
	}
}

func mapHasKey(m map[string]any, k string) bool { _, ok := m[k]; return ok }

func strField(get fieldGetter, keys ...string) string {
	v := get(keys...)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func boolField(get fieldGetter, keys ...string) bool {
	v := get(keys...)
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, _ := strconv.ParseBool(strings.TrimSpace(t))
		return b
	}
	return false
}

func int64Field(get fieldGetter, keys ...string) int64 {
	v := get(keys...)
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	}
	return 0
}

func defaultIfEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

// unitFor maps a package_name input to the systemd unit name node-agent uses
// elsewhere ("globular-<name>.service" with hyphens).
func unitFor(get fieldGetter) string {
	name := strField(get, "package_name", "name")
	if name == "" {
		return ""
	}
	return "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/govops"
	pb "github.com/globulario/services/golang/govops/governed_operationpb"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

// ── globular ops ─────────────────────────────────────────────────────────────
//
// The Governed Operation Gateway CLI (Slice 3). `ops preflight` is the runtime
// front-door: it loads an OperationRequest and runs govops.Validate (the same gate
// the controller and MCP enforce), reporting the decision before any mutation.
// `ops apply` preflights, refuses on a non-allowed verdict, and records an
// OperationLedgerEntry. `ops ledger` queries the audit ledger. The gate extends the
// existing governor (plan/validate/approve/execute), it does not replace it.

var opsCmd = &cobra.Command{
	Use:   "ops",
	Short: "Governed Operation Gateway — preflight, apply, and audit live mutations",
}

var opsPreflightCmd = &cobra.Command{
	Use:   "preflight <operation-file.json>",
	Short: "Validate an OperationRequest against the governed-operation rules (no mutation)",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpsPreflight,
}

var opsApplyCmd = &cobra.Command{
	Use:   "apply <operation-file.json>",
	Short: "Preflight an OperationRequest, dispatch it through the owner path if allowed, and ledger the result",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpsApply,
}

var (
	opsApplyYes    bool
	opsApplyDryRun bool
)

// opsLedgerStore and ownerDispatch are package vars so tests can substitute an
// in-memory ledger and a stub dispatcher (the real ones require a live cluster).
var opsLedgerStore govops.LedgerStore = govops.NewEtcdLedgerStore()

var ownerDispatch = dispatchThroughOwnerPath

var (
	opsLedgerOperation  string
	opsLedgerActor      string
	opsLedgerOwner      string
	opsLedgerInvariant  string
	opsLedgerResult     string
	opsLedgerRefused    bool
	opsLedgerBreakGlass bool
	opsLedgerSince      string
	opsLedgerUntil      string
	opsLedgerLimit      int
)

var opsLedgerCmd = &cobra.Command{
	Use:   "ledger",
	Short: "Query the operation ledger (by operation, actor, owner, invariant, result, time)",
	RunE:  runOpsLedger,
}

func init() {
	opsApplyCmd.Flags().BoolVar(&opsApplyYes, "yes", false, "Confirm dispatch of a production/destructive operation (required for those scopes)")
	opsApplyCmd.Flags().BoolVar(&opsApplyDryRun, "dry-run", false, "Validate and ledger only; do not dispatch through the owner path")

	opsLedgerCmd.Flags().StringVar(&opsLedgerOperation, "operation", "", "Filter by operation id")
	opsLedgerCmd.Flags().StringVar(&opsLedgerActor, "actor", "", "Filter by actor")
	opsLedgerCmd.Flags().StringVar(&opsLedgerOwner, "owner", "", "Filter by target owner")
	opsLedgerCmd.Flags().StringVar(&opsLedgerInvariant, "invariant", "", "Filter by a governing AWG invariant id")
	opsLedgerCmd.Flags().StringVar(&opsLedgerResult, "result", "", "Filter by result (allowed|refused|failed|completed|break_glass_completed)")
	opsLedgerCmd.Flags().BoolVar(&opsLedgerRefused, "refused", false, "Only refused operations")
	opsLedgerCmd.Flags().BoolVar(&opsLedgerBreakGlass, "break-glass", false, "Only break-glass operations")
	opsLedgerCmd.Flags().StringVar(&opsLedgerSince, "since", "", "Only entries at/after this RFC3339 time")
	opsLedgerCmd.Flags().StringVar(&opsLedgerUntil, "until", "", "Only entries at/before this RFC3339 time")
	opsLedgerCmd.Flags().IntVar(&opsLedgerLimit, "limit", 50, "Maximum entries to show")

	opsCmd.AddCommand(opsPreflightCmd)
	opsCmd.AddCommand(opsApplyCmd)
	opsCmd.AddCommand(opsLedgerCmd)
	rootCmd.AddCommand(opsCmd)
}

func loadOperationRequest(path string) (*pb.OperationRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var req pb.OperationRequest
	if err := protojson.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse %s as OperationRequest JSON: %w", path, err)
	}
	return &req, nil
}

// decisionView is the renderable form of a govops.Decision plus the governor scope.
type decisionView struct {
	Decision      string             `json:"decision"`
	ApprovalScope string             `json:"approval_scope"`
	Violations    []govops.Violation `json:"violations,omitempty"`
}

func renderDecision(req *pb.OperationRequest, d govops.Decision) {
	view := decisionView{
		Decision:      string(d.Kind),
		ApprovalScope: govops.ApprovalScope(req),
		Violations:    d.Violations,
	}
	if rootCfg.output == "json" {
		out, _ := json.MarshalIndent(view, "", "  ")
		fmt.Println(string(out))
		return
	}
	fmt.Printf("operation:      %s\n", req.GetId())
	fmt.Printf("action:         %s\n", req.GetAction())
	fmt.Printf("target:         %s/%s (owner=%s)\n", req.GetTarget().GetResourceType(), req.GetTarget().GetResourceId(), req.GetTarget().GetOwner())
	fmt.Printf("decision:       %s\n", view.Decision)
	fmt.Printf("approval scope: %s\n", emptyDash(view.ApprovalScope))
	if len(view.Violations) == 0 {
		fmt.Println("violations:     none")
		return
	}
	fmt.Println("violations:")
	for _, v := range view.Violations {
		fmt.Printf("  - [%s] %s\n", v.Code, v.Message)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func runOpsPreflight(cmd *cobra.Command, args []string) error {
	req, err := loadOperationRequest(args[0])
	if err != nil {
		return err
	}
	renderDecision(req, govops.Validate(req))
	return nil
}

func runOpsApply(cmd *cobra.Command, args []string) error {
	req, err := loadOperationRequest(args[0])
	if err != nil {
		return err
	}
	d := govops.Validate(req)
	renderDecision(req, d)

	// A refused operation is ledgered (refusals are auditable) and not dispatched.
	if d.Refused() {
		writeLedger(ledgerEntryFromRequest(req, pb.OperationResult_REFUSED))
		return fmt.Errorf("operation refused (%s) — not applied", d.Kind)
	}

	// Allowed. Production/destructive scope requires explicit confirmation before a
	// real mutation is dispatched (mirrors the governor's RequiresUserConfirmation).
	scope := govops.ApprovalScope(req)
	if !opsApplyDryRun && (scope == govops.ScopeProduction || scope == govops.ScopeDestructive) && !opsApplyYes {
		return fmt.Errorf("operation has %q scope — re-run with --yes to dispatch (or --dry-run to validate + ledger only)", scope)
	}

	if opsApplyDryRun {
		writeLedger(ledgerEntryFromRequest(req, pb.OperationResult_ALLOWED))
		fmt.Println("\nALLOWED and ledgered (--dry-run: not dispatched).")
		return nil
	}

	// Dispatch through the owner path.
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	outcome, derr := ownerDispatch(ctx, req)

	result := pb.OperationResult_COMPLETED
	if derr != nil || !outcome.Succeeded {
		result = pb.OperationResult_FAILED
	}
	entry := ledgerEntryFromRequest(req, result)
	entry.GenerationAfter = outcome.GenerationAfter
	entry.ReconcileObserved = outcome.ReconcileObserved
	entry.PostconditionsPassed = outcome.PostconditionsPassed
	entry.PostconditionsFailed = outcome.PostconditionsFailed
	writeLedger(entry)

	if derr != nil {
		return fmt.Errorf("dispatch through %s failed: %w", emptyDash(req.GetAuthority().GetRequiredOwnerPath()), derr)
	}
	fmt.Printf("\nCOMPLETED via %s (revision=%s). postconditions passed: %v\n",
		emptyDash(req.GetAuthority().GetRequiredOwnerPath()), emptyDash(outcome.GenerationAfter), outcome.PostconditionsPassed)
	return nil
}

func writeLedger(e *pb.OperationLedgerEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	if lerr := opsLedgerStore.Put(ctx, e); lerr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write ledger entry: %v\n", lerr)
	}
}

// dispatchOutcome is the result of executing an operation through its owner path.
type dispatchOutcome struct {
	Succeeded            bool
	GenerationAfter      string
	ReconcileObserved    bool
	PostconditionsPassed []string
	PostconditionsFailed []string
}

// dispatchThroughOwnerPath routes an allowed OperationRequest to the typed owner RPC
// named in authority.required_owner_path. Only typed owner paths are wired — there is
// no generic raw-write escape hatch.
func dispatchThroughOwnerPath(ctx context.Context, req *pb.OperationRequest) (dispatchOutcome, error) {
	switch req.GetAuthority().GetRequiredOwnerPath() {
	case "cluster_controller.UpsertDesiredService":
		return dispatchUpsertDesired(ctx, req)
	case "cluster_controller.RemoveDesiredService":
		return dispatchRemoveDesired(ctx, req)
	case "cluster_controller.ApplyInfrastructureRelease":
		return dispatchApplyInfraRelease(ctx, req)
	default:
		return dispatchOutcome{}, fmt.Errorf(
			"no governed dispatcher for owner path %q — only typed owner RPCs are wired "+
				"(cluster_controller.UpsertDesiredService, RemoveDesiredService, ApplyInfrastructureRelease)",
			req.GetAuthority().GetRequiredOwnerPath())
	}
}

// dispatchUpsertDesired performs a desired-state write through the controller's typed
// owner RPC. The controller re-guards cross-kind writes server-side (Slice 4), so this
// path cannot bypass desired.keyed_by_kind_and_name even if validation were skipped.
func dispatchUpsertDesired(ctx context.Context, req *pb.OperationRequest) (dispatchOutcome, error) {
	serviceID := req.GetTarget().GetResourceId()
	if serviceID == "" {
		return dispatchOutcome{}, fmt.Errorf("target.resourceId (service name) is required")
	}
	version := req.GetParameters()["version"]
	if version == "" {
		return dispatchOutcome{}, fmt.Errorf("parameters.version is required for UpsertDesiredService")
	}
	var build int64
	if b := req.GetParameters()["build_number"]; b != "" {
		if n, perr := strconv.ParseInt(b, 10, 64); perr == nil {
			build = n
		}
	}

	conn, err := controllerClient()
	if err != nil {
		return dispatchOutcome{}, fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer func() { _ = conn.Close() }()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	resp, err := cc.UpsertDesiredService(ctx, &cluster_controllerpb.UpsertDesiredServiceRequest{
		Service: &cluster_controllerpb.DesiredService{ServiceId: serviceID, Version: version, BuildNumber: build},
	})
	if err != nil {
		return dispatchOutcome{Succeeded: false, PostconditionsFailed: []string{"owner_rpc_accepted"}}, err
	}
	// The owner RPC bumped the desired-state revision and triggered reconcile
	// server-side; record the new revision as the post-mutation generation marker.
	return dispatchOutcome{
		Succeeded:            true,
		GenerationAfter:      resp.GetRevision(),
		PostconditionsPassed: []string{"owner_rpc_accepted"},
	}, nil
}

// dispatchRemoveDesired removes a desired service through the controller's typed
// owner RPC (which deletes the ServiceDesiredVersion and drives the removal
// workflow). The controller re-guards cross-kind removes server-side, so an
// infrastructure package cannot be removed through this service path.
func dispatchRemoveDesired(ctx context.Context, req *pb.OperationRequest) (dispatchOutcome, error) {
	serviceID := req.GetTarget().GetResourceId()
	if serviceID == "" {
		return dispatchOutcome{}, fmt.Errorf("target.resourceId (service name) is required")
	}
	conn, err := controllerClient()
	if err != nil {
		return dispatchOutcome{}, fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer func() { _ = conn.Close() }()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	resp, err := cc.RemoveDesiredService(ctx, &cluster_controllerpb.RemoveDesiredServiceRequest{ServiceId: serviceID})
	if err != nil {
		return dispatchOutcome{Succeeded: false, PostconditionsFailed: []string{"owner_rpc_accepted"}}, err
	}
	return dispatchOutcome{
		Succeeded:            true,
		GenerationAfter:      resp.GetRevision(),
		PostconditionsPassed: []string{"owner_rpc_accepted"},
	}, nil
}

// dispatchApplyInfraRelease applies an infrastructure release through the
// controller's typed ResourcesService owner RPC. It is deliberately NOT a
// "clear yellow" wand: it applies a real release and then reports what the owner
// path actually crossed (generation) and resolved (status), naming anything that
// remained pending rather than asserting convergence the CLI cannot observe.
//
// Required typed parameters (no generic key/path/value escape hatch):
//
//	service_id        the infrastructure component (e.g. "xds")
//	publisher_id      the artifact publisher namespace
//	version           the release version to apply
//	release_channel   the channel the release is drawn from
//	reason            operator-supplied justification (recorded)
//	build_number      optional build iteration (default 0)
//	expected_current_version  optional optimistic precondition — refuse a blind
//	                          apply if the current spec version differs
func dispatchApplyInfraRelease(ctx context.Context, req *pb.OperationRequest) (dispatchOutcome, error) {
	p := req.GetParameters()
	component := firstNonEmpty(p["service_id"], p["target_services"], req.GetTarget().GetResourceId())
	publisher := p["publisher_id"]
	version := p["version"]
	channel := p["release_channel"]
	reason := p["reason"]
	for name, val := range map[string]string{
		"service_id (component)": component, "publisher_id": publisher,
		"version": version, "release_channel": channel, "reason": reason,
	} {
		if strings.TrimSpace(val) == "" {
			return dispatchOutcome{}, fmt.Errorf("parameters.%s is required for ApplyInfrastructureRelease", name)
		}
	}
	var build int64
	if b := p["build_number"]; b != "" {
		if n, perr := strconv.ParseInt(b, 10, 64); perr == nil {
			build = n
		}
	}

	conn, err := controllerClient()
	if err != nil {
		return dispatchOutcome{}, fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer func() { _ = conn.Close() }()
	rc := cluster_controllerpb.NewResourcesServiceClient(conn)
	name := publisher + "/" + component

	// Optimistic precondition: refuse a blind apply over an unexpected current state.
	if want := strings.TrimSpace(p["expected_current_version"]); want != "" {
		cur, gerr := rc.GetInfrastructureRelease(ctx, &cluster_controllerpb.GetInfrastructureReleaseRequest{Name: name})
		if gerr == nil {
			if got := infraSpecVersion(cur); got != want {
				return dispatchOutcome{Succeeded: false, PostconditionsFailed: []string{"expected_current_version"}},
					fmt.Errorf("expected current version %q for %s but found %q — refusing blind apply", want, name, got)
			}
		}
	}

	obj := &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: name, Annotations: map[string]string{"reason": reason}},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			PublisherID: publisher, Component: component, Version: version, BuildNumber: build, Channel: channel,
		},
	}
	applied, err := rc.ApplyInfrastructureRelease(ctx, &cluster_controllerpb.ApplyInfrastructureReleaseRequest{Object: obj})
	if err != nil {
		return dispatchOutcome{Succeeded: false, PostconditionsFailed: []string{"owner_rpc_accepted"}}, err
	}

	var gen int64
	if applied != nil && applied.Meta != nil {
		gen = applied.Meta.Generation
	}
	out := dispatchOutcome{
		Succeeded:            true,
		GenerationAfter:      strconv.FormatInt(gen, 10),
		PostconditionsPassed: []string{"owner_rpc_accepted"},
	}
	// resolved_version_observed — only if the owner path already re-resolved to the
	// applied version; otherwise NAME the pending reconcile rather than claim success.
	if infraResolvedVersion(applied) == version {
		out.PostconditionsPassed = append(out.PostconditionsPassed, "resolved_version_observed")
	} else {
		out.PostconditionsFailed = append(out.PostconditionsFailed, "reconcile_pending:resolved_version")
	}
	// The derived cache/digest projection is observed downstream by the reconciler /
	// doctor, not by this apply call — always named pending, never asserted.
	out.PostconditionsFailed = append(out.PostconditionsFailed, "projection_pending:cache_digest")
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func infraSpecVersion(r *cluster_controllerpb.InfrastructureRelease) string {
	if r == nil || r.Spec == nil {
		return ""
	}
	return r.Spec.Version
}

func infraResolvedVersion(r *cluster_controllerpb.InfrastructureRelease) string {
	if r == nil || r.Status == nil {
		return ""
	}
	return r.Status.ResolvedVersion
}

func ledgerEntryFromRequest(req *pb.OperationRequest, result pb.OperationResult) *pb.OperationLedgerEntry {
	return &pb.OperationLedgerEntry{
		OperationId:           req.GetId(),
		Timestamp:             time.Now().UTC().Format(time.RFC3339Nano),
		Actor:                 req.GetActor().String(),
		Command:               req.GetAction(),
		TargetOwner:           req.GetTarget().GetOwner(),
		TargetResource:        req.GetTarget().GetResourceId(),
		AwgInvariants:         req.GetEvidence().GetAwgInvariants(),
		BehavioralRules:       req.GetEvidence().GetBehavioralRules(),
		RelatedAiMemoryEvents: req.GetEvidence().GetRelatedIncidents(),
		BeforeStateHash:       req.GetEvidence().GetBeforeSnapshot(),
		Result:                result,
	}
}

var resultByName = map[string]pb.OperationResult{
	"allowed":               pb.OperationResult_ALLOWED,
	"refused":               pb.OperationResult_REFUSED,
	"failed":                pb.OperationResult_FAILED,
	"completed":             pb.OperationResult_COMPLETED,
	"break_glass_completed": pb.OperationResult_BREAK_GLASS_COMPLETED,
}

func runOpsLedger(cmd *cobra.Command, args []string) error {
	filter := govops.LedgerFilter{
		OperationID:    opsLedgerOperation,
		Actor:          opsLedgerActor,
		Owner:          opsLedgerOwner,
		Invariant:      opsLedgerInvariant,
		Result:         resultByName[opsLedgerResult],
		OnlyRefused:    opsLedgerRefused,
		OnlyBreakGlass: opsLedgerBreakGlass,
		Since:          opsLedgerSince,
		Until:          opsLedgerUntil,
	}
	if opsLedgerResult != "" && filter.Result == pb.OperationResult_OPERATION_RESULT_UNSPECIFIED {
		return fmt.Errorf("unknown --result %q (allowed|refused|failed|completed|break_glass_completed)", opsLedgerResult)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	all, err := opsLedgerStore.List(ctx)
	if err != nil {
		return err
	}
	matched := govops.QueryLedger(all, filter)
	if len(matched) > opsLedgerLimit {
		matched = matched[:opsLedgerLimit]
	}

	if len(matched) == 0 {
		fmt.Println("no ledger entries match")
		return nil
	}
	for _, e := range matched {
		if rootCfg.output == "json" {
			printProto(e)
			continue
		}
		fmt.Printf("%s  %-22s  actor=%-14s owner=%-14s %s\n",
			e.GetTimestamp(), e.GetOperationId(), e.GetActor(), emptyDash(e.GetTargetOwner()), e.GetResult().String())
	}
	return nil
}

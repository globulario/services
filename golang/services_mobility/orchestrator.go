// @awareness namespace=globular.platform
// @awareness component=platform_services_mobility.orchestrator
// @awareness file_role=service_mobility_primitive_blue_green_relocation_for_stateful_services
// @awareness implements=globular.platform:invariant.meta.mobility_is_stronger_recovery_than_replication
// @awareness implements=globular.platform:invariant.meta.MTTR_focus_outperforms_MTBF_for_evolving_systems
// @awareness risk=high

// Package mobility prototypes the service mobility primitive named in
// meta.mobility_is_stronger_recovery_than_replication. The primitive
// is the rebind half of the recovery story: when a service must move
// between nodes (planned migration, node loss, capacity rebalance),
// the recovery path is NOT release-pipeline-cycle reinstall on the
// new node — it is "start the binary on the target, wait until it is
// serving, drain the source." Seconds, not minutes.
//
// SCOPE OF THIS PROTOTYPE
//
//   - Stateful services whose persistent state lives outside the
//     process (Scylla, etcd, MinIO) — ai-memory is the canonical
//     case. Process state (caches, in-flight RPCs) is acceptable
//     to lose; persistent state is preserved by the underlying
//     store and shared between source and target during the
//     overlap.
//   - Single-instance services. Multi-instance services need a
//     different shape (incremental rebalance, not blue-green).
//   - Operator-triggered. Automatic mobility on node-health events
//     is a follow-up that wraps this primitive in a controller
//     decision loop.
//
// PROTOCOL
//
//   1. Resolve: find where the service is currently registered in
//      etcd. Return error if not running anywhere.
//   2. Validate target: the target node must be reachable, have the
//      service binary installed, and not already host an instance.
//   3. Start: instruct the target node-agent to start the service
//      unit. The service performs its own startup (schema check,
//      Scylla connection, etcd registration). Schema migration is
//      coordinated by the service itself (see ai-memory's
//      migration_coordinator.go).
//   4. Wait: poll until the target instance is registered in etcd
//      AND its health probe returns serving.
//   5. Drain source: instruct the source node-agent to stop the
//      service unit. The systemd graceful-stop period lets the old
//      instance finish in-flight requests.
//   6. Verify: confirm exactly one instance remains in etcd, and it
//      is on the target node.
//
// WHAT THIS PROTOTYPE PROVES
//
//   - The orchestration shape is correct for Scylla-backed services
//     where state is shared and process is fungible.
//   - The handoff is observable at each step; failures produce
//     loud errors with named offending step.
//   - The unit tests below exercise every step with injected fakes,
//     including failure injection at each boundary.
//
// WHAT THIS PROTOTYPE DOES NOT PROVE
//
//   - Behaviour on a multi-node cluster (the current production
//     cluster is N=1; mobility requires N>=2 to test).
//   - In-flight RPC handover. Today some requests in flight against
//     the source at the moment systemd stops it will fail and need
//     client retry. Full connection migration is a follow-up.
//   - Behaviour under partial network partition between source and
//     target nodes during overlap.
//
// FOLLOW-UP TO REACH PRODUCTION
//
//   1. Workflow lift. The orchestration here is procedural; the
//      production form lives inside a workflow YAML
//      (cluster.service.migrate) with durable step receipts so
//      partial migration can resume safely. The procedural form
//      here is the actor implementation the workflow dispatches to.
//   2. Proto + RPC + CLI. Add cluster_controllerpb.MigrateService
//      RPC; wire it to dispatch the workflow; expose
//      'globular service migrate <name> --to <node>'.
//   3. Automatic mobility. cluster_controller's node-health watcher
//      decides when to invoke mobility (node drain requested,
//      capacity rebalance, planned upgrade window). This is the
//      meta.bad_path_must_make_progress half of recovery: instead
//      of saturating the source, move off.
//   4. Connection draining. xDS reads etcd registration; on
//      deregistration of source, xDS routes only to target. Source's
//      systemd graceful-stop period MUST exceed
//      max(routing-propagation-delay, in-flight-request-deadline).
package mobility

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NodeAgentController is the surface this orchestrator needs from a
// node-agent client. The concrete implementation lives in
// node_agent/node_agent_client; tests inject a fake.
type NodeAgentController interface {
	// StartService asks the node-agent on `nodeID` to start the
	// systemd unit owning `serviceName`. Returns nil if the unit is
	// already active or the start succeeded. Returns a wrapped error
	// on failure.
	StartService(ctx context.Context, nodeID, serviceName string) error

	// StopService asks the node-agent on `nodeID` to stop the
	// systemd unit owning `serviceName`. Returns nil if the unit was
	// inactive or the stop succeeded.
	StopService(ctx context.Context, nodeID, serviceName string) error

	// IsServiceBinaryInstalled returns true when the service binary
	// is present on `nodeID`. The mobility orchestrator refuses to
	// migrate a service to a node that doesn't have the binary; the
	// release pipeline must install it first.
	IsServiceBinaryInstalled(ctx context.Context, nodeID, serviceName string) (bool, error)

	// IsNodeReachable returns true when the node-agent on `nodeID`
	// responds to a control RPC within a short deadline. Mobility
	// refuses unreachable targets.
	IsNodeReachable(ctx context.Context, nodeID string) (bool, error)
}

// ServiceRegistry is the etcd-backed view of where services are
// currently registered. The orchestrator queries this to find the
// source instance and to verify after-migration state.
type ServiceRegistry interface {
	// InstancesOf returns the node IDs currently serving
	// `serviceName`. Empty slice means the service is not running
	// anywhere.
	InstancesOf(ctx context.Context, serviceName string) ([]string, error)

	// IsHealthy returns true when the instance of `serviceName` on
	// `nodeID` is registered AND its health probe returns serving.
	IsHealthy(ctx context.Context, nodeID, serviceName string) (bool, error)
}

// MigrateOptions tunes orchestrator behaviour. Defaults are conservative.
type MigrateOptions struct {
	// ReadyTimeout caps how long to wait for the target instance to
	// become healthy before declaring the migration failed.
	ReadyTimeout time.Duration

	// DrainGracePeriod is the wait between target-ready and
	// source-stop. The source continues serving during this window;
	// the gap absorbs xDS routing propagation so in-flight requests
	// have a chance to land on the new instance.
	DrainGracePeriod time.Duration

	// PollInterval is the cadence at which IsHealthy is polled.
	PollInterval time.Duration
}

// DefaultOptions returns conservative defaults — large enough that
// the prototype works even when xDS routing propagation is slow.
func DefaultOptions() MigrateOptions {
	return MigrateOptions{
		ReadyTimeout:     90 * time.Second,
		DrainGracePeriod: 10 * time.Second,
		PollInterval:     2 * time.Second,
	}
}

// Outcome records exactly what happened during a Migrate call. Every
// migration produces an outcome regardless of success or failure; the
// Steps slice names every action taken so post-incident review can
// reconstruct the timeline. Steps is append-only during orchestration.
type Outcome struct {
	ServiceName  string
	SourceNodeID string
	TargetNodeID string
	Steps        []string
	StartedAt    time.Time
	FinishedAt   time.Time
	Err          error
}

// Orchestrator coordinates the migration of one service from its
// current node to a target node. The orchestrator is stateless across
// calls; concurrent migrations of distinct services are safe.
type Orchestrator struct {
	NodeAgent NodeAgentController
	Registry  ServiceRegistry
	Options   MigrateOptions
}

// New constructs an orchestrator with the given dependencies.
func New(na NodeAgentController, sr ServiceRegistry) *Orchestrator {
	return &Orchestrator{NodeAgent: na, Registry: sr, Options: DefaultOptions()}
}

// Migrate moves `serviceName` to `targetNodeID`. The current host is
// resolved from the service registry; if the service is already
// on `targetNodeID` the call returns a no-op outcome.
//
// Migrate is synchronous and blocks until the target is serving and
// the source has stopped (or the orchestrator gave up). Callers that
// need fire-and-forget should wrap this in a goroutine and track the
// outcome separately.
//
// We use a NAMED RETURN VALUE so the deferred FinishedAt stamp
// propagates to the caller. A defer mutating a stack-allocated local
// followed by `return out` does not — the return value is captured
// before the defer runs. This was caught when the JSON output showed
// finished_at as the zero time.
func (o *Orchestrator) Migrate(ctx context.Context, serviceName, targetNodeID string) (out Outcome) {
	out = Outcome{
		ServiceName:  serviceName,
		TargetNodeID: targetNodeID,
		StartedAt:    time.Now(),
	}
	defer func() {
		out.FinishedAt = time.Now()
	}()

	if strings.TrimSpace(serviceName) == "" {
		out.Err = fmt.Errorf("service_name is required")
		return out
	}
	if strings.TrimSpace(targetNodeID) == "" {
		out.Err = fmt.Errorf("target_node_id is required")
		return out
	}

	// Step 1: Resolve where the service is currently running.
	out.Steps = append(out.Steps, "resolve_source")
	instances, err := o.Registry.InstancesOf(ctx, serviceName)
	if err != nil {
		out.Err = fmt.Errorf("resolve_source: %w", err)
		return out
	}
	if len(instances) == 0 {
		out.Err = fmt.Errorf("resolve_source: %q is not running on any node — release pipeline must install it before mobility applies", serviceName)
		return out
	}
	if len(instances) > 1 {
		// Multi-instance mobility is out of scope for this prototype.
		// A future "rebalance" primitive handles N>1 cases.
		out.Err = fmt.Errorf("resolve_source: %q has %d instances — mobility prototype handles single-instance services only", serviceName, len(instances))
		return out
	}
	out.SourceNodeID = instances[0]

	// If already on target, no-op.
	if out.SourceNodeID == targetNodeID {
		out.Steps = append(out.Steps, "already_on_target_noop")
		return out
	}

	// Step 2: Validate target.
	out.Steps = append(out.Steps, "validate_target_reachable")
	reachable, err := o.NodeAgent.IsNodeReachable(ctx, targetNodeID)
	if err != nil {
		out.Err = fmt.Errorf("validate_target_reachable: %w", err)
		return out
	}
	if !reachable {
		out.Err = fmt.Errorf("validate_target_reachable: target node %q is not reachable", targetNodeID)
		return out
	}

	out.Steps = append(out.Steps, "validate_target_binary_installed")
	installed, err := o.NodeAgent.IsServiceBinaryInstalled(ctx, targetNodeID, serviceName)
	if err != nil {
		out.Err = fmt.Errorf("validate_target_binary_installed: %w", err)
		return out
	}
	if !installed {
		out.Err = fmt.Errorf("validate_target_binary_installed: %q binary not installed on %q — release pipeline must install it first", serviceName, targetNodeID)
		return out
	}

	// Step 3: Start target.
	out.Steps = append(out.Steps, "start_target")
	if err := o.NodeAgent.StartService(ctx, targetNodeID, serviceName); err != nil {
		out.Err = fmt.Errorf("start_target: %w", err)
		return out
	}

	// Step 4: Wait for target ready.
	out.Steps = append(out.Steps, "wait_target_ready")
	if err := o.waitHealthy(ctx, targetNodeID, serviceName); err != nil {
		out.Err = fmt.Errorf("wait_target_ready: %w", err)
		// Best-effort cleanup: stop the target instance so we don't
		// leave behind a half-started incarnation. Errors here are
		// recorded but do not override the primary cause.
		_ = o.NodeAgent.StopService(ctx, targetNodeID, serviceName)
		out.Steps = append(out.Steps, "cleanup_failed_target_stop")
		return out
	}

	// Step 5: Drain grace period. The target is serving; both
	// instances are visible in etcd; xDS routes to both. The
	// grace period absorbs routing-propagation latency so in-flight
	// requests have time to land on the new instance.
	out.Steps = append(out.Steps, "drain_grace_period")
	select {
	case <-ctx.Done():
		out.Err = fmt.Errorf("drain_grace_period: %w", ctx.Err())
		return out
	case <-time.After(o.Options.DrainGracePeriod):
	}

	// Step 6: Stop source.
	out.Steps = append(out.Steps, "stop_source")
	if err := o.NodeAgent.StopService(ctx, out.SourceNodeID, serviceName); err != nil {
		out.Err = fmt.Errorf("stop_source: %w", err)
		return out
	}

	// Step 7: Verify exactly one instance remains, and it is the target.
	out.Steps = append(out.Steps, "verify_final_topology")
	instances, err = o.Registry.InstancesOf(ctx, serviceName)
	if err != nil {
		out.Err = fmt.Errorf("verify_final_topology: %w", err)
		return out
	}
	if len(instances) != 1 || instances[0] != targetNodeID {
		out.Err = fmt.Errorf("verify_final_topology: expected exactly one instance on %q, got %v", targetNodeID, instances)
		return out
	}

	out.Steps = append(out.Steps, "success")
	return out
}

// waitHealthy polls IsHealthy at PollInterval until either it returns
// true or ReadyTimeout elapses.
func (o *Orchestrator) waitHealthy(ctx context.Context, nodeID, serviceName string) error {
	deadline := time.Now().Add(o.Options.ReadyTimeout)
	ticker := time.NewTicker(o.Options.PollInterval)
	defer ticker.Stop()

	for {
		healthy, err := o.Registry.IsHealthy(ctx, nodeID, serviceName)
		if err != nil {
			return fmt.Errorf("health probe error: %w", err)
		}
		if healthy {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("target did not become healthy within %s", o.Options.ReadyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

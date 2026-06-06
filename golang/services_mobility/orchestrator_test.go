// @awareness namespace=globular.platform
// @awareness component=platform_services_mobility.orchestrator_test
// @awareness file_role=mobility_protocol_tests_with_injected_fakes
// @awareness risk=medium
package mobility

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeNodeAgent implements NodeAgentController with explicit failure
// hooks. Each method's behaviour is controlled by per-call function
// fields so each test can shape the boundary precisely.
type fakeNodeAgent struct {
	mu             sync.Mutex
	calls          []string
	startService   func(nodeID, serviceName string) error
	stopService    func(nodeID, serviceName string) error
	binaryInstalled func(nodeID, serviceName string) (bool, error)
	nodeReachable  func(nodeID string) (bool, error)
}

func (f *fakeNodeAgent) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

func (f *fakeNodeAgent) StartService(_ context.Context, nodeID, serviceName string) error {
	f.record("StartService:" + nodeID + ":" + serviceName)
	if f.startService != nil {
		return f.startService(nodeID, serviceName)
	}
	return nil
}
func (f *fakeNodeAgent) StopService(_ context.Context, nodeID, serviceName string) error {
	f.record("StopService:" + nodeID + ":" + serviceName)
	if f.stopService != nil {
		return f.stopService(nodeID, serviceName)
	}
	return nil
}
func (f *fakeNodeAgent) IsServiceBinaryInstalled(_ context.Context, nodeID, serviceName string) (bool, error) {
	f.record("IsServiceBinaryInstalled:" + nodeID + ":" + serviceName)
	if f.binaryInstalled != nil {
		return f.binaryInstalled(nodeID, serviceName)
	}
	return true, nil
}
func (f *fakeNodeAgent) IsNodeReachable(_ context.Context, nodeID string) (bool, error) {
	f.record("IsNodeReachable:" + nodeID)
	if f.nodeReachable != nil {
		return f.nodeReachable(nodeID)
	}
	return true, nil
}

// fakeRegistry implements ServiceRegistry. The instances map carries the
// current "registered on which nodes" state for each service.
// StartService and StopService calls on fakeNodeAgent update this map
// to model the etcd round-trip — but only when the test sets the
// startService/stopService field to nil (default success path) AND
// registers a side-effect via SetInstances. Real tests just SET the
// map directly to model what the underlying systems would do.
type fakeRegistry struct {
	mu        sync.Mutex
	instances map[string][]string // service → nodeIDs
	healthy   map[string]bool     // service+node → healthy
	listErr   error
	healthErr error
}

func newRegistry() *fakeRegistry {
	return &fakeRegistry{
		instances: map[string][]string{},
		healthy:   map[string]bool{},
	}
}

func (r *fakeRegistry) SetInstances(service string, nodes ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]string, len(nodes))
	copy(cp, nodes)
	r.instances[service] = cp
}
func (r *fakeRegistry) SetHealthy(nodeID, service string, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.healthy[nodeID+"|"+service] = healthy
}

func (r *fakeRegistry) InstancesOf(_ context.Context, service string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	cp := make([]string, len(r.instances[service]))
	copy(cp, r.instances[service])
	return cp, nil
}
func (r *fakeRegistry) IsHealthy(_ context.Context, nodeID, service string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.healthErr != nil {
		return false, r.healthErr
	}
	return r.healthy[nodeID+"|"+service], nil
}

// fastOptions makes the tests sub-second by collapsing waits.
func fastOptions() MigrateOptions {
	return MigrateOptions{
		ReadyTimeout:     2 * time.Second,
		DrainGracePeriod: 10 * time.Millisecond,
		PollInterval:     5 * time.Millisecond,
	}
}

// TestMigrate_HappyPath drives the orchestrator through every step
// without injecting failures, asserting the final state is exactly one
// instance on the target and the step trail names every expected
// action in order.
func TestMigrate_HappyPath(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")

	na := &fakeNodeAgent{
		startService: func(nodeID, serviceName string) error {
			// Model the side effect: target now appears in registry.
			reg.SetInstances(serviceName, "node-A", nodeID)
			reg.SetHealthy(nodeID, serviceName, true)
			return nil
		},
		stopService: func(nodeID, serviceName string) error {
			// Source disappears from registry.
			reg.mu.Lock()
			current := reg.instances[serviceName]
			filtered := current[:0]
			for _, n := range current {
				if n != nodeID {
					filtered = append(filtered, n)
				}
			}
			reg.instances[serviceName] = filtered
			reg.mu.Unlock()
			return nil
		},
	}

	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err != nil {
		t.Fatalf("Migrate: unexpected error: %v (steps=%v)", out.Err, out.Steps)
	}
	if out.SourceNodeID != "node-A" {
		t.Errorf("source = %q, want node-A", out.SourceNodeID)
	}
	if out.TargetNodeID != "node-B" {
		t.Errorf("target = %q, want node-B", out.TargetNodeID)
	}

	wantSteps := []string{
		"resolve_source",
		"validate_target_reachable",
		"validate_target_binary_installed",
		"start_target",
		"wait_target_ready",
		"drain_grace_period",
		"stop_source",
		"verify_final_topology",
		"success",
	}
	if !stepsMatch(out.Steps, wantSteps) {
		t.Errorf("steps = %v, want %v", out.Steps, wantSteps)
	}
}

// TestMigrate_ServiceNotRunning verifies that mobility refuses to
// migrate a service that is not registered anywhere — the principle
// says rebind, not install-from-nothing.
func TestMigrate_ServiceNotRunning(t *testing.T) {
	reg := newRegistry()
	na := &fakeNodeAgent{}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "nowhere-service", "node-B")
	if out.Err == nil {
		t.Fatalf("expected error for service not running anywhere")
	}
	if !strings.Contains(out.Err.Error(), "is not running on any node") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// TestMigrate_AlreadyOnTarget is the no-op path. Migrate to the node
// the service is already on; outcome is success with a single
// "already_on_target_noop" step.
func TestMigrate_AlreadyOnTarget(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-B")
	na := &fakeNodeAgent{}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err != nil {
		t.Fatalf("no-op should not error: %v", out.Err)
	}
	if out.SourceNodeID != "node-B" {
		t.Errorf("source should be node-B, got %q", out.SourceNodeID)
	}
	wantTrail := []string{"resolve_source", "already_on_target_noop"}
	if !stepsMatch(out.Steps, wantTrail) {
		t.Errorf("steps = %v, want %v", out.Steps, wantTrail)
	}
}

// TestMigrate_MultiInstanceRejected enforces the prototype's
// scope-limitation: services with more than one instance are not
// handled by this primitive. They need a "rebalance" primitive that
// hasn't been built yet.
func TestMigrate_MultiInstanceRejected(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("doubled", "node-A", "node-B")
	na := &fakeNodeAgent{}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "doubled", "node-C")
	if out.Err == nil {
		t.Fatalf("expected rejection for multi-instance service")
	}
	if !strings.Contains(out.Err.Error(), "has 2 instances") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// TestMigrate_TargetNotReachable refuses to migrate to a node whose
// node-agent does not respond.
func TestMigrate_TargetNotReachable(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")
	na := &fakeNodeAgent{
		nodeReachable: func(nodeID string) (bool, error) { return false, nil },
	}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err == nil {
		t.Fatalf("expected error for unreachable target")
	}
	if !strings.Contains(out.Err.Error(), "not reachable") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// TestMigrate_BinaryNotInstalled refuses to migrate to a node that
// lacks the service binary. The release pipeline must install it
// first; mobility does not double as a deploy mechanism.
func TestMigrate_BinaryNotInstalled(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")
	na := &fakeNodeAgent{
		binaryInstalled: func(nodeID, serviceName string) (bool, error) { return false, nil },
	}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err == nil {
		t.Fatalf("expected error for missing binary on target")
	}
	if !strings.Contains(out.Err.Error(), "binary not installed") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// TestMigrate_TargetFailsToBecomeHealthy exercises the wait_target_ready
// timeout. Source remains running; target is stopped to avoid leaving
// a half-started incarnation.
func TestMigrate_TargetFailsToBecomeHealthy(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")
	stopCalledFor := ""
	na := &fakeNodeAgent{
		startService: func(nodeID, serviceName string) error {
			// Target appears in registry but never becomes healthy.
			reg.SetInstances(serviceName, "node-A", nodeID)
			reg.SetHealthy(nodeID, serviceName, false)
			return nil
		},
		stopService: func(nodeID, serviceName string) error {
			stopCalledFor = nodeID
			return nil
		},
	}
	o := New(na, reg)
	o.Options = MigrateOptions{
		ReadyTimeout:     50 * time.Millisecond,
		DrainGracePeriod: 5 * time.Millisecond,
		PollInterval:     5 * time.Millisecond,
	}

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err == nil {
		t.Fatalf("expected ready-timeout error")
	}
	if !strings.Contains(out.Err.Error(), "did not become healthy") {
		t.Errorf("unexpected error: %v", out.Err)
	}
	// The cleanup step must have stopped the target so we don't
	// leave behind a half-started incarnation. Source stays running.
	if stopCalledFor != "node-B" {
		t.Errorf("cleanup did not stop target; stopCalledFor=%q", stopCalledFor)
	}
	if !contains(out.Steps, "cleanup_failed_target_stop") {
		t.Errorf("missing cleanup_failed_target_stop in steps: %v", out.Steps)
	}
}

// TestMigrate_StopSourceFails covers the post-target-healthy failure
// case. Both source and target are running at this point; if we cannot
// stop the source, the cluster is left with two instances. The error
// is surfaced so an operator can resolve manually.
func TestMigrate_StopSourceFails(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")
	na := &fakeNodeAgent{
		startService: func(nodeID, serviceName string) error {
			reg.SetInstances(serviceName, "node-A", nodeID)
			reg.SetHealthy(nodeID, serviceName, true)
			return nil
		},
		stopService: func(nodeID, serviceName string) error {
			if nodeID == "node-A" {
				return errors.New("systemd unresponsive")
			}
			return nil
		},
	}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err == nil {
		t.Fatalf("expected stop_source error")
	}
	if !strings.Contains(out.Err.Error(), "stop_source") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// TestMigrate_VerifyFinalTopologyFails covers the case where the
// orchestrator believed the source was stopped but the registry still
// shows it. This is a race-window detector — the registry is the
// authority; if it disagrees with what the orchestrator just did,
// something is wrong (xDS lag, etcd partition, racing concurrent op).
func TestMigrate_VerifyFinalTopologyFails(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")
	na := &fakeNodeAgent{
		startService: func(nodeID, serviceName string) error {
			reg.SetInstances(serviceName, "node-A", nodeID)
			reg.SetHealthy(nodeID, serviceName, true)
			return nil
		},
		stopService: func(nodeID, serviceName string) error {
			// Lie about success — registry still shows both.
			return nil
		},
	}
	o := New(na, reg)
	o.Options = fastOptions()

	out := o.Migrate(context.Background(), "ai-memory", "node-B")
	if out.Err == nil {
		t.Fatalf("expected verify_final_topology error")
	}
	if !strings.Contains(out.Err.Error(), "expected exactly one instance") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// TestMigrate_ContextCancelDuringGrace verifies that context cancellation
// during the drain grace period is handled cleanly.
func TestMigrate_ContextCancelDuringGrace(t *testing.T) {
	reg := newRegistry()
	reg.SetInstances("ai-memory", "node-A")
	na := &fakeNodeAgent{
		startService: func(nodeID, serviceName string) error {
			reg.SetInstances(serviceName, "node-A", nodeID)
			reg.SetHealthy(nodeID, serviceName, true)
			return nil
		},
	}
	o := New(na, reg)
	o.Options = MigrateOptions{
		ReadyTimeout:     1 * time.Second,
		DrainGracePeriod: 5 * time.Second, // long enough to cancel during
		PollInterval:     5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	out := o.Migrate(ctx, "ai-memory", "node-B")
	if out.Err == nil {
		t.Fatalf("expected context-cancel error")
	}
	if !strings.Contains(out.Err.Error(), "drain_grace_period") {
		t.Errorf("unexpected error: %v", out.Err)
	}
}

// ── helpers ───────────────────────────────────────────────────────────

func stepsMatch(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// _ keeps fmt referenced even if all uses are removed during edits.
var _ = fmt.Sprintf

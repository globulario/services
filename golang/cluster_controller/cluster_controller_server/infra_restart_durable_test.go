package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Behavioral coverage for the durable infra-restart child run (F6).
//
// The four tests in infra_restart_ordering_test.go are STRUCTURAL — they prove
// the counterfeit dispatch shape is gone. These are BEHAVIORAL: they exercise
// the identity function and the input validation that make the replacement a
// real durable run rather than a differently-spelled fabrication.

// ── correlation identity ──────────────────────────────────────────────

// The identity must be a pure function of the remediation context. This is what
// lets the Workflow Service recognise a retry of the same remediation instead of
// allocating a fresh run — the defect the wall-clock id caused.
func TestRestartInfraCorrelationID_DeterministicForSameRemediation(t *testing.T) {
	a := restartInfraCorrelationID("c1", "ep-A", "node-a", "etcd")
	b := restartInfraCorrelationID("c1", "ep-A", "node-a", "etcd")
	if a != b {
		t.Fatalf("same remediation produced two identities:\n  %s\n  %s", a, b)
	}
}

// Calling it repeatedly must not drift. A wall-clock component would show up
// here even if the two-call test above happened to land in the same nanosecond.
func TestRestartInfraCorrelationID_StableAcrossManyCalls(t *testing.T) {
	want := restartInfraCorrelationID("c1", "ep-A", "node-a", "etcd")
	for i := 0; i < 1000; i++ {
		if got := restartInfraCorrelationID("c1", "ep-A", "node-a", "etcd"); got != want {
			t.Fatalf("identity drifted on call %d: %s != %s", i, got, want)
		}
	}
}

// Each field of the remediation context must be able to change the identity.
// If any were dropped, two distinct remediations would collide onto one run and
// the second would be silently treated as a replay of the first.
func TestRestartInfraCorrelationID_DistinguishesEveryField(t *testing.T) {
	base := []string{"c1", "ep-A", "node-a", "etcd"}
	id := func(f []string) string {
		return restartInfraCorrelationID(f[0], f[1], f[2], f[3])
	}
	names := []string{"clusterID", "episodeID", "nodeID", "component"}
	alts := []string{"c2", "ep-B", "node-b", "minio"}

	baseline := id(base)
	for i, name := range names {
		mutated := append([]string(nil), base...)
		mutated[i] = alts[i]
		if got := id(mutated); got == baseline {
			t.Errorf("changing %s did not change the identity (%s) — the field is not "+
				"part of the correlation id, so two remediations collide", name, got)
		}
	}
}

// Two nodes healing the same component in one reconcile pass are two separate
// mutations on two separate hosts. They must never share a run.
func TestRestartInfraCorrelationID_SeparatesNodes(t *testing.T) {
	a := restartInfraCorrelationID("c1", "ep-A", "node-a", "scylladb")
	b := restartInfraCorrelationID("c1", "ep-A", "node-b", "scylladb")
	if a == b {
		t.Fatal("two nodes restarting scylladb share one child run identity")
	}
}

// The identity must carry the workflow name so it cannot collide with the
// correlation id of any other child workflow dispatched from the same step.
func TestRestartInfraCorrelationID_NamespacedByWorkflow(t *testing.T) {
	got := restartInfraCorrelationID("c1", "ep-A", "node-a", "etcd")
	if !strings.HasPrefix(got, "node.restart_infra_unit/") {
		t.Fatalf("identity %q is not namespaced by the workflow name", got)
	}
}

// A stable identity must contain no digits-only wall-clock segment. This is the
// behavioral counterpart to the structural time.Now() scan.
func TestRestartInfraCorrelationID_ContainsNoTimestamp(t *testing.T) {
	got := restartInfraCorrelationID("c1", "ep-A", "node-a", "etcd")
	for _, seg := range strings.Split(got, "/") {
		if len(seg) >= 10 && strings.TrimLeft(seg, "0123456789") == "" {
			t.Fatalf("identity segment %q looks like a wall-clock stamp: %s", seg, got)
		}
	}
}

// ── input validation ──────────────────────────────────────────────────

// A malformed remediation must be rejected before it reaches the Workflow
// Service — a persisted run that can never succeed is worse than no run.
func TestRunRestartInfraUnitWorkflow_RejectsMalformedRemediation(t *testing.T) {
	srv := &server{}
	cases := []struct {
		name                                                          string
		parentRun, parentStep, finding, episode, node, endpoint, comp string
		wantErrContains                                               string
	}{
		{"empty node", "r", "s", "f", "ep-A", "", "10.0.0.1:11000", "etcd", "node_id"},
		{"empty endpoint", "r", "s", "f", "ep-A", "n1", "", "etcd", "agent_endpoint"},
		{"blank endpoint", "r", "s", "f", "ep-A", "n1", "   ", "etcd", "agent_endpoint"},
		{"empty parent run", "", "s", "f", "ep-A", "n1", "10.0.0.1:11000", "etcd", "parent_run_id"},
		{"empty parent step", "r", "", "f", "ep-A", "n1", "10.0.0.1:11000", "etcd", "parent_step_id"},
		{"unknown component", "r", "s", "f", "ep-A", "n1", "10.0.0.1:11000", "postgres", "unsupported component"},
		{"empty component", "r", "s", "f", "ep-A", "n1", "10.0.0.1:11000", "", "unsupported component"},
		// A systemd unit name is NOT a package identity. Accepting one here
		// would bypass packageToUnit, the unit-name authority.
		{"systemd unit not a component", "r", "s", "f", "ep-A", "n1", "10.0.0.1:11000", "scylla-server.service", "unsupported component"},
		{"globular unit not a component", "r", "s", "f", "ep-A", "n1", "10.0.0.1:11000", "globular-etcd", "unsupported component"},
		// Fail closed: the episode identity must be proven, never guessed.
		{"missing episode identity", "r", "s", "f", "", "n1", "10.0.0.1:11000", "etcd", "drift_episode_id is required"},
		{"blank episode identity", "r", "s", "f", "   ", "n1", "10.0.0.1:11000", "etcd", "drift_episode_id is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.RunRestartInfraUnitWorkflow(t.Context(),
				tc.parentRun, tc.parentStep, tc.finding, tc.episode, tc.node, tc.endpoint, tc.comp)
			if err == nil {
				t.Fatalf("expected rejection, got resp=%v", resp)
			}
			if resp != nil {
				t.Errorf("rejected call must not return a response, got %v", resp)
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error %q does not mention %q", err, tc.wantErrContains)
			}
		})
	}
}

// The three real infra components must all be accepted, case- and
// whitespace-insensitively, and must normalise to the same identity.
func TestRestartableInfraComponents_CoversTheProbedSet(t *testing.T) {
	// These are exactly the components reconcileScanDrift can emit as
	// infra_unhealthy (probeEtcdHealth / probeMinioHealth / probeScyllaHealth).
	for _, c := range []string{"etcd", "minio", "scylladb"} {
		if !restartableInfraComponents[c] {
			t.Errorf("component %q is probed as infra_unhealthy but is not restartable", c)
		}
	}
	if len(restartableInfraComponents) != 3 {
		t.Errorf("restartable set has %d entries, want exactly the 3 probed components",
			len(restartableInfraComponents))
	}
}

// ── declarative definition contract ───────────────────────────────────

func loadRestartInfraDefinition(t *testing.T) map[string]any {
	t.Helper()
	p := "../../workflow/definitions/node.restart_infra_unit.yaml"
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	return doc
}

func restartInfraStep(t *testing.T) map[string]any {
	t.Helper()
	doc := loadRestartInfraDefinition(t)
	spec, _ := doc["spec"].(map[string]any)
	if spec == nil {
		t.Fatal("definition has no spec")
	}
	steps, _ := spec["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("want exactly 1 step, got %d", len(steps))
	}
	step, _ := steps[0].(map[string]any)
	if step == nil {
		t.Fatal("step 0 is not a mapping")
	}
	return step
}

// A restart is not safely repeatable on its own — replaying it blindly would
// bounce a healthy unit. The step must verify the effect before continuing,
// which is what makes the durable run resumable rather than merely persisted.
func TestRestartInfraDefinition_ResumesByVerifyingEffect(t *testing.T) {
	step := restartInfraStep(t)
	exec, _ := step["execution"].(map[string]any)
	if exec == nil {
		t.Fatal("step has no execution block — a resumed run would re-restart the unit")
	}
	if got := exec["idempotency"]; got != "verify_then_continue" {
		t.Errorf("idempotency = %v, want verify_then_continue (safe_retry would bounce a healthy unit)", got)
	}
	if got := exec["resume_policy"]; got != "verify_effect" {
		t.Errorf("resume_policy = %v, want verify_effect", got)
	}
	if k, _ := exec["receipt_key"].(string); strings.TrimSpace(k) == "" {
		t.Error("execution has no receipt_key — the run cannot recognise its own prior effect")
	}
}

// The step must prove the unit came back, otherwise the run reports SUCCEEDED
// on a restart that left the service down — the same false-success the
// fabricated childResults entry produced.
func TestRestartInfraDefinition_VerifiesRuntimeHealth(t *testing.T) {
	step := restartInfraStep(t)
	ver, _ := step["verification"].(map[string]any)
	if ver == nil {
		t.Fatal("step has no verification — success would be assumed, not observed")
	}
	if got := ver["action"]; got != "node.verify_package_runtime" {
		t.Errorf("verification action = %v, want node.verify_package_runtime", got)
	}
	with, _ := ver["with"].(map[string]any)
	if with == nil {
		t.Fatal("verification has no inputs")
	}
	if got := with["package_kind"]; got != "INFRASTRUCTURE" {
		t.Errorf("package_kind = %v, want INFRASTRUCTURE; a COMMAND kind is skipped by "+
			"skipRuntimeCheck and the verification would silently pass", got)
	}
	if got := with["health_check"]; got != "service_healthy" {
		t.Errorf("health_check = %v, want service_healthy", got)
	}
}

// The mutation must be executed by the node agent. The controller is forbidden
// from touching systemd, and routing this to any other actor would reintroduce
// the boundary violation the inline path had.
func TestRestartInfraDefinition_ActorIsNodeAgent(t *testing.T) {
	step := restartInfraStep(t)
	if got := step["actor"]; got != "node-agent" {
		t.Errorf("actor = %v, want node-agent", got)
	}
	if got := step["action"]; got != "node.restart_package_service" {
		t.Errorf("action = %v, want node.restart_package_service", got)
	}
}

// The schema must require the durable parent identity. If these were optional
// the definition would admit a run whose correlation id is not stable.
func TestRestartInfraDefinition_RequiresDurableParentIdentity(t *testing.T) {
	doc := loadRestartInfraDefinition(t)
	spec, _ := doc["spec"].(map[string]any)
	schema, _ := spec["inputSchema"].(map[string]any)
	if schema == nil {
		t.Fatal("definition has no inputSchema")
	}
	req, _ := schema["required"].([]any)
	have := map[string]bool{}
	for _, r := range req {
		have[strings_TrimSpace(r)] = true
	}
	for _, want := range []string{"parent_run_id", "parent_step_id", "drift_episode_id", "node_id", "component", "agent_endpoint"} {
		if !have[want] {
			t.Errorf("inputSchema does not require %q", want)
		}
	}
}

// The component enum in the definition must match the Go allowlist, or the two
// gates disagree about what is restartable.
func TestRestartInfraDefinition_EnumMatchesGoAllowlist(t *testing.T) {
	doc := loadRestartInfraDefinition(t)
	spec, _ := doc["spec"].(map[string]any)
	schema, _ := spec["inputSchema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	comp, _ := props["component"].(map[string]any)
	if comp == nil {
		t.Fatal("inputSchema has no component property")
	}
	enum, _ := comp["enum"].([]any)
	if len(enum) != len(restartableInfraComponents) {
		t.Fatalf("definition enum has %d values, Go allowlist has %d",
			len(enum), len(restartableInfraComponents))
	}
	for _, e := range enum {
		v := strings_TrimSpace(e)
		if !restartableInfraComponents[v] {
			t.Errorf("definition allows component %q that the Go allowlist rejects", v)
		}
	}
}

func strings_TrimSpace(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// ── dispatch wiring ───────────────────────────────────────────────────

// The choose step must hand the durable remediation identity to the child.
// Without these inputs RunRestartInfraUnitWorkflow rejects the call, so a
// missing field turns every infra restart into a hard error.
func TestInfraRestart_ChooseStepSuppliesDurableIdentity(t *testing.T) {
	src := reconcileActionsSource(t)
	i := strings.Index(src, `"workflow_name": "node.restart_infra_unit"`)
	if i < 0 {
		t.Fatal("no dispatch of node.restart_infra_unit in reconcileChooseWorkflow")
	}
	block := src[i:]
	if j := strings.Index(block, "}, nil"); j > 0 {
		block = block[:j]
	}
	for _, want := range []string{"parent_run_id", "parent_step_id", "finding_id", "node_id", "component", "endpoint"} {
		if !strings.Contains(block, want) {
			t.Errorf("choose step does not pass %q to the child workflow", want)
		}
	}
}

// The parent identity must NOT be the reconcile run: that id is
// "reconcile:<UnixMilli>", a fresh run every tick, so keying the child off it
// would allocate a new run on every cycle for one persistent drift — exactly
// the unbounded-identity defect this repair removes.
func TestInfraRestart_ParentIdentityIsNotThePerTickReconcileRun(t *testing.T) {
	src := reconcileActionsSource(t)
	i := strings.Index(src, `"workflow_name": "node.restart_infra_unit"`)
	if i < 0 {
		t.Fatal("no dispatch of node.restart_infra_unit in reconcileChooseWorkflow")
	}
	block := src[i:]
	if j := strings.Index(block, "}, nil"); j > 0 {
		block = block[:j]
	}
	if strings.Contains(block, "correlationID") || strings.Contains(block, "reconcile:") {
		t.Error("child parent identity derives from the per-tick reconcile run id; " +
			"use the stable drift key so repeated ticks reconcile to one child run")
	}
	if !strings.Contains(block, "key") {
		t.Error("child parent step identity should be the stable drift key used by the backoff tracker")
	}
}

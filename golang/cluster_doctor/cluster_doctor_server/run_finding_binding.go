package main

import (
	"fmt"
	"sync"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
)

// Run-scoped finding bindings.
//
// The autonomous healer selects an exact rules.Finding during its enforcement
// cycle, then asks Workflow Service to run remediate.doctor.finding. The
// workflow later calls back into resolve_finding to obtain the finding it
// should act on.
//
// Without a binding that callback re-reads s.lastFindings, which is MUTABLE and
// republished by every cluster-wide evaluation. Between the healer's selection
// and the workflow's callback the cache can be replaced, so the workflow could
// resolve a DIFFERENT finding under the same id — or the same id carrying a
// different step, action, or subject — and then execute a mutation the healer
// never authorized. The gap is not hypothetical: the healer loop republishes
// lastFindings on every tick.
//
// The binding closes that window by making the finding an immutable input to
// the run rather than a lookup performed later. It is deliberately NOT a cache:
// it is keyed by the real workflow run id, is written exactly once before the
// run starts, is read during the run, and is removed when the run reaches a
// terminal state.
//
// Operator-started runs carry no binding and keep their existing behavior —
// they resolve from lastFindings as before, which is correct because an
// operator names a finding id and expects the current one.

// maxRunFindingBindings bounds the table so a pathological sequence of runs that
// never reach a terminal state cannot grow it without limit. The healer starts
// one run per dispatch and removes each on completion, so reaching this bound
// means bindings are leaking; refusing to start is preferable to consuming
// memory silently.
const maxRunFindingBindings = 256

// runFindingBinding is the immutable finding identity bound to one workflow run.
type runFindingBinding struct {
	finding     rules.Finding
	stepIndex   uint32
	findingID   string
	invariantID string
	entityRef   string
	nodeID      string
	actionType  cluster_doctorpb.ActionType
}

// runFindingBindings is the concurrency-safe table of run id → bound finding.
type runFindingBindings struct {
	mu sync.RWMutex
	m  map[string]*runFindingBinding
}

// bind records the exact finding for runID.
//
// Called BEFORE the run is started, so the resolve callback can never observe a
// run without its binding. Refuses to overwrite an existing binding: a run id is
// unique per attempt, so a second bind under one id means run identity has
// collapsed, which is exactly the aliasing that lets one attempt act on
// another's finding.
func (t *runFindingBindings) bind(runID string, f rules.Finding, stepIndex uint32) error {
	if runID == "" {
		return fmt.Errorf("bind run finding: empty run id")
	}
	if int(stepIndex) >= len(f.Remediation) {
		return fmt.Errorf("bind run finding: step_index %d out of range (finding %s has %d steps)",
			stepIndex, f.FindingID, len(f.Remediation))
	}
	action := f.Remediation[stepIndex].GetAction()
	if action == nil {
		return fmt.Errorf("bind run finding: finding %s step %d carries no structured action",
			f.FindingID, stepIndex)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.m == nil {
		t.m = make(map[string]*runFindingBinding)
	}
	if _, exists := t.m[runID]; exists {
		return fmt.Errorf("bind run finding: run %s already bound — run ids must be unique per attempt", runID)
	}
	if len(t.m) >= maxRunFindingBindings {
		return fmt.Errorf("bind run finding: %d bindings outstanding (max %d) — refusing to start; "+
			"bindings are leaking because runs are not reaching a terminal state",
			len(t.m), maxRunFindingBindings)
	}
	t.m[runID] = &runFindingBinding{
		finding:     f,
		stepIndex:   stepIndex,
		findingID:   f.FindingID,
		invariantID: f.InvariantID,
		entityRef:   f.EntityRef,
		nodeID:      action.GetParams()["node_id"],
		actionType:  action.GetActionType(),
	}
	return nil
}

// lookup returns the binding for runID, if this run is autonomous.
//
// The bool distinguishes "no binding, so this is an operator-started run" from
// "bound to something else". Only the caller can tell those apart, and
// collapsing them would let an autonomous run silently fall back to
// lastFindings — the exact substitution the binding exists to prevent.
func (t *runFindingBindings) lookup(runID string) (*runFindingBinding, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	b, ok := t.m[runID]
	return b, ok
}

// release removes the binding for runID.
//
// Called from a defer on the dispatch path so it runs on every terminal outcome
// — success, failure, refusal, context cancellation, or panic unwinding. A
// binding that outlived its run would both leak and, if a run id were ever
// reused, offer a stale finding to a later attempt.
func (t *runFindingBindings) release(runID string) {
	if runID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.m, runID)
}

// outstanding reports how many bindings are currently held. Test and diagnostic
// use only: a non-zero count after all runs have completed is a leak.
func (t *runFindingBindings) outstanding() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.m)
}

// matches reports whether the workflow asked for the finding this run was bound
// to. A mismatch is a hard error, never a fallback: it means the workflow is
// acting on a different subject than the healer authorized.
func (b *runFindingBinding) matches(findingID string, stepIndex uint32) error {
	if b.findingID != findingID {
		return fmt.Errorf("bound finding %s does not match requested %s", b.findingID, findingID)
	}
	if b.stepIndex != stepIndex {
		return fmt.Errorf("bound step_index %d does not match requested %d", b.stepIndex, stepIndex)
	}
	return nil
}

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"github.com/globulario/services/golang/workflow/engine"
)

// Binding-mode consistency proof.
//
// The two resolution contracts are MUTUALLY EXCLUSIVE. Autonomous runs must use
// their bound finding and never the cache; operator runs must use the cache and
// never coexist with a binding. The second half is the downgrade guard: a mode
// that is lost, corrupted, or wrongly propagated arrives as operator_current or
// empty, and without the guard it would silently demote an autonomous run into
// the contract that reads a mutable cache.

func bindingModeServer(t *testing.T, cached ...rules.Finding) *ClusterDoctorServer {
	t.Helper()
	srv := &ClusterDoctorServer{
		clusterID:    "c1",
		cfg:          &clusterdoctorConfig{Port: 10300},
		lastFindings: cached,
	}
	srv.isAuthoritative.Store(true)
	return srv
}

// resolveVia exercises the production ResolveFinding callback exactly as the
// workflow actor invokes it.
func resolveVia(t *testing.T, srv *ClusterDoctorServer, runID, findingID string, step uint32, mode string) (*engine.ResolvedFinding, error) {
	t.Helper()
	cfg := srv.buildDoctorRemediationConfig()
	return cfg.ResolveFinding(context.Background(), runID, findingID, step, mode)
}

func TestResolveFinding_BindingModeMatrix(t *testing.T) {
	bound := autonomousFinding()

	for _, tc := range []struct {
		name       string
		bind       bool
		mode       string
		findingID  string
		step       uint32
		wantErr    string
		wantEntity string
		why        string
	}{
		{
			name: "autonomous with matching binding resolves the bound finding",
			bind: true, mode: engine.FindingBindingAutonomousRequired,
			findingID: bound.FindingID, step: 0,
			wantEntity: bound.EntityRef,
			why:        "the healer's exact selection must be what executes",
		},
		{
			name: "autonomous without binding fails closed",
			bind: false, mode: engine.FindingBindingAutonomousRequired,
			findingID: bound.FindingID, step: 0,
			wantErr: "no run-scoped",
			why: "absence has many causes — cleanup, cancellation, timeout, a bug — " +
				"and none of them licenses reading the mutable cache",
		},
		{
			name: "autonomous with wrong finding id fails closed",
			bind: true, mode: engine.FindingBindingAutonomousRequired,
			findingID: "f-different", step: 0,
			wantErr: "bound to a different subject",
			why:     "acting on another subject than the one authorized is the worst outcome",
		},
		{
			name: "autonomous with wrong step index fails closed",
			bind: true, mode: engine.FindingBindingAutonomousRequired,
			findingID: bound.FindingID, step: 7,
			wantErr: "bound to a different subject",
			why:     "a different step is a different action",
		},
		{
			name: "operator without binding resolves the current cached finding",
			bind: false, mode: engine.FindingBindingOperatorCurrent,
			findingID: bound.FindingID, step: 0,
			wantEntity: bound.EntityRef,
			why:        "an operator names an id and expects the CURRENT finding",
		},
		{
			name: "operator WITH binding fails closed — downgrade rejected",
			bind: true, mode: engine.FindingBindingOperatorCurrent,
			findingID: bound.FindingID, step: 0,
			wantErr: "refusing to downgrade",
			why:     "a lost or corrupted mode must not demote a bound autonomous run",
		},
		{
			name: "empty mode without binding stays backward compatible",
			bind: false, mode: "",
			findingID: bound.FindingID, step: 0,
			wantEntity: bound.EntityRef,
			why:        "genuine legacy operator callers keep working",
		},
		{
			name: "empty mode WITH binding fails closed",
			bind: true, mode: "",
			findingID: bound.FindingID, step: 0,
			wantErr: "refusing to downgrade",
			why:     "an absent mode must not be a way past an existing binding",
		},
		{
			name: "unknown mode fails closed",
			bind: false, mode: "something_else",
			findingID: bound.FindingID, step: 0,
			wantErr: "unknown finding_binding_mode",
			why:     "guessing which contract applies is never safe",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The cache always holds the finding, so a fallback would SUCCEED
			// rather than error — that is what makes the fail-closed assertions
			// meaningful instead of vacuous.
			srv := bindingModeServer(t, bound)
			const runID = "run-mode"
			if tc.bind {
				if err := srv.runFindings.bind(runID, bound, 0); err != nil {
					t.Fatalf("bind: %v", err)
				}
			}

			got, err := resolveVia(t, srv, runID, tc.findingID, tc.step, tc.mode)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected failure containing %q, got success (%+v) — %s", tc.wantErr, got, tc.why)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %v, want it to contain %q — %s", err, tc.wantErr, tc.why)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v — %s", err, tc.why)
			}
			if got.EntityRef != tc.wantEntity {
				t.Errorf("entity_ref = %q, want %q — %s", got.EntityRef, tc.wantEntity, tc.why)
			}
		})
	}
}

// TestResolveFinding_LateCallbackAfterCleanupFailsClosed proves the race the
// binding mode exists to survive: a callback that arrives after the dispatch
// returned and released the binding.
//
// Without the explicit mode this would find no binding, conclude "operator
// run", and resolve whatever the cache holds — which by then may be a
// different finding entirely.
func TestResolveFinding_LateCallbackAfterCleanupFailsClosed(t *testing.T) {
	bound := autonomousFinding()
	srv := bindingModeServer(t, bound)
	const runID = "run-late"

	if err := srv.runFindings.bind(runID, bound, 0); err != nil {
		t.Fatalf("bind: %v", err)
	}
	srv.runFindings.release(runID) // dispatch returned; binding cleaned up

	_, err := resolveVia(t, srv, runID, bound.FindingID, 0, engine.FindingBindingAutonomousRequired)
	if err == nil {
		t.Fatal("a late callback for a cleaned-up autonomous run must fail closed, " +
			"never fall through to the mutable cache")
	}
}

// TestResolveFinding_LastFindingsMutationCannotChangeBoundRun proves the
// original motivation for the binding: the healer loop republishes lastFindings
// every tick, so between selection and callback the cache can hold a DIFFERENT
// finding under the same id.
func TestResolveFinding_LastFindingsMutationCannotChangeBoundRun(t *testing.T) {
	bound := autonomousFinding()
	srv := bindingModeServer(t, bound)
	const runID = "run-mutate"

	if err := srv.runFindings.bind(runID, bound, 0); err != nil {
		t.Fatalf("bind: %v", err)
	}

	// Same finding id, different subject and action — exactly what a later
	// evaluation cycle could publish.
	impostor := autonomousFinding()
	impostor.EntityRef = "node-9/globular-attacker.service"
	impostor.Remediation[0].Action.Params = map[string]string{
		"node_id": "node-9", "unit": "globular-attacker.service",
	}
	srv.lastFindingsMu.Lock()
	srv.lastFindings = []rules.Finding{impostor}
	srv.lastFindingsMu.Unlock()

	got, err := resolveVia(t, srv, runID, bound.FindingID, 0, engine.FindingBindingAutonomousRequired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EntityRef != bound.EntityRef {
		t.Errorf("entity_ref = %q, want %q — a cache mutation after dispatch must not change "+
			"the subject the bound run acts on", got.EntityRef, bound.EntityRef)
	}
	if got.NodeID != "node-1" {
		t.Errorf("node_id = %q, want node-1 — the action would have run on the wrong node", got.NodeID)
	}
}

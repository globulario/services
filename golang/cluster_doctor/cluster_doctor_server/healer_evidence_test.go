package main

import (
	"go/ast"
	"go/parser"
	"go/token"

	"testing"
)

// The governed gate authorizes a remediation only when qualifying evidence
// already exists for the finding: sat.doctor.finding_observed wants an evidence
// row of kind cluster_doctor_evidence naming the same invariant and entity.
//
// Until 2026-08-01 the only producer of that row was emitBehavioralClusterReport,
// reached from the GetClusterReport RPC — while the consumer was the healer
// cycle. An autonomous healer therefore gated on evidence it never emitted:
// CheckAction returned needs_evidence and the repair silently never happened.
// An operator who happened to pull a cluster report first made the very same
// action succeed, which is the kind of coupling that makes a bug look like a
// flake.
//
// These tests assert the producer and the consumer stay on the same path.
//
// They read the source rather than running a cycle: runHealerCycle needs a
// collector snapshot, an invariant registry, a leader election and an audit
// store, so exercising it here would test the harness more than the coupling.
// The property that matters is structural — "the healer emits before it
// dispatches" — and that is exactly what the AST shows.

func healerLoopFunc(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "healer_loop.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse healer_loop.go: %v", err)
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("func %s not found in healer_loop.go", name)
	return nil
}

// callOrder returns a monotonic position for the first call whose selector name
// is exactly want, or -1 when absent.
//
// Exact, not substring: runHealerCycle calls registry.EvaluateAll BEFORE the
// emit and healer.Evaluate after it, so a substring match on "Evaluate" finds
// the wrong call and the ordering assertion silently inverts.
func callOrder(fn *ast.FuncDecl, want string) int {
	idx, found, n := -1, false, 0
	ast.Inspect(fn, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		n++
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && !found {
			if sel.Sel.Name == want {
				idx, found = n, true
			}
		}
		return true
	})
	return idx
}

// TestHealerCycle_EmitsBehavioralEvidence is the coupling guard: the healer must
// produce the evidence its own gate will require.
func TestHealerCycle_EmitsBehavioralEvidence(t *testing.T) {
	fn := healerLoopFunc(t, "runHealerCycle")

	if callOrder(fn, "emitBehavioralClusterReport") < 0 {
		t.Fatal("runHealerCycle does not emit behavioral finding evidence.\n" +
			"The governed gate requires cluster_doctor_evidence for the finding before it\n" +
			"will authorize a remediation. Without this the autonomous healer gates on\n" +
			"evidence nobody produced: CheckAction returns needs_evidence and the repair\n" +
			"never happens, while the same action succeeds for an operator who pulled a\n" +
			"cluster report first.")
	}
}

// TestHealerCycle_EmitsEvidenceBeforeDispatch pins the ordering.
//
// Evidence recorded after Evaluate cannot inform the gate that Evaluate just
// consulted. Emitting first also hands the recorder the whole dispatch window to
// drain, since delivery is non-blocking by contract.
func TestHealerCycle_EmitsEvidenceBeforeDispatch(t *testing.T) {
	fn := healerLoopFunc(t, "runHealerCycle")

	emit := callOrder(fn, "emitBehavioralClusterReport")
	evaluate := callOrder(fn, "Evaluate")

	if emit < 0 || evaluate < 0 {
		t.Fatalf("expected both emitBehavioralClusterReport and Evaluate in runHealerCycle "+
			"(emit=%d evaluate=%d)", emit, evaluate)
	}
	if emit > evaluate {
		t.Error("runHealerCycle emits behavioral evidence AFTER healer.Evaluate.\n" +
			"Evidence recorded after dispatch cannot authorize the dispatch that just ran.")
	}
}

// TestHealerCycle_SkipsEvidenceOnReducedHarvest guards the safety coupling.
//
// The cycle already downgrades enforce→observe on a partial snapshot because
// findings may be false positives (doctor.healer_auto_remediation_on_reduced_harvest).
// Evidence is authorization, and it outlives the cycle that produced it: minting
// it from a snapshot we refuse to act on would launder a possibly-false finding
// into standing authorization for a later cycle that IS trusted.
func TestHealerCycle_SkipsEvidenceOnReducedHarvest(t *testing.T) {
	fn := healerLoopFunc(t, "runHealerCycle")

	var guarded bool
	ast.Inspect(fn, func(node ast.Node) bool {
		ifs, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		var condHasHarvest bool
		ast.Inspect(ifs.Cond, func(c ast.Node) bool {
			if sel, ok := c.(*ast.SelectorExpr); ok && sel.Sel.Name == "DataIncomplete" {
				condHasHarvest = true
			}
			return true
		})
		if !condHasHarvest {
			return true
		}
		ast.Inspect(ifs.Body, func(b ast.Node) bool {
			if call, ok := b.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok &&
					sel.Sel.Name == "emitBehavioralClusterReport" {
					guarded = true
				}
			}
			return true
		})
		return true
	})

	if !guarded {
		t.Error("behavioral evidence emission in runHealerCycle is not gated on snapshot harvest.\n" +
			"A reduced-harvest snapshot may contain false-positive findings; recording them as\n" +
			"cluster_doctor_evidence would authorize a future remediation on a problem that was\n" +
			"never real.")
	}
}

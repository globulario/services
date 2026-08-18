package evolution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const (
	// ratchetExemptionMarker names a failure path that genuinely cannot withdraw,
	// because the withdrawal channel is the thing that failed.
	ratchetExemptionMarker = "ratchet:no-withdrawal-possible"
	// Capped deliberately: exemptions are the pressure valve, and an uncapped
	// valve is how the rule quietly stops applying.
	maxRatchetExemptions = 1
	// Bare failure returns above the identity, all of them precondition checks
	// that cannot have superseded anything yet. Frozen so new work cannot be
	// quietly added above the boundary.
	expectedPreconditionReturns = 6
)

// TestEveryFailurePathAfterTheHelperWithdraws is a structural ratchet, not a
// behavioural test.
//
// Twice now a change has been inserted into RunDeclaredTest above the point
// where its withdrawal helper is defined, so the new early return could not
// withdraw the standing claim and a rerun left a superseded PASS — and possibly
// PROVEN — in place. Both times the code was correct in isolation and wrong only
// in its position, which is invisible in review and cheap to repeat.
//
// So the rule is enforced on the shape of the function: once `fail` exists,
// every failing return goes through it. A bare `return TestRunResult{}, err`
// below that point is the exact shape of the defect.
func TestEveryFailurePathAfterTheHelperWithdraws(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test_runner.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var fn *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if decl, ok := n.(*ast.FuncDecl); ok && decl.Name.Name == "RunDeclaredTest" {
			fn = decl
			return false
		}
		return true
	})
	if fn == nil {
		t.Fatal("RunDeclaredTest not found; this ratchet must be re-pointed, not deleted")
	}

	// Where the withdrawal helper comes into existence, and the span of its own
	// body — the helper necessarily returns directly, and must not flag itself.
	//
	// identityPos is the real boundary. From the moment the envelope's identity
	// is taken, this run has committed to superseding whatever claim stands, so
	// every failure from there must withdraw. Anchoring on the helper instead
	// left the pre-helper region unguarded — which is exactly where both prior
	// regressions were inserted.
	helperPos := token.NoPos
	identityPos := token.NoPos
	helperStart, helperEnd := token.NoPos, token.NoPos
	ast.Inspect(fn, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 {
			return true
		}
		if ident, ok := assign.Lhs[0].(*ast.Ident); ok && ident.Name == "identity" && !identityPos.IsValid() {
			identityPos = assign.Pos()
		}
		if ident, ok := assign.Lhs[0].(*ast.Ident); ok && ident.Name == "fail" {
			helperPos = assign.Pos()
			if len(assign.Rhs) == 1 {
				if lit, ok := assign.Rhs[0].(*ast.FuncLit); ok {
					helperStart, helperEnd = lit.Body.Pos(), lit.Body.End()
				}
			}
			return false
		}
		return true
	})
	if !helperPos.IsValid() {
		t.Fatal("RunDeclaredTest no longer defines a `fail` withdrawal helper")
	}
	if !identityPos.IsValid() {
		t.Fatal("RunDeclaredTest no longer takes the envelope identity; this ratchet must be re-pointed")
	}
	if identityPos > helperPos {
		t.Fatal("the withdrawal helper is declared before the identity it withdraws with")
	}

	// Nothing may be inserted between taking the identity and declaring the
	// helper. Both prior regressions were work slipped into exactly that gap,
	// where a failure can supersede a claim but cannot yet withdraw it.
	for _, stmt := range fn.Body.List {
		if stmt.Pos() > identityPos && stmt.End() <= helperPos {
			t.Fatalf(
				"statement at %s sits between the identity and the withdrawal helper; "+
					"a failure there supersedes a standing claim without being able to withdraw it",
				fset.Position(stmt.Pos()),
			)
		}
	}

	// An explicit, greppable exemption. A failure of the durable mutation itself
	// cannot be withdrawn through that same mutation, so those returns carry a
	// marker and are counted rather than hidden.
	exempt := map[int]bool{}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.Contains(c.Text, ratchetExemptionMarker) {
				exempt[fset.Position(c.Pos()).Line] = true
			}
		}
	}

	// The superseding operation itself must sit below the helper. A region rule
	// alone cannot hold: both regressions were evaded by moving work *above* the
	// boundary, so the operation is pinned rather than the neighbourhood.
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != "newInvocationID" {
			return true
		}
		if call.Pos() < helperPos {
			t.Errorf(
				"newInvocationID is called at %s, above the withdrawal helper; "+
					"minting an invocation commits this run to superseding the standing claim, "+
					"so its failure must be able to withdraw",
				fset.Position(call.Pos()),
			)
		}
		return true
	})

	// Everything above the identity is precondition checking, which legitimately
	// returns without withdrawing because no claim has been superseded yet. That
	// region is frozen by count: a new bare failure return there is how work gets
	// smuggled in above the boundary, and it must be justified rather than added.
	preconditionReturns := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || ret.Pos() >= identityPos || len(ret.Results) != 2 {
			return true
		}
		if lit, ok := ret.Results[0].(*ast.CompositeLit); ok && len(lit.Elts) == 0 {
			preconditionReturns++
		}
		return true
	})
	if preconditionReturns != expectedPreconditionReturns {
		t.Fatalf(
			"precondition region has %d bare failure returns, expected %d; "+
				"if a new one is genuinely a precondition update the constant, "+
				"but if it can supersede a standing claim it belongs below the withdrawal helper",
			preconditionReturns, expectedPreconditionReturns,
		)
	}

	var bypasses []string
	exemptions := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || ret.Pos() < identityPos || len(ret.Results) != 2 {
			return true
		}
		// The helper's own body returns directly by construction.
		if helperStart.IsValid() && ret.Pos() >= helperStart && ret.Pos() < helperEnd {
			return true
		}
		// A failing return is `TestRunResult{}, <something>` — the zero value
		// paired with an error. The success return carries populated fields.
		lit, ok := ret.Results[0].(*ast.CompositeLit)
		if !ok || len(lit.Elts) != 0 {
			return true
		}
		if ident, ok := lit.Type.(*ast.Ident); !ok || ident.Name != "TestRunResult" {
			return true
		}
		// Returning a nil error with a zero result is not a failure path.
		if ident, ok := ret.Results[1].(*ast.Ident); ok && ident.Name == "nil" {
			return true
		}
		// The marker sits in the comment block explaining the return, which may be
		// a few lines above it.
		line := fset.Position(ret.Pos()).Line
		for back := 0; back <= 8; back++ {
			if exempt[line-back] {
				exemptions++
				return true
			}
		}
		bypasses = append(bypasses, fset.Position(ret.Pos()).String())
		return true
	})
	if exemptions > maxRatchetExemptions {
		t.Fatalf(
			"%d exempted failure paths, expected at most %d; a growing exemption list means the rule is being routed around",
			exemptions, maxRatchetExemptions,
		)
	}

	if len(bypasses) > 0 {
		t.Fatalf(
			"failure paths below the withdrawal helper return without withdrawing the standing claim: %v\n"+
				"route them through fail(...), or move them above the helper if they genuinely cannot supersede a prior run",
			bypasses,
		)
	}
}

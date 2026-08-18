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
	helperPos := token.NoPos
	helperStart, helperEnd := token.NoPos, token.NoPos
	ast.Inspect(fn, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 {
			return true
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

	var bypasses []string
	exemptions := 0
	ast.Inspect(fn, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || ret.Pos() < helperPos || len(ret.Results) != 2 {
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

// uncertainty-scan finds places where authority uncertainty is collapsed into a
// benign-looking zero value at a verdict boundary.
//
// The shape it looks for, from the fail-open audit of 2026-08-14:
//
//	producer:  error means "authority information unavailable"  (preserved)
//	consumer:  log a warning
//	           x = zeroValue
//	           continue into Authorize / Evaluate / Verify / Admit / Resolve / Certify
//
// The producer usually gets this right, often with a comment explaining why the
// error detail matters. The consumer erases it one call later, and after that no
// downstream code can tell "there is nothing to check" from "I could not find
// out what to check".
//
// # This tool REPORTS CANDIDATES. It does not fail CI by default.
//
// That is deliberate and should stay that way until a labelled corpus exists.
// There are legitimate cases where an empty value IS the declared fail-safe
// behaviour, and a scanner that cannot tell them apart would push people to
// "fix" correct code. Use -strict only for rule classes proven high-confidence
// against real findings.
//
// # What it deliberately does NOT flag
//
// Absence of a LOCAL error guard is not the test. Correctness may be centralised
// somewhere proven. cluster-doctor is the reference example: 38 of 61 rules
// carry no local snap.HadError guard, by an accepted decision
// (docs/awareness/decisions/doctor-btail-guards-are-legibility-not-correctness.md),
// because snapshotSourceUnavailableFindings emits INVARIANT_UNKNOWN for every
// errored source and a ratchet test forbids confident failures from an errored
// snapshot. Mechanically adding guards there would damage legitimate
// partial-snapshot reasoning. So this tool looks for an ERASURE, never for a
// missing guard.
//
// Usage:
//
//	go run ./tools/uncertainty-scan -root ./
//	go run ./tools/uncertainty-scan -root ./ -json
//	go run ./tools/uncertainty-scan -root ./ -strict   # exit 1 on findings
//
// Opt-out for a declared fail-safe, on the line before the if or inside it:
//
//	//go:uncertainty:declared-failsafe <reason the empty value is correct here>
//
// The reason is required. An opt-out with no justification is itself reported,
// because "someone silenced this once" is not evidence that it is safe.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const optOutPragma = "//go:uncertainty:declared-failsafe"

// sinkSubstrings are calls that consume the collapsed value to reach a verdict.
// Matched case-insensitively against the callee name. Kept deliberately narrow:
// the sink requirement is what separates "logged and carried on" (usually fine)
// from "logged and then decided something" (the actual defect).
var sinkSubstrings = []string{
	"authorize", "authoriz",
	"validateaction", "validateaccess", "validatesubject",
	"evaluate", "verify", "admit", "resolve", "certify",
	"policydecision", "signaturepolicy",
	"checkpermission", "haspermission", "isallowed", "candoaction",
	"decide", "decision",
}

type Finding struct {
	Rule    string `json:"rule"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Func    string `json:"func"`
	Var     string `json:"var,omitempty"`
	Sink    string `json:"sink,omitempty"`
	Message string `json:"message"`
}

func main() {
	root := flag.String("root", "./", "root directory to scan")
	asJSON := flag.Bool("json", false, "emit findings as JSON")
	strict := flag.Bool("strict", false, "exit non-zero when findings exist (default: report only)")
	includeTests := flag.Bool("include-tests", false, "scan _test.go files too")
	flag.Parse()

	findings, err := scan(*root, *includeTests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uncertainty-scan: %v\n", err)
		os.Exit(2)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(findings)
	} else {
		report(findings)
	}

	if *strict && len(findings) > 0 {
		os.Exit(1)
	}
}

func report(findings []Finding) {
	if len(findings) == 0 {
		fmt.Println("uncertainty-scan: no candidates found")
		fmt.Println("  NOTE: zero findings is not proof of safety — this tool detects one shape")
		fmt.Println("  (error -> zero value -> verdict). It does not audit centralised fail-safes.")
		return
	}
	byRule := map[string]int{}
	for _, f := range findings {
		byRule[f.Rule]++
		fmt.Printf("%s:%d  [%s]\n", f.File, f.Line, f.Rule)
		fmt.Printf("    in %s: %s\n", f.Func, f.Message)
		if f.Sink != "" {
			fmt.Printf("    reaches verdict call: %s\n", f.Sink)
		}
	}
	fmt.Printf("\n%d candidate(s):\n", len(findings))
	rules := make([]string, 0, len(byRule))
	for r := range byRule {
		rules = append(rules, r)
	}
	sort.Strings(rules)
	for _, r := range rules {
		fmt.Printf("  %-24s %d\n", r, byRule[r])
	}
	fmt.Println("\nThese are CANDIDATES, not confirmed defects. Each needs a human decision:")
	fmt.Println("  - real erasure           -> make the uncertainty reach the decision")
	fmt.Println("  - declared fail-safe     -> annotate with " + optOutPragma + " <reason>")
	fmt.Println("  - centralised elsewhere  -> verify the central mechanism, then annotate")
}

func scan(root string, includeTests bool) ([]Finding, error) {
	var out []Finding
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // unreadable paths are skipped, not fatal
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", "dist", "node_modules", ".git", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.HasSuffix(path, ".pb.go") || strings.Contains(path, "_generated.go") {
			return nil // generated code is not authored policy
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil // unparseable file: skip rather than abort the sweep
		}
		out = append(out, scanFile(fset, path, f)...)
		return nil
	})
	return out, err
}

func scanFile(fset *token.FileSet, path string, file *ast.File) []Finding {
	var findings []Finding
	optOuts := collectOptOuts(fset, file)

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		fname := fn.Name.Name
		stmts := fn.Body.List
		for i, st := range stmts {
			ifs, ok := st.(*ast.IfStmt)
			if !ok || !isErrCheck(ifs.Cond) {
				continue
			}
			line := fset.Position(ifs.Pos()).Line
			if reason, silenced := optOuts[line]; silenced {
				if strings.TrimSpace(reason) == "" {
					findings = append(findings, Finding{
						Rule: "unjustified-optout", File: path, Line: line, Func: fname,
						Message: "opt-out pragma with no stated reason; an unexplained silence is not evidence of safety",
					})
				}
				continue
			}

			// RULE B — error erased at the return boundary:
			//   if err != nil { return nil, nil }
			// The caller cannot distinguish failure from an empty result.
			if v, ok := returnsZeroAndNilError(ifs.Body); ok {
				findings = append(findings, Finding{
					Rule: "error-erased-on-return", File: path, Line: line, Func: fname, Var: v,
					Message: "error branch returns a zero value AND a nil error, so callers cannot distinguish failure from an empty result",
				})
				continue
			}

			// RULE A — uncertainty collapsed, then a verdict is reached:
			//   if err != nil { log(...); x = zeroValue }  ... Authorize(x)
			if terminates(ifs.Body) {
				continue // the branch stops; nothing flows onward
			}
			assigned := zeroAssignments(ifs.Body)
			if len(assigned) == 0 {
				continue
			}
			sink := findSink(stmts[i+1:])
			if sink == "" {
				continue // logged and carried on, but no verdict downstream
			}
			findings = append(findings, Finding{
				Rule: "uncertainty-collapsed-to-zero", File: path, Line: line, Func: fname,
				Var: strings.Join(assigned, ", "), Sink: sink,
				Message: "error is collapsed into a zero value and execution continues into a verdict",
			})
		}
		return true
	})
	return findings
}

// collectOptOuts maps the line of the guarded if-statement to the stated reason.
func collectOptOuts(fset *token.FileSet, file *ast.File) map[int]string {
	out := map[int]string{}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			txt := strings.TrimSpace(c.Text)
			if !strings.HasPrefix(txt, optOutPragma) {
				continue
			}
			reason := strings.TrimSpace(strings.TrimPrefix(txt, optOutPragma))
			// Applies to the next line (the if) and its own line.
			ln := fset.Position(c.Pos()).Line
			out[ln] = reason
			out[ln+1] = reason
		}
	}
	return out
}

// isErrCheck matches `err != nil` and `x.err != nil` style conditions.
func isErrCheck(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	if id, ok := bin.Y.(*ast.Ident); !ok || id.Name != "nil" {
		return false
	}
	name := exprName(bin.X)
	return strings.HasSuffix(strings.ToLower(name), "err") || strings.Contains(strings.ToLower(name), "err")
}

// terminates reports whether the block ends control flow rather than falling through.
func terminates(b *ast.BlockStmt) bool {
	if b == nil || len(b.List) == 0 {
		return false
	}
	switch s := b.List[len(b.List)-1].(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			n := strings.ToLower(exprName(call.Fun))
			return strings.Contains(n, "panic") || strings.Contains(n, "fatal") || strings.Contains(n, "exit")
		}
	}
	return false
}

// returnsZeroAndNilError detects `return <zero>, nil` inside an error branch.
func returnsZeroAndNilError(b *ast.BlockStmt) (string, bool) {
	if b == nil || len(b.List) == 0 {
		return "", false
	}
	ret, ok := b.List[len(b.List)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) < 2 {
		return "", false
	}
	last := ret.Results[len(ret.Results)-1]
	if id, ok := last.(*ast.Ident); !ok || id.Name != "nil" {
		return "", false // the error slot is not nil: the error is propagated
	}
	for _, r := range ret.Results[:len(ret.Results)-1] {
		if isZeroValue(r) {
			return exprName(r), true
		}
	}
	return "", false
}

// zeroAssignments returns names assigned a zero value inside the branch.
func zeroAssignments(b *ast.BlockStmt) []string {
	var names []string
	ast.Inspect(b, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range as.Rhs {
			if i < len(as.Lhs) && isZeroValue(rhs) {
				names = append(names, exprName(as.Lhs[i]))
			}
		}
		return true
	})
	return names
}

// isZeroValue recognises the benign-looking values uncertainty gets collapsed into.
func isZeroValue(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		// NOTE: `false` is deliberately NOT a collapse.
		//
		// Calibrated against real findings on 2026-08-14. Assigning false in an
		// error branch is almost always the CORRECT propagation of failure —
		// `inventoryComplete = false` (node_agent/heartbeat.go:1715) and
		// `result.Success = false` (mcp/governor.go:417) both mark the failure
		// honestly. That is fail-CLOSED: the exact opposite of the defect this
		// tool hunts. Treating it as a collapse inverted the polarity and made
		// two correct fail-safes look like violations.
		//
		// Residual gap, accepted knowingly: a NEGATIVELY named flag
		// (isRestricted = false, denied = false) would be a genuine fail-open
		// and is not detected. Narrow, and preferable to reporting every
		// correct failure propagation in the codebase.
		return v.Name == "nil"
	case *ast.BasicLit:
		return v.Value == `""` || v.Value == "0"
	case *ast.CompositeLit:
		return len(v.Elts) == 0 // []T{}, map[K]V{}, T{}
	case *ast.CallExpr:
		name := exprName(v.Fun)
		// make(T, 0) — an explicitly empty collection.
		if name == "make" {
			if len(v.Args) >= 2 {
				if lit, ok := v.Args[1].(*ast.BasicLit); ok && lit.Value == "0" {
					return true
				}
			}
			return false
		}
		// defaultX() / emptyX() — a named stand-in for "nothing known".
		low := strings.ToLower(name)
		return strings.HasPrefix(low, "default") || strings.HasPrefix(low, "empty")
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			return isZeroValue(v.X)
		}
	}
	return false
}

// findSink reports the first verdict-reaching call in the statements that follow.
func findSink(rest []ast.Stmt) string {
	found := ""
	for _, st := range rest {
		ast.Inspect(st, func(n ast.Node) bool {
			if found != "" {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := exprName(call.Fun)
			low := strings.ToLower(name)
			for _, s := range sinkSubstrings {
				if strings.Contains(low, s) {
					found = name
					return false
				}
			}
			return true
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func exprName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprName(v.X) + "." + v.Sel.Name
	case *ast.CallExpr:
		return exprName(v.Fun)
	case *ast.IndexExpr:
		return exprName(v.X)
	}
	return ""
}

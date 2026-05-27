package intentaudit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ASTFinding is a single finding from AST-based violation scanning.
// Source is always "ast" to distinguish from grep-based findings.
type ASTFinding struct {
	IntentID string `json:"intent_id" yaml:"intent_id"`
	Pattern  string `json:"pattern" yaml:"pattern"`
	File     string `json:"file" yaml:"file"`
	Line     int    `json:"line" yaml:"line"`
	Source   string `json:"source" yaml:"source"` // always "ast"
}

// ScanGoAST walks Go files under srcDir and reports AST-level findings
// for known violation patterns (os.Getenv, exec.Command, exec.CommandContext).
// Files matching accepted exceptions are suppressed.
func ScanGoAST(srcDir string, exceptions []Exception) ([]ASTFinding, error) {
	var findings []ASTFinding

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Skip generated code.
		if strings.HasSuffix(path, ".pb.go") || strings.HasSuffix(path, "_generated.go") {
			return nil
		}

		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			rel = path
		}

		// Skip the intentaudit package itself (it references patterns as strings).
		if isIntentAuditPackage(rel) {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil // skip unparseable files
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			pkg := ident.Name
			fn := sel.Sel.Name
			pos := fset.Position(call.Pos())

			switch {
			case pkg == "os" && fn == "Getenv":
				if !isExcepted(rel, exceptions) {
					findings = append(findings, ASTFinding{
						IntentID: "etcd.is_source_of_truth",
						Pattern:  "os.Getenv",
						File:     rel,
						Line:     pos.Line,
						Source:   "ast",
					})
				}

			case pkg == "exec" && (fn == "Command" || fn == "CommandContext"):
				// node_agent is allowed to exec.
				if isNodeAgentPath(rel) {
					return true
				}
				if !isExcepted(rel, exceptions) {
					findings = append(findings, ASTFinding{
						IntentID: "controller.decides_but_does_not_execute_leaf_work",
						Pattern:  "exec." + fn,
						File:     rel,
						Line:     pos.Line,
						Source:   "ast",
					})
					findings = append(findings, ASTFinding{
						IntentID: "workflow.source_of_operational_truth",
						Pattern:  "exec." + fn,
						File:     rel,
						Line:     pos.Line,
						Source:   "ast",
					})
				}
			}

			return true
		})

		return nil
	})

	return findings, err
}

// collectAllExceptions gathers exceptions from all loaded intent nodes.
func collectAllExceptions(nodes map[string]*Node) []Exception {
	var all []Exception
	for _, n := range nodes {
		all = append(all, n.Exceptions...)
	}
	return all
}

// isIntentAuditPackage returns true if the relative path is inside the
// intentaudit package directory.
func isIntentAuditPackage(rel string) bool {
	norm := filepath.ToSlash(rel)
	return strings.HasPrefix(norm, "awareness/intentaudit/") ||
		strings.HasPrefix(norm, "awareness/intentaudit")
}

// isNodeAgentPath returns true if the relative path is inside node_agent/.
func isNodeAgentPath(rel string) bool {
	norm := filepath.ToSlash(rel)
	return strings.HasPrefix(norm, "node_agent/") ||
		strings.HasPrefix(norm, "node_agent")
}

// isExcepted returns true if the file matches any of the provided exceptions.
func isExcepted(rel string, exceptions []Exception) bool {
	relLower := strings.ToLower(rel)
	for _, exc := range exceptions {
		for _, fp := range exc.Files {
			if strings.Contains(relLower, strings.ToLower(fp)) {
				return true
			}
		}
	}
	return false
}

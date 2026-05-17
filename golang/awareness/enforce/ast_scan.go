package enforce

// ast_scan.go — Go source violation scanner for awareness HARD RULES.
//
// Detects violations of the project hard rules:
//   - NO localhost/127.0.0.1 in string literals or const values
//   - NO gRPC dial with loopback address
//   - NO os.Getenv calls (config must come from etcd)
//   - NO os/exec imports in cluster_controller packages
//
// These enforce the same rules checked by 'make check-services' but operate
// on a single file as an AST-level awareness check.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ScanViolation is a single rule violation found in a Go source file.
type ScanViolation struct {
	File    string
	Line    int
	Kind    string // LOOPBACK_STRING_LITERAL | CONST_LOOPBACK | GRPC_DIAL_LOOPBACK | OS_GETENV | EXEC_IMPORT_IN_CONTROLLER
	Message string
}

// ScanGoFileResult holds all violations found in a single file.
type ScanGoFileResult struct {
	File       string
	Violations []ScanViolation
}

// ScanGoFile parses a single .go file and returns all hard-rule violations.
// isController should be true when the file is in a cluster_controller package
// (disallows os/exec imports).
func ScanGoFile(path string, isController bool) (*ScanGoFileResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ScanGoSource(path, data, isController)
}

// ScanGoSource parses Go source from src (with filename path for position info)
// and returns all hard-rule violations.
func ScanGoSource(path string, src []byte, isController bool) (*ScanGoFileResult, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		// Return empty result for unparseable files (generated, build-tagged).
		return &ScanGoFileResult{File: path}, nil
	}

	res := &ScanGoFileResult{File: path}

	// 1. Import checks.
	for _, imp := range f.Imports {
		if imp.Path == nil {
			continue
		}
		importPath := strings.Trim(imp.Path.Value, `"`)
		if importPath == "os/exec" && isController {
			pos := fset.Position(imp.Pos())
			res.Violations = append(res.Violations, ScanViolation{
				File:    path,
				Line:    pos.Line,
				Kind:    "EXEC_IMPORT_IN_CONTROLLER",
				Message: "cluster_controller must not import os/exec",
			})
		}
	}

	// 2. Walk the AST for string literals and call expressions.
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				val := strings.Trim(v.Value, `"`)
				if val == "127.0.0.1" || val == "localhost" || strings.HasPrefix(val, "127.0.0.1:") || strings.HasPrefix(val, "localhost:") {
					pos := fset.Position(v.Pos())
					res.Violations = append(res.Violations, ScanViolation{
						File:    path,
						Line:    pos.Line,
						Kind:    "LOOPBACK_STRING_LITERAL",
						Message: "loopback address in string literal: " + v.Value,
					})
				}
			}
		case *ast.ValueSpec:
			// Const/var declarations: check initializer values.
			for _, val := range v.Values {
				if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					inner := strings.Trim(lit.Value, `"`)
					if inner == "127.0.0.1" || inner == "localhost" {
						pos := fset.Position(lit.Pos())
						res.Violations = append(res.Violations, ScanViolation{
							File:    path,
							Line:    pos.Line,
							Kind:    "CONST_LOOPBACK",
							Message: "loopback address in const/var: " + lit.Value,
						})
					}
				}
			}
		case *ast.CallExpr:
			// os.Getenv("...") calls.
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "os" && sel.Sel.Name == "Getenv" {
						pos := fset.Position(v.Pos())
						res.Violations = append(res.Violations, ScanViolation{
							File:    path,
							Line:    pos.Line,
							Kind:    "OS_GETENV",
							Message: "os.Getenv is forbidden; config must come from etcd",
						})
					}
				}
			}
			// grpc.Dial / grpc.DialContext with loopback address.
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "grpc" && (sel.Sel.Name == "Dial" || sel.Sel.Name == "DialContext" || sel.Sel.Name == "NewClient") {
						for _, arg := range v.Args {
							if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
								val := strings.Trim(lit.Value, `"`)
								if strings.HasPrefix(val, "127.0.0.1") || strings.HasPrefix(val, "localhost") {
									pos := fset.Position(v.Pos())
									res.Violations = append(res.Violations, ScanViolation{
										File:    path,
										Line:    pos.Line,
										Kind:    "GRPC_DIAL_LOOPBACK",
										Message: "grpc.Dial with loopback address: " + lit.Value,
									})
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	return res, nil
}

// ScanGoPackage scans all non-test .go files in dir for hard-rule violations.
// isControllerPkg should be true for cluster_controller packages.
func ScanGoPackage(dir string, isControllerPkg bool) ([]ScanViolation, error) {
	var all []ScanViolation
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		res, err := ScanGoFile(path, isControllerPkg)
		if err != nil {
			return nil
		}
		all = append(all, res.Violations...)
		return nil
	})
	return all, err
}

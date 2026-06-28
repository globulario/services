package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPExecSurfacesAreExplicitlyAllowlisted(t *testing.T) {
	allowed := map[string]string{
		"governor.go":       "legacy governed CLI executor",
		"tools_governor.go": "plan/validate/approval-gated CLI executor",
		"tools_package.go":  "package build/publish helpers shell out to canonical globular CLI",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(".", name)
		usesExec, callsCommand := fileUsesOSExec(t, path)
		if !usesExec && !callsCommand {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Fatalf("%s uses os/exec or exec.Command outside the MCP execution allowlist", name)
		}
	}
}

func fileUsesOSExec(t *testing.T, path string) (usesExec bool, callsCommand bool) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports %s: %v", path, err)
	}
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, `"`) == "os/exec" {
			usesExec = true
			break
		}
	}

	file, err = parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext" {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "exec" {
			callsCommand = true
		}
		return true
	})
	return usesExec, callsCommand
}

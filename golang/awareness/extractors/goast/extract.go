// Package goast extracts source graph nodes from Go source files.
// It walks .go files, creates source_file, go_package, and symbol nodes,
// and adds defines/imports edges. It also processes //globular: annotations.
package goast

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// Extract walks walkDir for .go files (excluding _test.go) and extracts
// source_file, go_package, and symbol nodes into g.
// Paths stored in the graph are relative to pathRoot (typically the repo root).
func Extract(ctx context.Context, g *graph.Graph, walkDir, pathRoot string) error {
	return filepath.WalkDir(walkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			// Skip hidden directories and vendor.
			if strings.HasPrefix(name, ".") || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(pathRoot, path)
		if err != nil {
			return err
		}
		return extractFile(ctx, g, path, rel)
	})
}

func extractFile(ctx context.Context, g *graph.Graph, absPath, relPath string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		// Skip files that don't parse cleanly (generated files, build tags, etc.).
		return nil
	}

	pkgName := f.Name.Name
	pkgDir := filepath.Dir(relPath)
	pkgID := "go_package:" + pkgDir

	// Ensure package node.
	if err := g.AddNode(ctx, graph.Node{
		ID:   pkgID,
		Type: graph.NodeTypeGoPackage,
		Name: pkgName,
		Path: pkgDir,
	}); err != nil {
		return err
	}

	// Source file node.
	fileID := "source_file:" + relPath
	if err := g.AddNode(ctx, graph.Node{
		ID:   fileID,
		Type: graph.NodeTypeSourceFile,
		Name: filepath.Base(relPath),
		Path: relPath,
	}); err != nil {
		return err
	}

	// Package owns file.
	if err := g.AddEdge(ctx, graph.Edge{Src: pkgID, Kind: graph.EdgeDefines, Dst: fileID}); err != nil {
		return err
	}

	// Imports.
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		importID := "go_package:" + importPath
		if err := g.AddNode(ctx, graph.Node{
			ID:   importID,
			Type: graph.NodeTypeGoPackage,
			Name: filepath.Base(importPath),
			Path: importPath,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImports, Dst: importID}); err != nil {
			return err
		}
	}

	// Declarations (functions, methods, types).
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			symName := funcDeclName(d)
			symID := "symbol:" + pkgDir + "." + symName
			if err := g.AddNode(ctx, graph.Node{
				ID:      symID,
				Type:    graph.NodeTypeSymbol,
				Name:    symName,
				Path:    relPath,
				Summary: extractDocComment(d.Doc),
			}); err != nil {
				return err
			}
			if err := g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: symID}); err != nil {
				return err
			}
			// Process //globular: annotations in doc comment.
			if err := processAnnotations(ctx, g, symID, d.Doc); err != nil {
				return err
			}

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				symID := "symbol:" + pkgDir + "." + ts.Name.Name
				if err := g.AddNode(ctx, graph.Node{
					ID:   symID,
					Type: graph.NodeTypeSymbol,
					Name: ts.Name.Name,
					Path: relPath,
				}); err != nil {
					return err
				}
				if err := g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: symID}); err != nil {
					return err
				}
				// Process annotations on GenDecl doc comment (covers the type block).
				if err := processAnnotations(ctx, g, symID, d.Doc); err != nil {
					return err
				}
			}
		}
	}

	// Process file-level annotations from the package doc comment.
	if err := processAnnotations(ctx, g, fileID, f.Doc); err != nil {
		return err
	}

	return nil
}

// funcDeclName returns "(*Recv).Name" for methods, "Name" for plain funcs.
func funcDeclName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	recv := d.Recv.List[0].Type
	switch r := recv.(type) {
	case *ast.StarExpr:
		if id, ok := r.X.(*ast.Ident); ok {
			return "(*" + id.Name + ")." + d.Name.Name
		}
	case *ast.Ident:
		return r.Name + "." + d.Name.Name
	}
	return d.Name.Name
}

// extractDocComment returns the first line of a doc comment, or "".
func extractDocComment(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	for _, c := range cg.List {
		line := strings.TrimPrefix(c.Text, "//")
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "globular:") {
			return line
		}
	}
	return ""
}

// processAnnotations handles //globular: directives in a comment group.
func processAnnotations(ctx context.Context, g *graph.Graph, ownerID string, cg *ast.CommentGroup) error {
	if cg == nil {
		return nil
	}
	for _, c := range cg.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if !strings.HasPrefix(line, "globular:") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		directive := strings.TrimPrefix(parts[0], "globular:")
		value := strings.TrimSpace(parts[1])

		switch directive {
		case "service":
			svcID := "service:" + value
			_ = g.AddNode(ctx, graph.Node{ID: svcID, Type: graph.NodeTypeGlobularService, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: svcID, Kind: graph.EdgeOwns, Dst: ownerID})

		case "enforces":
			invID := "invariant:" + value
			_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeEnforces, Dst: invID, Required: true, Confidence: 1.0})

		case "protects":
			invID := "invariant:" + value
			_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeProtects, Dst: invID, Required: true, Confidence: 1.0})

		case "reads":
			etcdID := "etcd_key:" + value
			_ = g.AddNode(ctx, graph.Node{ID: etcdID, Type: graph.NodeTypeEtcdKey, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeReads, Dst: etcdID})

		case "writes":
			etcdID := "etcd_key:" + value
			_ = g.AddNode(ctx, graph.Node{ID: etcdID, Type: graph.NodeTypeEtcdKey, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeWrites, Dst: etcdID})

		case "controls":
			unitID := "systemd_unit:" + value
			_ = g.AddNode(ctx, graph.Node{ID: unitID, Type: graph.NodeTypeSystemdUnit, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeControls, Dst: unitID})

		case "forbids":
			fixID := "forbidden_fix:" + value
			_ = g.AddNode(ctx, graph.Node{ID: fixID, Type: graph.NodeTypeForbiddenFix, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeForbids, Dst: fixID, Required: true, Confidence: 1.0})

		case "hash_schema":
			schemaID := "hash_schema:" + value
			_ = g.AddNode(ctx, graph.Node{ID: schemaID, Type: graph.NodeTypeHashSchema, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeProduces, Dst: schemaID, Confidence: 1.0})

		case "expects_hash_schema":
			schemaID := "hash_schema:" + value
			_ = g.AddNode(ctx, graph.Node{ID: schemaID, Type: graph.NodeTypeHashSchema, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeRequires, Dst: schemaID, Confidence: 1.0})

		case "state_transition":
			// Parse "from -> to" or "from->to".
			transName := strings.ReplaceAll(value, "->", " -> ")
			transName = strings.Join(strings.Fields(transName), " ")
			transID := "state_transition:" + strings.ReplaceAll(transName, " ", "")
			_ = g.AddNode(ctx, graph.Node{ID: transID, Type: graph.NodeTypeStateTransition, Name: transName})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeAffects, Dst: transID})

		case "phase":
			phaseID := "dependency_phase:" + value
			_ = g.AddNode(ctx, graph.Node{ID: phaseID, Type: graph.NodeTypeDependencyPhase, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeAffects, Dst: phaseID})

		case "risk":
			riskID := "risk_surface:" + value
			_ = g.AddNode(ctx, graph.Node{ID: riskID, Type: graph.NodeTypeRiskSurface, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeAffects, Dst: riskID})

		case "tested_by":
			testID := "test:" + value
			_ = g.AddNode(ctx, graph.Node{ID: testID, Type: graph.NodeTypeTest, Name: value})
			_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeTestedBy, Dst: testID, Confidence: 1.0})
		}
	}
	return nil
}

// ExtractTests walks walkDir for *_test.go files and creates test nodes.
// Paths stored in the graph are relative to pathRoot (typically the repo root).
func ExtractTests(ctx context.Context, g *graph.Graph, walkDir, pathRoot string) error {
	return filepath.WalkDir(walkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(pathRoot, path)
		if err != nil {
			return err
		}
		return extractTestFile(ctx, g, path, rel)
	})
}

func extractTestFile(ctx context.Context, g *graph.Graph, absPath, relPath string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return nil
	}

	fileID := "source_file:" + relPath
	if err := g.AddNode(ctx, graph.Node{
		ID:   fileID,
		Type: graph.NodeTypeSourceFile,
		Name: filepath.Base(relPath),
		Path: relPath,
		Metadata: map[string]any{"is_test": true},
	}); err != nil {
		return err
	}

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !strings.HasPrefix(fd.Name.Name, "Test") &&
			!strings.HasPrefix(fd.Name.Name, "Benchmark") &&
			!strings.HasPrefix(fd.Name.Name, "Example") {
			continue
		}
		testID := "test:" + fd.Name.Name
		if err := g.AddNode(ctx, graph.Node{
			ID:   testID,
			Type: graph.NodeTypeTest,
			Name: fd.Name.Name,
			Path: relPath,
		}); err != nil {
			return err
		}
		if err := g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: testID}); err != nil {
			return err
		}
	}

	return nil
}

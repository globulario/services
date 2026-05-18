// Package javascript extracts source_file, symbol, and test nodes from
// .js, .jsx, and .mjs files using line-based pattern matching (ES2025).
//
// What is extracted:
//   - source_file node per file (with is_test metadata for spec/test files)
//   - symbol nodes for exported declarations (functions, classes, constants)
//   - test nodes for test/it/describe calls in spec/test files
//   - imports edges from source_file to locally-imported modules (relative paths only)
//   - defines edges from source_file to each symbol or test it declares
//
// CommonJS files (.cjs) are indexed as source_file nodes but export extraction
// is not attempted — CJS uses module.exports, not the ES export keyword.
package javascript

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// skippedDirs are directories that never contain JavaScript source we care about.
var skippedDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	".git":         true,
	".turbo":       true,
	".next":        true,
}

// jsExtensions are the file extensions handled by this extractor.
// .cjs is included for source_file indexing only (no export extraction).
var jsExtensions = map[string]bool{
	".js":  true,
	".jsx": true,
	".mjs": true,
	".cjs": true,
}

// testNameRe matches the opening of a test/it/describe call and captures the name.
var testNameRe = regexp.MustCompile(`^\s*(?:test|it|describe)(?:\.each|\.only|\.skip|\.todo)?\s*\(\s*(?:'([^']*)'|"([^"]*)"|` + "`([^`]*)`" + `)`)

// importFromRe matches `import ... from './path'` and captures the module path.
var importFromRe = regexp.MustCompile(`\bfrom\s+['"](\.[^'"]+)['"]`)

// Extract walks walkDir for JavaScript files and populates the graph.
// Paths stored in the graph are relative to pathRoot (typically the repo root).
func Extract(ctx context.Context, g *graph.Graph, walkDir, pathRoot string) error {
	return filepath.WalkDir(walkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !jsExtensions[ext] {
			return nil
		}
		rel, err := filepath.Rel(pathRoot, path)
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		isTest := strings.Contains(base, ".spec.") || strings.Contains(base, ".test.")
		isCJS := ext == ".cjs"

		if isTest {
			return extractTestFile(ctx, g, path, rel)
		}
		return extractSourceFile(ctx, g, path, rel, isCJS)
	})
}

func extractSourceFile(ctx context.Context, g *graph.Graph, absPath, relPath string, cjs bool) error {
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	fileID := "source_file:" + relPath
	meta := map[string]any{"lang": "javascript"}
	if cjs {
		meta["module_format"] = "commonjs"
	}
	_ = g.AddNode(ctx, graph.Node{
		ID:       fileID,
		Type:     graph.NodeTypeSourceFile,
		Name:     filepath.Base(relPath),
		Path:     relPath,
		Metadata: meta,
	})

	if cjs {
		return nil // CJS: index the file but skip export/import extraction.
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Export declarations → symbol nodes.
		if name, ok := extractExportedName(trimmed); ok && name != "" {
			symID := "symbol:" + relPath + "#" + name
			_ = g.AddNode(ctx, graph.Node{
				ID:   symID,
				Type: graph.NodeTypeSymbol,
				Name: name,
				Path: relPath,
			})
			_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: symID})
		}

		// Local import statements → imports edges.
		if m := importFromRe.FindStringSubmatch(line); m != nil {
			dir := filepath.Dir(relPath)
			resolved := filepath.Clean(filepath.Join(dir, m[1]))
			_ = g.AddEdge(ctx, graph.Edge{
				Src:  fileID,
				Kind: graph.EdgeImports,
				Dst:  "source_file:" + resolved,
			})
		}
	}
	return scanner.Err()
}

func extractTestFile(ctx context.Context, g *graph.Graph, absPath, relPath string) error {
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	fileID := "source_file:" + relPath
	_ = g.AddNode(ctx, graph.Node{
		ID:   fileID,
		Type: graph.NodeTypeSourceFile,
		Name: filepath.Base(relPath),
		Path: relPath,
		Metadata: map[string]any{"lang": "javascript", "is_test": true},
	})

	var describeStack []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		m := testNameRe.FindStringSubmatch(trimmed)
		if m == nil {
			if strings.HasPrefix(trimmed, "})") || trimmed == "})" {
				if len(describeStack) > 0 {
					describeStack = describeStack[:len(describeStack)-1]
				}
			}
			continue
		}

		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			name = m[3]
		}
		if name == "" {
			continue
		}

		keyword := strings.TrimSpace(strings.SplitN(trimmed, "(", 2)[0])
		if idx := strings.Index(keyword, "."); idx >= 0 {
			keyword = keyword[:idx]
		}
		keyword = strings.TrimSpace(keyword)

		if keyword == "describe" {
			describeStack = append(describeStack, name)
			continue
		}

		qualifiedName := name
		if len(describeStack) > 0 {
			qualifiedName = strings.Join(describeStack, " > ") + " > " + name
		}

		testID := "test:" + relPath + "#" + qualifiedName
		_ = g.AddNode(ctx, graph.Node{
			ID:   testID,
			Type: graph.NodeTypeTest,
			Name: qualifiedName,
			Path: relPath,
			Metadata: map[string]any{"lang": "javascript"},
		})
		_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: testID})
	}
	return scanner.Err()
}

// extractExportedName returns the exported identifier from a JavaScript ES
// export declaration line. TypeScript-only keywords (interface, type) are
// intentionally excluded. Returns ("", false) if not an export.
func extractExportedName(line string) (string, bool) {
	if !strings.HasPrefix(line, "export ") {
		return "", false
	}
	rest := strings.TrimPrefix(line, "export ")

	for _, mod := range []string{"default ", "async ", "declare "} {
		rest = strings.TrimPrefix(rest, mod)
	}

	for _, kw := range []string{"function ", "class ", "const ", "let ", "var ", "enum ", "abstract class "} {
		if !strings.HasPrefix(rest, kw) {
			continue
		}
		rest = strings.TrimPrefix(rest, kw)
		name := strings.FieldsFunc(rest, func(r rune) bool {
			return r == ' ' || r == '(' || r == '<' || r == '=' || r == '{' || r == ':' || r == '\t'
		})
		if len(name) == 0 {
			return "", false
		}
		n := name[0]
		if n == "" || n == "{" {
			return "", false
		}
		return n, true
	}

	return "", false
}

// Package typescript extracts source_file, symbol, and test nodes from
// .ts and .tsx files using line-based pattern matching.
//
// What is extracted:
//   - source_file node per file (with is_test metadata for spec/test files)
//   - symbol nodes for exported declarations (functions, classes, interfaces, types, constants)
//   - test nodes for test/it/describe calls in spec/test files
//   - imports edges from source_file to locally-imported modules (relative paths only)
//   - defines edges from source_file to each symbol or test it declares
package typescript

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

// skippedDirs are directories that never contain TypeScript source we care about.
var skippedDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"coverage":     true,
	".git":         true,
	".turbo":       true,
	".next":        true,
}

// testNameRe matches the opening of a test/it/describe call and captures the name.
// Handles single, double, and backtick-quoted names.
var testNameRe = regexp.MustCompile(`^\s*(?:test|it|describe)(?:\.each|\.only|\.skip|\.todo)?\s*\(\s*(?:'([^']*)'|"([^"]*)"|` + "`([^`]*)`" + `)`)

// importFromRe matches `import ... from './path'` and captures the module path.
var importFromRe = regexp.MustCompile(`\bfrom\s+['"](\.[^'"]+)['"]`)

// Extract walks walkDir for .ts and .tsx files and populates the graph.
// Declaration files (.d.ts) are skipped. Paths stored in the graph are
// relative to pathRoot (typically the repo root).
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
		if strings.HasSuffix(path, ".d.ts") {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") {
			return nil
		}
		rel, err := filepath.Rel(pathRoot, path)
		if err != nil {
			return err
		}
		isTest := strings.Contains(filepath.Base(path), ".spec.") ||
			strings.Contains(filepath.Base(path), ".test.")
		if isTest {
			return extractTestFile(ctx, g, path, rel)
		}
		return extractSourceFile(ctx, g, path, rel)
	})
}

func extractSourceFile(ctx context.Context, g *graph.Graph, absPath, relPath string) error {
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
		Metadata: map[string]any{"lang": "typescript"},
	})

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
			modPath := m[1]
			// Resolve relative to the importing file's directory so the target
			// ID is always repo-root-relative.
			dir := filepath.Dir(relPath)
			resolved := filepath.Clean(filepath.Join(dir, modPath))
			targetID := "source_file:" + resolved
			_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImports, Dst: targetID})
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
		Metadata: map[string]any{"lang": "typescript", "is_test": true},
	})

	// describeStack tracks nested describe names for qualified test IDs.
	var describeStack []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		m := testNameRe.FindStringSubmatch(trimmed)
		if m == nil {
			// Track closing braces to pop describe stack.
			if strings.HasPrefix(trimmed, "})") || trimmed == "})" {
				if len(describeStack) > 0 {
					describeStack = describeStack[:len(describeStack)-1]
				}
			}
			continue
		}

		// Extract matched name from whichever capture group fired.
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
		// Strip any .each/.only/.skip modifier.
		if idx := strings.Index(keyword, "."); idx >= 0 {
			keyword = keyword[:idx]
		}
		keyword = strings.TrimSpace(keyword)

		if keyword == "describe" {
			describeStack = append(describeStack, name)
			continue
		}

		// Qualify test name with any enclosing describe blocks.
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
			Metadata: map[string]any{"lang": "typescript"},
		})
		_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: testID})
	}
	return scanner.Err()
}

// extractExportedName returns the exported identifier from a TypeScript export
// declaration line. Returns ("", false) if the line is not an export.
func extractExportedName(line string) (string, bool) {
	if !strings.HasPrefix(line, "export ") {
		return "", false
	}
	rest := strings.TrimPrefix(line, "export ")

	// Strip modifiers.
	for _, mod := range []string{"default ", "async ", "declare "} {
		rest = strings.TrimPrefix(rest, mod)
	}

	// Determine kind and extract name.
	for _, kw := range []string{"function ", "class ", "interface ", "type ", "const ", "let ", "var ", "enum ", "abstract class "} {
		if !strings.HasPrefix(rest, kw) {
			continue
		}
		rest = strings.TrimPrefix(rest, kw)
		// Name ends at first whitespace, '(', '<', '=', '{', or ':'
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

	// export { Foo, Bar } — skip, these are re-exports.
	// export default <expr> without a named declaration — skip.
	return "", false
}

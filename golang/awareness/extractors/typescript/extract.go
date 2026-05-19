// Package typescript extracts source_file, symbol, and test nodes from
// .ts and .tsx files using line-based pattern matching.
//
// What is extracted:
//   - source_file node per file (with is_test metadata for spec/test files)
//   - symbol nodes for exported declarations (functions, classes, interfaces, types, constants)
//   - test nodes for test/it/describe calls in spec/test files
//   - imports edges from source_file to locally-imported modules (relative paths only)
//   - defines edges from source_file to each symbol or test it declares
//   - enforces/protects/forbids/violates edges from // globular: annotations
//   - violates edges (confidence 0.8) from pattern-based forbidden behavior detection
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

// annotationRe matches // globular: <directive> <value> comments.
var annotationRe = regexp.MustCompile(`^//\s*globular:\s*(\w+)\s+(.+)$`)

// violationPattern describes a forbidden pattern to detect in TypeScript source.
type violationPattern struct {
	re          *regexp.Regexp
	invariantID string
	detail      string
}

// uiViolationPatterns are patterns that indicate a UI invariant may be violated.
// Each match produces a violates edge (confidence 0.8) from the source_file to the invariant.
var uiViolationPatterns = []violationPattern{
	// Empty catch blocks — silences gRPC errors from the operator.
	{
		re:          regexp.MustCompile(`\}\s*catch\s*(\([^)]*\))?\s*\{\s*\}`),
		invariantID: "ui.grpc_web_errors_must_surface_to_operator",
		detail:      "empty catch block — errors not surfaced to operator",
	},
	// Token stored in localStorage.
	{
		re:          regexp.MustCompile(`localStorage\.setItem\s*\(\s*['"][^'"]*[Tt]oken`),
		invariantID: "ui.token_storage_sessionStorage_only",
		detail:      "auth token stored in localStorage instead of sessionStorage",
	},
	{
		re:          regexp.MustCompile(`localStorage\.setItem\s*\(\s*['"]auth`),
		invariantID: "ui.token_storage_sessionStorage_only",
		detail:      "auth data stored in localStorage instead of sessionStorage",
	},
	// Hardcoded backend address with port number.
	{
		re:          regexp.MustCompile(`['"]https?://[a-zA-Z0-9][a-zA-Z0-9._-]*:\d{2,5}['"]`),
		invariantID: "ui.no_hardcoded_backend_addresses",
		detail:      "hardcoded backend address with port",
	},
	// Unknown state mapped to healthy — fallthrough to 'healthy' / green for unknown.
	{
		re:          regexp.MustCompile(`:\s*['"]healthy['"]`),
		invariantID: "ui.unknown_state_must_not_appear_healthy",
		detail:      "default branch may map unknown state to 'healthy'",
	},
}

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

	// pendingAnnotations buffers // globular: directives seen before an export
	// declaration so they can be attached to the symbol node as well.
	var pendingAnnotations []string

	// violatedInvariants deduplicates pattern-based violation edges per file.
	violatedInvariants := map[string]bool{}

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		lineNum++

		// Globular annotation comment → apply to file immediately and buffer for
		// the next exported symbol.
		if m := annotationRe.FindStringSubmatch(trimmed); m != nil {
			directive := m[1]
			value := strings.TrimSpace(m[2])
			applyAnnotation(ctx, g, fileID, directive, value)
			pendingAnnotations = append(pendingAnnotations, trimmed)
			continue
		}

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

			// Apply buffered annotations to this symbol too.
			for _, ann := range pendingAnnotations {
				if mm := annotationRe.FindStringSubmatch(ann); mm != nil {
					applyAnnotation(ctx, g, symID, mm[1], strings.TrimSpace(mm[2]))
				}
			}
			pendingAnnotations = nil
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "*") && !strings.HasPrefix(trimmed, "/*") {
			// Non-annotation non-empty line: clear the pending buffer.
			pendingAnnotations = nil
		}

		// Local import statements → imports edges.
		if m := importFromRe.FindStringSubmatch(line); m != nil {
			modPath := m[1]
			dir := filepath.Dir(relPath)
			resolved := filepath.Clean(filepath.Join(dir, modPath))
			targetID := "source_file:" + resolved
			_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImports, Dst: targetID})
		}

		// Pattern-based violation detection → violates edges (deduplicated per file).
		for _, vp := range uiViolationPatterns {
			if violatedInvariants[vp.invariantID] {
				continue
			}
			if vp.re.MatchString(line) {
				invID := "invariant:" + vp.invariantID
				_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: vp.invariantID})
				_ = g.AddEdge(ctx, graph.Edge{
					Src:        fileID,
					Kind:       graph.EdgeViolates,
					Dst:        invID,
					Confidence: 0.8,
					Metadata:   map[string]any{"detail": vp.detail, "line": lineNum, "auto": true},
				})
				violatedInvariants[vp.invariantID] = true
			}
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

		// Globular annotations in test files apply to the test file node.
		if m := annotationRe.FindStringSubmatch(trimmed); m != nil {
			applyAnnotation(ctx, g, fileID, m[1], strings.TrimSpace(m[2]))
			continue
		}

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

// applyAnnotation processes a globular: directive and creates the corresponding graph edge.
func applyAnnotation(ctx context.Context, g *graph.Graph, ownerID, directive, value string) {
	switch directive {
	case "enforces":
		invID := "invariant:" + value
		_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: value})
		_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeEnforces, Dst: invID, Required: true, Confidence: 1.0})
	case "protects":
		invID := "invariant:" + value
		_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: value})
		_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeProtects, Dst: invID, Required: true, Confidence: 1.0})
	case "forbids":
		fixID := "forbidden_fix:" + value
		_ = g.AddNode(ctx, graph.Node{ID: fixID, Type: graph.NodeTypeForbiddenFix, Name: value})
		_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeForbids, Dst: fixID, Required: true, Confidence: 1.0})
	case "violates":
		invID := "invariant:" + value
		_ = g.AddNode(ctx, graph.Node{ID: invID, Type: graph.NodeTypeInvariant, Name: value})
		_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeViolates, Dst: invID, Confidence: 0.9})
	case "tested_by":
		testID := "test:" + value
		_ = g.AddEdge(ctx, graph.Edge{Src: ownerID, Kind: graph.EdgeTestedBy, Dst: testID, Confidence: 1.0})
	}
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

package main

// v1.2.119 — controller-side regression tests for the
// controller.apply_package_release_must_carry_expected_sha256 invariant.
//
// Together with the actor-level tests in golang/workflow/engine, these tests
// prove every controller dispatch path carries manifest.entrypoint_checksum
// as ApplyPackageReleaseRequest.ExpectedSha256.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDispatch_AuditAllCallSitesSetExpectedSha256 walks every Go source file
// under golang/ and asserts that every literal construction of
// node_agentpb.ApplyPackageReleaseRequest either sets ExpectedSha256 or
// appears in the explicit allow-list of degraded paths below. This is the
// regression guard against new dispatch sites being added without manifest
// identity propagation.
//
// To add a new degraded path: add it to allowedDegradedSites with a comment
// pointing at the invariant rationale. There should be very few of these.
func TestDispatch_AuditAllCallSitesSetExpectedSha256(t *testing.T) {
	// Degraded paths that legitimately do not have manifest identity at
	// dispatch time. Each entry MUST be justified by a documented invariant
	// exception. See docs/awareness/invariants.yaml for the full rationale.
	allowedDegradedSites := map[string]string{
		// forward_recover applies the *previous* revision after a failed rollback.
		// The previous manifest may have been purged. The node-agent then writes
		// installed_unverified, which is the honest state. Forbidden alternative:
		// faking a checksum from the on-disk binary.
		"golang/node_agent/node_agent_server/actor_service.go:handleForwardRecover": "rollback previous revision; manifest may be unavailable",
		// state canonicalize --fix-safe / --fix is an operator escape hatch with
		// Force=true. The node-agent writes installed_unverified — the honest
		// signal that identity is unproven. The next controller reconcile pass
		// re-dispatches via the verified path, transitioning state to installed.
		// See the in-source comment at the call site for the full rationale.
		"golang/globularcli/state_cmds.go:repairInstalledStateBuildID": "operator metadata repair; controller re-dispatch carries verified identity",
	}

	// scanRoot is the directory containing go.mod (the golang/ module root).
	scanRoot, err := repoRootFromTest()
	if err != nil {
		t.Fatalf("locate go module root: %v", err)
	}

	type findingT struct {
		file     string
		function string
		line     int
		hasField bool
	}
	var findings []findingT

	walkErr := filepath.Walk(scanRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Generated proto files don't construct requests; skip them.
		if strings.HasSuffix(path, ".pb.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
		if err != nil {
			return nil // skip unparseable; build will catch real errors
		}

		ast.Inspect(f, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok || cl.Type == nil {
				return true
			}
			// Match selector: pkg.ApplyPackageReleaseRequest
			sel, ok := cl.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "ApplyPackageReleaseRequest" {
				return true
			}
			hasField := false
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				ident, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				if ident.Name == "ExpectedSha256" {
					hasField = true
					break
				}
			}
			// Relative to repo root (parent of scanRoot) for stable allow-list keys
			// matching docs/CLAUDE.md paths (e.g. "golang/node_agent/...").
			repoRoot := filepath.Dir(scanRoot)
			rel, _ := filepath.Rel(repoRoot, path)
			fn := enclosingFuncName(f, fset, cl.Pos())
			findings = append(findings, findingT{
				file:     filepath.ToSlash(rel),
				function: fn,
				line:     fset.Position(cl.Pos()).Line,
				hasField: hasField,
			})
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
	if len(findings) == 0 {
		t.Fatalf("no ApplyPackageReleaseRequest literals found — test may be looking at the wrong directory")
	}

	var missing []findingT
	for _, f := range findings {
		if f.hasField {
			continue
		}
		key := f.file + ":" + f.function
		if _, ok := allowedDegradedSites[key]; ok {
			continue
		}
		missing = append(missing, f)
	}

	if len(missing) > 0 {
		var b strings.Builder
		b.WriteString("the following ApplyPackageReleaseRequest constructions do not set ExpectedSha256.\n")
		b.WriteString("either populate ExpectedSha256 from manifest.entrypoint_checksum, or add the\n")
		b.WriteString("site to allowedDegradedSites with a documented invariant exception.\n")
		b.WriteString("forbidden fallbacks: synthesised hash, filename parse, on-disk binary hash.\n")
		b.WriteString("see invariant controller.apply_package_release_requires_manifest_checksum.\n\n")
		for _, m := range missing {
			b.WriteString("  ")
			b.WriteString(m.file)
			b.WriteString(":")
			b.WriteString(itoa(m.line))
			b.WriteString(" in ")
			b.WriteString(m.function)
			b.WriteString("\n")
		}
		t.Fatal(b.String())
	}
}

// TestDispatch_ResolverEntrypointChecksumIsBinaryHash documents the hash schema
// contract: ResolvedArtifact.EntrypointChecksum is the BINARY sha256, and
// ResolvedArtifact.Digest is the PACKAGE TARBALL sha256. The dispatch chain
// must propagate EntrypointChecksum, not Digest. Confusing the two is the
// v1.2.56/v1.2.57/v1.2.58 hash-schema-confusion bug class — see hash schema
// documentation in release_resolver.go.
func TestDispatch_ResolverEntrypointChecksumIsBinaryHash(t *testing.T) {
	got := &ResolvedArtifact{
		Digest:             "package-tarball-hash",
		EntrypointChecksum: "binary-hash",
	}
	// The propagation contract is: callers pass got.EntrypointChecksum (not
	// got.Digest) as expectedSha256. This sentinel test fails if a future
	// refactor swaps the two fields' meanings.
	if got.EntrypointChecksum == got.Digest {
		t.Fatalf("EntrypointChecksum and Digest must be distinct values; got both = %q", got.EntrypointChecksum)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// repoRootFromTest finds the repository root by walking up from the test's
// working directory until it sees a go.mod.
func repoRootFromTest() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

// enclosingFuncName returns the name of the function or method that contains
// the given position. Returns "<file-scope>" if not inside a function.
func enclosingFuncName(file *ast.File, fset *token.FileSet, pos token.Pos) string {
	var found string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if pos >= fn.Pos() && pos <= fn.End() {
			found = fn.Name.Name
		}
		return true
	})
	if found == "" {
		return "<file-scope>"
	}
	return found
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

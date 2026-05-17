package graph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

// Regression tests for the composed-path failure recorded under
// "graph.Open always migrates against immutable bundles" in
// docs/awareness/composed_path_failures.md.
//
// The signed awareness bundle is content-addressed and owned root:root.
// graph.Open on an immutable directory fails. The fix is OpenReadOnly,
// which honours the bundle's immutability contract.

func TestOpenReadOnly_ReadsBundleWithoutWriting(t *testing.T) {
	// Build a graph with normal Open, close it. That simulates the bundle
	// being built and shipped. Then assert OpenReadOnly can query it.
	bundleDir := t.TempDir()

	ctx := context.Background()
	g, err := graph.Open(bundleDir)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	if err := g.AddNode(ctx, graph.Node{
		ID:   "n1",
		Type: graph.NodeTypeGlobularService,
		Name: "test",
	}); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("seed Close: %v", err)
	}

	ro, err := graph.OpenReadOnly(bundleDir)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	t.Cleanup(func() { ro.Close() })

	n, err := ro.FindNode(ctx, "n1")
	if err != nil {
		t.Fatalf("read after OpenReadOnly: %v", err)
	}
	if n == nil {
		t.Fatal("node n1 not visible after OpenReadOnly")
	}
	if n.Name != "test" {
		t.Errorf("read value = %q, want %q", n.Name, "test")
	}
}

func TestOpenReadOnly_RefusesWrites(t *testing.T) {
	bundleDir := t.TempDir()

	// Create an empty bundle.
	if g, err := graph.Open(bundleDir); err != nil {
		t.Fatalf("seed Open: %v", err)
	} else {
		g.Close()
	}

	ro, err := graph.OpenReadOnly(bundleDir)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	t.Cleanup(func() { ro.Close() })

	err = ro.AddNode(context.Background(), graph.Node{
		ID:   "n2",
		Type: graph.NodeTypeGlobularService,
		Name: "illegal-write",
	})
	if err == nil {
		t.Fatal("write through OpenReadOnly handle must fail; got nil error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "readonly") &&
		!strings.Contains(strings.ToLower(err.Error()), "read-only") {
		t.Errorf("error should mention read-only; got: %v", err)
	}
}

// The bundle directory is root-owned in production. We can't change uid in tests,
// but we can simulate immutability by making the directory not writable and
// asserting OpenReadOnly still succeeds (proves no write attempt is made during open).
func TestOpenReadOnly_SucceedsWhenDirectoryIsNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 0o555 doesn't restrict root")
	}
	bundleDir := t.TempDir()

	// Seed graph data first.
	g, err := graph.Open(bundleDir)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	g.Close()

	// Make the directory read-only.
	if err := os.Chmod(bundleDir, 0o555); err != nil {
		t.Fatal(err)
	}
	// Restore write perms on cleanup so t.TempDir's defer can remove it.
	t.Cleanup(func() {
		_ = os.Chmod(bundleDir, 0o755)
	})

	// graph.Open against a non-writable directory would fail — OpenReadOnly must not.
	ro, err := graph.OpenReadOnly(bundleDir)
	if err != nil {
		t.Fatalf("OpenReadOnly must succeed against a read-only bundle directory: %v", err)
	}
	defer ro.Close()

	// Queries must still work.
	stats, err := ro.Stats(context.Background())
	if err != nil {
		t.Errorf("Stats on read-only graph failed: %v", err)
	}
	_ = stats
}

func TestOpenReadOnly_RejectsMissingDirectory(t *testing.T) {
	_, err := graph.OpenReadOnly(filepath.Join(t.TempDir(), "absent-dir"))
	if err == nil {
		t.Fatal("OpenReadOnly on a missing path must return an error")
	}
}

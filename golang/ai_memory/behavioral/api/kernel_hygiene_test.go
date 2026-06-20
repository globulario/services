package api

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The generic behavioral-memory kernel must stay domain-agnostic: it may never
// import cluster/operator-specific systems or Globular cluster packages. Cluster
// specifics arrive only as opaque refs resolved through the domain registry, and
// the cluster_operator pack lives OUTSIDE behavioral/ (a later PR).
//
// This test walks every non-test .go file under behavioral/ and fails if any
// import path contains a forbidden marker. It is the mechanical enforcement of
// the design's domain-isolation rule.
func TestKernelHasNoClusterSpecificImports(t *testing.T) {
	// Locate the behavioral/ root: this file is behavioral/api/kernel_hygiene_test.go.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	behavioralRoot := filepath.Dir(filepath.Dir(thisFile)) // .../behavioral

	// Forbidden import-path markers — cluster/operator-specific concerns the
	// generic kernel must never reach for.
	forbidden := []string{
		"go.etcd.io", "/etcd", // etcd
		"minio",                            // object storage
		"envoy",                            // proxy
		"node_agent",                       // cluster node executor
		"cluster_controller",               // control plane
		"cluster_doctor",                   // health analysis
		"domains/cluster_operator",         // the domain pack must not be imported by the kernel
		"globular/services/golang/config",  // etcd-backed config
	}
	// "gocql" (and CQL generally) is allowed ONLY in the sanctioned store
	// adapter. Every other kernel file must be persistence-agnostic.
	const adapterFile = "scylla_store.go"

	fset := token.NewFileSet()
	var checked int
	var gocqlImporters []string
	err := filepath.Walk(behavioralRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			t.Errorf("parse %s: %v", path, perr)
			return nil
		}
		checked++
		rel, _ := filepath.Rel(behavioralRoot, path)
		isAdapter := filepath.Base(path) == adapterFile
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(p, "gocql") {
				gocqlImporters = append(gocqlImporters, rel)
				if !isAdapter {
					t.Errorf("%s imports gocql: only the store adapter %q may speak to ScyllaDB; the kernel depends on the store interface", rel, adapterFile)
				}
			}
			for _, bad := range forbidden {
				if strings.Contains(p, bad) {
					t.Errorf("%s imports forbidden cluster-specific package %q (matched %q): the generic kernel must stay domain-agnostic", rel, p, bad)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk behavioral/: %v", err)
	}
	if checked == 0 {
		t.Fatal("no kernel source files were scanned — test is not exercising anything")
	}
	// The adapter must be the sole gocql importer — proves Scylla coupling is
	// isolated, not leaking across the kernel.
	for _, f := range gocqlImporters {
		if filepath.Base(f) != adapterFile {
			t.Errorf("unexpected gocql importer %q (only %q is sanctioned)", f, adapterFile)
		}
	}
}

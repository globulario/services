package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// The etcd workflow KEY is the bare workflow name. The ".yaml" is a local
// filename convention and nothing more.
//
// SCAR — 2026-08-18, 5-node simulation on 1.2.325. fetchWorkflowDefsFromEtcd
// asked etcd for "node.join.yaml" while SeedCoreWorkflows deliberately writes
// "node.join" — workflow_day0.go strips the extension, with a comment saying
// keys carrying it "produced ghost entries that ExecuteWorkflow never
// resolved". All eight fetches therefore missed, and this cache — whose whole
// purpose is to serve a node whose local workflow directory is empty — never
// wrote a single file. node-5, freshly wiped and rejoined, logged
//
//	workflow-resolver: fetch node.join.yaml from etcd: workflow "node.join.yaml" not found in etcd
//
// and could not resolve node.join. Its seven compute workloads landed as
// systemd units with no installed-state receipt; cluster-doctor fail-closed at
// CRITICAL on all seven, failing five upgrade scenarios.
//
// Violates identity.field_semantic_is_single_writer_defined (critical): one
// field, one declared semantic, defined by its writer. The forbidden repair is
// conditional_field_semantic_by_writer — trying both key shapes, or sniffing
// which one exists.
func TestCacheWorkflowDefsUsesBareEtcdKeysAndYamlFilenames(t *testing.T) {
	destDir := t.TempDir()

	// Keyed exactly as SeedCoreWorkflows writes them: bare names.
	seeded := map[string][]byte{
		"node.join":         []byte("name: node.join\n"),
		"cluster.reconcile": []byte("name: cluster.reconcile\n"),
	}

	var listedKeys []string
	v1alpha1.EtcdWorkflowLister = func() (map[string][]byte, error) {
		copied := make(map[string][]byte, len(seeded))
		for name, data := range seeded {
			listedKeys = append(listedKeys, name)
			copied[name] = data
		}
		return copied, nil
	}
	v1alpha1.EtcdFetcher = func(name string) ([]byte, error) {
		t.Fatalf("the cache must list the prefix, not fetch a hardcoded name list (asked for %q)", name)
		return nil, nil
	}
	t.Cleanup(func() {
		v1alpha1.EtcdWorkflowLister = nil
		v1alpha1.EtcdFetcher = nil
	})

	if cached := cacheWorkflowDefsFromEtcd(destDir); cached != len(seeded) {
		t.Fatalf("cached %d definitions, want %d", cached, len(seeded))
	}

	// No key may carry the file extension — that is the exact drift that made
	// every lookup miss.
	for _, key := range listedKeys {
		if strings.HasSuffix(key, ".yaml") {
			t.Fatalf("etcd was addressed with a filename-shaped key %q; keys are bare workflow names", key)
		}
	}

	// Each definition must land under the name resolveWorkflowPath looks for,
	// which is where the extension does belong.
	for name, want := range seeded {
		path := filepath.Join(destDir, name+".yaml")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("workflow %q was not cached to %s: %v", name, path, err)
		}
		if string(body) != string(want) {
			t.Fatalf("cached %q holds %q", name, string(body))
		}
	}

	// Nothing may be written under a double extension, which is what appending
	// ".yaml" to an already-suffixed key produces.
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".yaml.yaml") {
			t.Fatalf("cache wrote a double-extension file %q", entry.Name())
		}
	}
	if len(entries) != len(seeded) {
		t.Fatalf("cached %d files for %d definitions", len(entries), len(seeded))
	}
}

// A definition the cache would resolve must be one the loader can parse. An
// empty file still satisfies resolveWorkflowPath's os.Stat, so writing it would
// turn a missing definition into a corrupt one — a worse failure, because the
// disk fallback would then stop looking.
func TestCacheWorkflowDefsSkipsEmptyDefinitions(t *testing.T) {
	destDir := t.TempDir()
	v1alpha1.EtcdWorkflowLister = func() (map[string][]byte, error) {
		return map[string][]byte{"node.join": {}}, nil
	}
	t.Cleanup(func() { v1alpha1.EtcdWorkflowLister = nil })

	if cached := cacheWorkflowDefsFromEtcd(destDir); cached != 0 {
		t.Fatalf("cached %d empty definitions", cached)
	}
	if _, err := os.Stat(filepath.Join(destDir, "node.join.yaml")); err == nil {
		t.Fatal("an empty definition was cached as a workflow file")
	}
}

// "Not asked" and "asked and empty" are different facts. A node with no lister
// configured must say so rather than report that etcd holds no workflows.
func TestCacheWorkflowDefsWithoutListerIsANoOp(t *testing.T) {
	destDir := t.TempDir()
	v1alpha1.EtcdWorkflowLister = nil

	if cached := cacheWorkflowDefsFromEtcd(destDir); cached != 0 {
		t.Fatalf("an unconfigured lister cached %d definitions", cached)
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("an unconfigured lister still wrote %d files", len(entries))
	}
}

// A listing failure must not be reported as an empty cluster: the caller would
// conclude no workflows exist and stop retrying.
func TestCacheWorkflowDefsReportsNothingCachedOnListFailure(t *testing.T) {
	destDir := t.TempDir()
	v1alpha1.EtcdWorkflowLister = func() (map[string][]byte, error) {
		return nil, os.ErrDeadlineExceeded
	}
	t.Cleanup(func() { v1alpha1.EtcdWorkflowLister = nil })

	if cached := cacheWorkflowDefsFromEtcd(destDir); cached != 0 {
		t.Fatalf("a failed listing reported %d cached definitions", cached)
	}
}

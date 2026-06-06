// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.scylla_schema_guard_drift_test
// @awareness file_role=guards_critical_keyspace_list_against_service_create_keyspace_drift
// @awareness enforces=globular.platform:invariant.scylla.critical_keyspace_replication_policy
// @awareness risk=critical
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// scyllaKeyspaceCarveOuts is the explicit, documented exception set for
// CREATE KEYSPACE statements that legitimately do NOT belong in
// criticalScyllaKeyspaces. Every entry must say WHY — drift means a real
// production-data keyspace silently runs at RF=1 on multi-node clusters.
var scyllaKeyspaceCarveOuts = map[string]string{
	// storage_store is a generic helper used by ai-watcher and a few other
	// services for ephemeral caches. The keyspace name comes from caller
	// config, not from a fixed CREATE-KEYSPACE constant. "cache" is the
	// default fallback when the caller doesn't supply one.
	"cache": "storage_store fallback keyspace for ephemeral caches — name chosen by caller config, not persistent application data",
}

// TestCriticalScyllaKeyspacesMatchSourceCreateStatements walks every .go
// file under golang/ for `CREATE KEYSPACE IF NOT EXISTS <name>` and asserts
// that <name> is present in either criticalScyllaKeyspaces or
// scyllaKeyspaceCarveOuts. The schema guard polices the listed keyspaces;
// any keyspace the codebase creates but doesn't list silently runs at
// RF=1 on a multi-node cluster, exactly the failure mode
// scylla.critical_keyspace_under_replicated names.
//
// The test ALSO catches the reverse: a name in criticalScyllaKeyspaces
// that no service actually creates. That would be a stale list entry —
// also drift, also worth fixing.
func TestCriticalScyllaKeyspacesMatchSourceCreateStatements(t *testing.T) {
	repoRoot := findRepoRootForScyllaTest(t)

	// Two flavours of source evidence prove a keyspace exists:
	//
	//   A. Direct CREATE — `CREATE KEYSPACE IF NOT EXISTS <bareword>`
	//      where the name is hardcoded in the CQL string.
	//
	//   B. Indirect CREATE — `CREATE KEYSPACE IF NOT EXISTS %s` driven by
	//      a Go constant in the same file, OR the storage_store options
	//      pattern `"keyspace":"<name>"` / `"keyspace": "<name>"` where
	//      the underlying helper does the CREATE.
	//
	// We collect both, then diff against criticalScyllaKeyspaces.
	directPat := regexp.MustCompile(`CREATE KEYSPACE IF NOT EXISTS\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	indirectCreatePat := regexp.MustCompile(`CREATE KEYSPACE IF NOT EXISTS\s+%s`)
	constNamePat := regexp.MustCompile(`(?:const|var)?\s*\w*[Kk]eyspace\w*\s*=\s*"([a-zA-Z_][a-zA-Z0-9_]*)"`)
	optionsKeyspacePat := regexp.MustCompile(`"keyspace"\s*:\s*"([a-zA-Z_][a-zA-Z0-9_]*)"`)

	created := map[string]map[string]bool{} // keyspace -> set of files
	addEvidence := func(ks, path string) {
		if created[ks] == nil {
			created[ks] = map[string]bool{}
		}
		created[ks][path] = true
	}

	err := filepath.Walk(filepath.Join(repoRoot, "golang"), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == "node_modules" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		src := string(data)
		rel, _ := filepath.Rel(repoRoot, path)

		// Pattern A — direct hardcoded name in CREATE KEYSPACE.
		for _, m := range directPat.FindAllStringSubmatch(src, -1) {
			addEvidence(m[1], rel)
		}

		// Pattern B — CREATE KEYSPACE %s in same file as a *keyspace*=
		// const or var. Pair them up: any constant in the file is treated
		// as a possible argument to the %s.
		if indirectCreatePat.MatchString(src) {
			for _, m := range constNamePat.FindAllStringSubmatch(src, -1) {
				addEvidence(m[1], rel)
			}
		}

		// Pattern B' — storage_store options-map pattern. The helper does
		// the CREATE on behalf of the service; the service's source names
		// the keyspace via "keyspace":"<name>".
		for _, m := range optionsKeyspacePat.FindAllStringSubmatch(src, -1) {
			addEvidence(m[1], rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk source: %v", err)
	}

	listed := map[string]bool{}
	for _, k := range criticalScyllaKeyspaces {
		listed[k] = true
	}

	// 1. Every keyspace the codebase creates must be listed or carved out.
	var unlisted []string
	for ks, files := range created {
		if listed[ks] {
			continue
		}
		if _, ok := scyllaKeyspaceCarveOuts[ks]; ok {
			continue
		}
		fileList := make([]string, 0, len(files))
		for f := range files {
			fileList = append(fileList, f)
		}
		sort.Strings(fileList)
		unlisted = append(unlisted, ks+" (created in: "+strings.Join(fileList, ", ")+")")
	}
	sort.Strings(unlisted)
	if len(unlisted) > 0 {
		t.Errorf("source creates %d Scylla keyspace(s) not policed by criticalScyllaKeyspaces — will run at RF=1 silently:", len(unlisted))
		for _, u := range unlisted {
			t.Errorf("  %s", u)
		}
		t.Errorf("Fix: add the keyspace to criticalScyllaKeyspaces in scylla_schema_guard.go, OR add a documented carve-out to scyllaKeyspaceCarveOuts in this test file.")
	}

	// 2. Every listed keyspace should have at least one piece of source
	//    evidence (CREATE, constant, or options-map). A listed entry with
	//    zero evidence is a stale list — fix or remove it.
	var stale []string
	for _, ks := range criticalScyllaKeyspaces {
		if len(created[ks]) == 0 {
			stale = append(stale, ks)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("criticalScyllaKeyspaces lists %d keyspace(s) that no Go source defines — stale list entries:", len(stale))
		for _, s := range stale {
			t.Errorf("  %s", s)
		}
		t.Errorf("Fix: remove the entry, or extend the scanner to recognize the missing definition pattern.")
	}
}

// findRepoRootForScyllaTest walks up from CWD until it finds the
// docs/awareness directory — the marker for the services repo root.
func findRepoRootForScyllaTest(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 12; i++ {
		if st, err := os.Stat(filepath.Join(dir, "docs", "awareness")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find services repo root walking up from %s", cwd)
	return ""
}

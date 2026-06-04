// @awareness namespace=globular.platform
// @awareness component=platform_mcp.etcd_authority_pin
// @awareness file_role=architectural_pin_tests_for_mcp_etcd_tool_removal
// @awareness enforces=globular.platform:invariant.mcp.etcd.tools_must_not_join_remote_allowlist
// @awareness enforces=globular.platform:invariant.etcd.path_has_single_owner
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=critical
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Architectural pin tests for the MCP-etcd bypass closure landed in
// v1.2.167. tools_etcd.go's registerEtcdTools is now a no-op; etcd_get,
// etcd_put, and etcd_delete are no longer registered as MCP tools.
//
// These tests fail loudly if a future contributor re-introduces any of
// the three tool registrations, or extends the MCP surface with a
// near-equivalent (etcd_list, etcd_scan, raw_kv_put, etc.).
//
// The principle (anchored in awareness):
//   - invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//   - invariant:etcd.path_has_single_owner
//   - invariant:mcp.etcd.tools_must_not_join_remote_allowlist
//   - forbidden_fix:generic_etcd_write_tool_exposed_to_callers
//   - forbidden_fix:generic_etcd_or_storage_read_tool_exposed_to_callers
//
// The safe state is "the tool does not exist", not "the tool exists but
// is blocked at the door".

// TestMCP_EtcdToolsNotRegistered is the runtime pin. It boots a server
// with the test-friendly registerAllTools path and asserts that no MCP
// tool named etcd_get / etcd_put / etcd_delete (or any near-equivalent)
// is in the registered map after init.
func TestMCP_EtcdToolsNotRegistered(t *testing.T) {
	s := &server{
		tools: make(map[string]*registeredTool),
		cfg:   &MCPConfig{ReadOnly: false},
	}
	registerEtcdTools(s)

	forbidden := []string{
		"etcd_get", "etcd_put", "etcd_delete",
		// Near-equivalents that would re-open the same vector under a
		// different name.
		"etcd_list", "etcd_scan", "etcd_keys",
		"raw_kv_get", "raw_kv_put", "raw_kv_delete",
		"kv_get", "kv_put", "kv_delete",
	}
	for _, name := range forbidden {
		if _, exists := s.tools[name]; exists {
			t.Errorf("CRITICAL MCP tool %q is registered. This is a generic etcd primitive that "+
				"bypasses every owner's typed RPC contract. Removed in v1.2.167; the safe state "+
				"is 'the tool does not exist'. See forbidden_fix:generic_etcd_write_tool_exposed_to_callers "+
				"and forbidden_fix:generic_etcd_or_storage_read_tool_exposed_to_callers.", name)
		}
	}
}

// TestMCP_EtcdToolsNotInSourceTree is the source-level pin. It walks the
// mcp package and asserts that no file (except this test, the test
// scaffolding, and the historical aggregator_policy entry that names
// forbidden tools) contains a string literal `"etcd_get"`, `"etcd_put"`,
// or `"etcd_delete"`.
//
// The aggregator_policy.go entries for etcd_put / etcd_delete in
// forbiddenRemoteTools are explicit "if anyone re-creates this, it's
// forbidden remotely" markers; they remain after v1.2.167 as
// documentation. This test allowlists that one file.
func TestMCP_EtcdToolsNotInSourceTree(t *testing.T) {
	root := mustMCPRoot(t)

	// Names that would re-register the tools or expose new generic
	// etcd primitives. Matched as quoted string literals so we catch
	// `Name: "etcd_put"` patterns specifically (vs. comments and
	// documentation).
	forbiddenLiterals := []string{
		`"etcd_get"`, `"etcd_put"`, `"etcd_delete"`,
		`"etcd_list"`, `"etcd_scan"`, `"etcd_keys"`,
		`"raw_kv_get"`, `"raw_kv_put"`, `"raw_kv_delete"`,
		`"kv_get"`, `"kv_put"`, `"kv_delete"`,
	}

	// Allowlist of file paths (relative to mcp/) that may contain the
	// forbidden literals as DOCUMENTATION. Each entry is reviewed.
	allowedFiles := map[string]string{
		"aggregator_policy.go":                  "documents etcd_put/etcd_delete in forbiddenRemoteTools so the regression class has a named marker",
		"mcp_etcd_authority_pin_test.go":        "this test file itself names the forbidden literals",
		"tools_etcd.go":                          "v1.2.167 removal stub documents the removed names in comments",
	}

	walkMCPGoFiles(t, root, func(path string, body []byte) {
		rel, _ := filepath.Rel(root, path)
		if _, allowed := allowedFiles[rel]; allowed {
			return
		}
		for _, lit := range forbiddenLiterals {
			if strings.Contains(string(body), lit) {
				t.Errorf("CRITICAL %s contains literal %s — a generic etcd primitive name. "+
					"v1.2.167 removed these from MCP; re-introducing one would re-open the bypass vector. "+
					"If you genuinely need a bootstrap-only generic primitive, scope it to a fixed prefix "+
					"allowlist HARD-CODED in the tool with a narrow name (e.g. day0_seed_set), and update "+
					"this test's allowlist with the operator-reviewed reason.",
					path, lit)
			}
		}
	})
}

// TestMCP_AggregatorAllowlistDoesNotIncludeEtcdTools is the
// remote-call pin. invariant:mcp.etcd.tools_must_not_join_remote_allowlist
// names the rule; this test asserts it concretely against the
// allowedRemoteTools map.
func TestMCP_AggregatorAllowlistDoesNotIncludeEtcdTools(t *testing.T) {
	forbidden := []string{
		"etcd_get", "etcd_put", "etcd_delete",
		"etcd_list", "etcd_scan", "etcd_keys",
		"raw_kv_get", "raw_kv_put", "raw_kv_delete",
	}
	for _, name := range forbidden {
		if allowedRemoteTools[name] {
			t.Errorf("CRITICAL allowedRemoteTools includes %q — "+
				"violates invariant:mcp.etcd.tools_must_not_join_remote_allowlist. "+
				"Generic etcd primitives must never be callable cross-node via mcp.remote_call.",
				name)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func mustMCPRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if filepath.Base(wd) != "mcp" {
		t.Fatalf("unexpected test cwd: %s (want mcp)", wd)
	}
	return wd
}

func walkMCPGoFiles(t *testing.T, root string, visit func(path string, body []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip subdirectories; the etcd-tool surface lives at
			// package main level.
			if path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		visit(path, body)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

// keep the regexp import live in case future test additions need it.
var _ = regexp.MustCompile

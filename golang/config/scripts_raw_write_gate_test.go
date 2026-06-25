package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// RT-4 (scripts half): the shell side-door gate.
//
// RT-1 (docs/design/rt1-direct-write-surface-audit.md, Surface D) found operator
// scripts that `etcdctl del/put` owner-owned etcd keys directly behind the live
// controller — the same raw-owner-write class the principle-check scanner now
// blocks in golang/globularcli (RT-4 CLI), but invisible to that Go-AST scanner
// because shell is not Go. This gate walks scripts/ and fails on any NEW script
// that mutates an owner-owned key, while the known steady-state scripts are
// baselined as tracked RT-2 debt (NOT blessed). It is the shell-side smoke alarm:
// once both Go and shell side doors trip the wire, RT-2 can migrate the bad paths
// knowing no new bypass — in either language — can land silently.

// scriptOwnerOwnedPrefixes are etcd key prefixes owned by a service (mirrors the
// ownership in config.CriticalKeyPolicies plus the resources/nodes authority
// prefixes). A script that put/del/rm's a key under one of these bypasses the
// owner's RPC.
var scriptOwnerOwnedPrefixes = []string{
	"/globular/resources/",         // desired / release — cluster_controller
	"/globular/nodes/",             // installed / status / storage — node_agent
	"/globular/ingress/v1/spec",    // ingress spec — cluster_controller
	"/globular/system/config",      // system config — cluster_controller
	"/globular/pki/",               // PKI / CA — cluster_controller
	"/globular/objectstore/config", // objectstore config — cluster_controller
}

var etcdctlMutateRe = regexp.MustCompile(`\betcdctl\b[^#]*\b(put|del|rm)\b`)

// classifyScriptEtcdLine reports "owner_write" when a script line raw-writes an
// owner-owned etcd key via etcdctl, or "ok" otherwise. Reads (get/health),
// cluster-membership ops, comments, and mutations of non-owner keys are "ok".
func classifyScriptEtcdLine(line string) string {
	l := strings.TrimSpace(line)
	if l == "" || strings.HasPrefix(l, "#") {
		return "ok"
	}
	if !etcdctlMutateRe.MatchString(l) {
		return "ok" // not a mutating etcdctl (get/health, member list, or no etcdctl at all)
	}
	if strings.Contains(l, "member ") {
		return "ok" // cluster membership change, not a key-space write
	}
	for _, p := range scriptOwnerOwnedPrefixes {
		if strings.Contains(l, p) {
			return "owner_write"
		}
	}
	return "ok" // mutating etcdctl on a non-owner key (e.g. /globular/plans, cluster/scylla/hosts day-0 seed)
}

// scriptRawWriteBaseline is the RT-4 baseline: known steady-state scripts that
// raw-write owner-owned keys, tracked as RT-2 debt. RT-2 replaces each with a
// typed owner RPC / gated break-glass, then removes its entry here.
var scriptRawWriteBaseline = map[string]string{
	"scripts/fix-stale-plans.sh": "RT-2 debt: del ServiceRelease/InfrastructureRelease behind the live controller; migrate to a typed reset RPC / gated break-glass",
	"scripts/reset-all-plans.sh": "RT-2 debt: del ServiceRelease/InfrastructureRelease + nodes/* installed state; migrate to typed RPC / gated break-glass",
	"scripts/reset-releases.sh":  "RT-2 debt: del ServiceRelease/InfrastructureRelease; migrate to typed RPC / gated break-glass",
}

// servicesRepoRootForScripts finds the services repo root by its signature
// (a directory holding both scripts/ and golang/). Robust to go.work being off.
func servicesRepoRootForScripts(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		_, sErr := os.Stat(filepath.Join(dir, "scripts"))
		_, gErr := os.Stat(filepath.Join(dir, "golang"))
		if sErr == nil && gErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("services repo root (with scripts/ + golang/) not found from %s", dir)
		}
		dir = parent
	}
}

// TestScriptsDoNotRawWriteOwnerOwnedEtcd is the gate: no NEW script may etcdctl
// put/del/rm an owner-owned key. Known steady-state scripts are baselined.
func TestScriptsDoNotRawWriteOwnerOwnedEtcd(t *testing.T) {
	root := servicesRepoRootForScripts(t)
	scriptsDir := filepath.Join(root, "scripts")

	var violations []string
	hitBaseline := map[string]bool{}

	err := filepath.WalkDir(scriptsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".sh") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if classifyScriptEtcdLine(line) != "owner_write" {
				continue
			}
			if _, baselined := scriptRawWriteBaseline[rel]; baselined {
				hitBaseline[rel] = true
				continue
			}
			violations = append(violations, fmt.Sprintf("%s:%d", rel, i+1))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk scripts: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("script(s) raw-write owner-owned etcd keys, bypassing the owner RPC — route via the "+
			"owner's typed RPC / govops, or (if a gated operator tool) add to scriptRawWriteBaseline as "+
			"tracked RT-2 debt:\n  %s", strings.Join(violations, "\n  "))
	}

	// No-stale-baseline: every baselined script must still exist AND still contain
	// an owner-owned write. A dead entry (script migrated by RT-2 or removed) must
	// be dropped, keeping the baseline honest and shrinking as RT-2 progresses.
	for rel := range scriptRawWriteBaseline {
		if !hitBaseline[rel] {
			t.Errorf("baseline entry %q no longer raw-writes an owner-owned key (migrated or removed?) — "+
				"drop it from scriptRawWriteBaseline", rel)
		}
	}
}

// TestClassifyScriptEtcdLine pins the classifier on representative shapes — the
// behavioral proof that a new script raw write is caught and legit ops are not.
func TestClassifyScriptEtcdLine(t *testing.T) {
	ownerWrite := []string{
		`etcdctl --endpoints=$EP --cacert=$C del /globular/resources/ServiceRelease/ --prefix`,
		`etcdctl --cacert=$C del /globular/nodes/4c2b3cb3-uuid/ --prefix`,
		`etcdctl put /globular/system/config "$DATA"`,
		`etcdctl del /globular/ingress/v1/spec`,
	}
	okLines := []string{
		`etcdctl --endpoints=$EP get /globular/resources/ServiceRelease/ --prefix`, // read
		`etcdctl member remove abc123`,                                             // membership
		`etcdctl member add node https://10.0.0.5:2380`,                            // membership
		`# etcdctl del /globular/resources/ServiceRelease/ --prefix`,               // comment
		`etcdctl put /globular/plans/foo "$X"`,                                     // non-owner key
		`etcdctl put cluster/scylla/hosts "$H"`,                                    // day-0 non-owner seed
		`echo "running reconcile retry"`,                                           // no etcdctl
	}
	for _, l := range ownerWrite {
		if got := classifyScriptEtcdLine(l); got != "owner_write" {
			t.Errorf("want owner_write for %q, got %q", l, got)
		}
	}
	for _, l := range okLines {
		if got := classifyScriptEtcdLine(l); got != "ok" {
			t.Errorf("want ok for %q, got %q", l, got)
		}
	}
}

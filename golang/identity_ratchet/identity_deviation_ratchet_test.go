// Package identity_ratchet is a source-scanning ratchet (Phase 0 of the identity
// deviation program). It has no production code — only this gate.
//
// Principle (proposed meta-principle dda8d669; ai-memory architecture
// d34edd34): an identifier for a PERSISTENT ENTITY (cluster, node, service
// instance, account/group/org/app) must be an opaque, immutable token minted
// once by its owning authority and read-through everywhere — NEVER derived from
// a mutable attribute (domain, MAC, hostname, name, email, version).
//
// This gate walks golang/ and classifies lines that derive an entity identity
// from a mutable attribute. The known current violations are baselined as
// tracked debt (the identity-deviation audit inventory); the ratchet fails on
// any NEW violation, and on any STALE baseline entry (a file the migration
// phases have since fixed), so the baseline can only shrink as Phases 1-4 land.
//
// Phase 1 (cluster membership identity → minted UUID) is COMPLETE: the
// cluster_id-is-domain / isDomainLike-coercion classifiers were retired because
// cluster_id is now the sanctioned namespace and identity moved to the opaque
// minted ClusterUID. A forward guard (clusterUIDFromDomainRe) now watches the
// identity field so it can never be re-derived from the domain. Remaining classes:
// node_id from MAC/hostname (Phase 2) and service-instance id churn (Phase 4).
//
// Deliberately NOT flagged (content-addressing of immutable content is correct,
// per invariant convergence.identity_is_build_id): GenerateUUID(publisher%name%
// version) artifact keys, DNS record natural keys, cache keys. The regexes below
// target only mutable-attribute-as-ENTITY-identity.
package identity_ratchet

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ── violation classifiers ────────────────────────────────────────────────────

// serviceInstanceIDChurnRe: srv.Id = GenerateUUID(Name + ":" + Version + ":" + Mac).
// Version churns on every upgrade and Mac on every NIC/host change, so the etcd
// /globular/services/{id} registration key is reborn exactly on upgrade/migrate.
// Match requires GenerateUUID + both Version and Mac on the line (the service-id
// seed) — this cannot match a content-address (publisher%name%version has no Mac).
var serviceInstanceIDChurnRe = regexp.MustCompile(`GenerateUUID\(.*[Vv]ersion.*[Mm]ac`)

// nodeIDFromMacRe: node identity derived from MAC/hostname (mutable hardware).
//   Utility.GenerateUUID(node.Mac)              — resource/peers.go (also diverges v3/MD5)
//   uuid.NewSHA1(ns, "mac:"+mac) / "host:"+key  — controller + node-agent canonical
var nodeIDFromMacRe = regexp.MustCompile(`GenerateUUID\([^)]*\.Mac\b|NewSHA1\([^)]*"(mac|host):`)

// clusterUIDFromDomainRe: the FORWARD guard for the migrated cluster identity.
// Phase 1 resolved cluster MEMBERSHIP identity into an opaque UUID minted once by
// the controller (ClusterUID / /globular/system/cluster/id). cluster_id and
// ClusterDomain are now the SANCTIONED namespace (DNS/storage/workflow) and are
// deliberately NOT flagged — assigning cluster_id from the domain is the intended
// design, not a deviation. What must never happen is the IDENTITY field being
// derived from the mutable domain; that would re-open exactly the deviation
// Phase 1 closed (and the isDomainLike coercion that used to do it is gone).
// Expected matches: zero.
var clusterUIDFromDomainRe = regexp.MustCompile(
	`ClusterUID\s*[:=]\s*[^=].*(GetDomain\(\)|DefaultClusterDomain\(\)|ClusterDomain\b)` +
		`|ClusterUID\s*[:=]\s*"globular\.internal"`)

type idClass string

const (
	classServiceChurn         idClass = "service_instance_id_churn"
	classNodeMac              idClass = "node_id_from_mutable_attr"
	classClusterUIDFromDomain idClass = "cluster_uid_derived_from_domain"
	classOK                   idClass = "ok"
)

// classifyIdentityLine returns the violation class for one Go source line, or
// classOK. Comments are ignored.
func classifyIdentityLine(line string) idClass {
	l := strings.TrimSpace(line)
	if l == "" || strings.HasPrefix(l, "//") || strings.HasPrefix(l, "*") {
		return classOK
	}
	switch {
	case serviceInstanceIDChurnRe.MatchString(l):
		return classServiceChurn
	case nodeIDFromMacRe.MatchString(l):
		return classNodeMac
	case clusterUIDFromDomainRe.MatchString(l):
		return classClusterUIDFromDomain
	}
	return classOK
}

// ── baseline (tracked identity-deviation debt) ───────────────────────────────
//
// Files with a KNOWN identity deviation from the 2026-07-05 audit. Each is debt
// a migration phase removes. Keyed by repo-relative path so line churn doesn't
// break the gate; a file drops out the moment its last deviation is fixed.
var identityDeviationBaseline = map[string]string{
	// Phase 4 — service-instance id churn: COMPLETE. The name:version:mac seed
	// (version churned the id on every upgrade, mac on every NIC change, orphaning
	// /globular/services/{id}) is gone. All ~22 genesis sites now route through the
	// single globular_service.ServiceInstanceID authority: a deterministic,
	// version-independent id = GenerateUUID(name + ":" + nodeid.FromMAC(mac)). The
	// node part derives through the nodeid authority, so service id is a *reader* of
	// node identity, not an independent deviation — it inherits the future
	// mint-random-node_id fix for free. No Phase-4 baseline files remain.
	//
	// Phase 2 — node_id derivation: the DIVERGENCE is FIXED. resource/peers.go no
	// longer fabricates a v3/MD5 id; the controller (deterministicNodeID), the node
	// agent (StableNodeID) and the resource store all derive through the single
	// nodeid authority, so a node maps to one id everywhere. The remaining deviation
	// is that node_id is still derived from the mutable MAC/hostname rather than
	// minted opaque + read through — now confined to ONE site for a later
	// mint-random-node_id phase:
	"golang/nodeid/nodeid.go": "node_id_from_mutable_attr (Phase 2 — SINGLE canonical authority; divergence fixed; mint-random-node_id is the remaining deviation, deferred)",
	// Phase 1 — cluster membership identity → minted UUID: COMPLETE. The
	// cluster_id_is_domain / isDomainLike-coercion classifiers were retired
	// (cluster_id is now the sanctioned namespace; identity moved to the minted
	// ClusterUID, guarded going forward by clusterUIDFromDomainRe). All former
	// Phase-1-only baseline files dropped out as the class no longer exists.
}

func servicesRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		_, gErr := os.Stat(filepath.Join(dir, "golang"))
		_, sErr := os.Stat(filepath.Join(dir, "scripts"))
		if gErr == nil && sErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("services repo root not found from %s", dir)
		}
		dir = parent
	}
}

// TestNoNewIdentityDeviation is the gate. DISCOVERY MODE: set IDENTITY_RATCHET_LIST=1
// to print every current match (used to seed the baseline); the test then skips.
func TestNoNewIdentityDeviation(t *testing.T) {
	root := servicesRepoRoot(t)
	golangDir := filepath.Join(root, "golang")

	type hit struct{ loc string; class idClass }
	var all []hit
	err := filepath.WalkDir(golangDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if c := classifyIdentityLine(line); c != classOK {
				all = append(all, hit{fmt.Sprintf("%s:%d", rel, i+1), c})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk golang: %v", err)
	}

	if os.Getenv("IDENTITY_RATCHET_LIST") == "1" {
		seen := map[string]bool{}
		for _, h := range all {
			fmt.Printf("MATCH %-42s %s\n", h.class, h.loc)
		}
		fmt.Println("---- files (baseline candidates) ----")
		for _, h := range all {
			f := strings.SplitN(h.loc, ":", 2)[0]
			if !seen[f] {
				seen[f] = true
				fmt.Printf("\t%q: %q,\n", f, "identity-deviation debt: "+string(h.class))
			}
		}
		t.Skip("discovery mode: baseline candidates printed above")
	}

	var violations []string
	hitBaseline := map[string]bool{}
	for _, h := range all {
		f := strings.SplitN(h.loc, ":", 2)[0]
		if _, ok := identityDeviationBaseline[f]; ok {
			hitBaseline[f] = true
			continue
		}
		violations = append(violations, string(h.class)+" @ "+h.loc)
	}
	if len(violations) > 0 {
		t.Errorf("NEW identity deviation(s) — an entity identity derived from a mutable attribute "+
			"(domain/MAC/version/name). Mint an opaque immutable id read from its owning authority "+
			"(principle dda8d669), or if genuinely unavoidable add the file to identityDeviationBaseline "+
			"with justification:\n  %s", strings.Join(violations, "\n  "))
	}
	for f := range identityDeviationBaseline {
		if !hitBaseline[f] {
			t.Errorf("stale baseline entry %q no longer has an identity deviation (migrated?) — "+
				"drop it from identityDeviationBaseline so the ratchet keeps shrinking", f)
		}
	}
}

// TestClassifyIdentityLine pins the classifier on representative shapes.
func TestClassifyIdentityLine(t *testing.T) {
	cases := []struct {
		line string
		want idClass
	}{
		{`srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Version + ":" + srv.Mac)`, classServiceChurn},
		{`s.Id = Utility.GenerateUUID(s.GetName() + ":" + s.GetVersion() + ":" + s.GetMac())`, classServiceChurn},
		{`node.NodeId = Utility.GenerateUUID(node.Mac)`, classNodeMac},
		{`return uuid.NewSHA1(globularNodeIDNamespace, []byte("mac:"+mac)).String()`, classNodeMac},
		{`return uuid.NewSHA1(ns, []byte("host:"+key)).String()`, classNodeMac},
		{`state.ClusterUID = config.GetDomain()`, classClusterUIDFromDomain},                   // identity must never be domain-derived
		{`return &JoinPlan{ClusterUID: srv.cfg.ClusterDomain}`, classClusterUIDFromDomain},     // identity field from domain
		{`state.ClusterId = netutil.DefaultClusterDomain()`, classOK},                          // cluster_id IS the sanctioned namespace now
		{`clusterID = "globular.internal"`, classOK},                                           // namespace scoping value, not identity
		{`cv := &ClusterValidator{localClusterID: config.GetDomain()}`, classOK},               // cluster_id namespace getter, not identity
		{`id := Utility.GenerateUUID(publisher + "%" + name + "%" + version)`, classOK},        // content-address, no Mac
		{`key := Utility.GenerateUUID("A:" + domain)`, classOK},                                // DNS record natural key
		{`// state.ClusterUID = config.GetDomain()`, classOK},                                  // comment
	}
	for _, c := range cases {
		if got := classifyIdentityLine(c.line); got != c.want {
			t.Errorf("classifyIdentityLine(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

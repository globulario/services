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

// clusterIDIsDomainRe: cluster IDENTITY assigned from the mutable domain, or the
// hardcoded "globular.internal" fallback shortcut that manufactures a value where
// the authority is absent (the shortcut that HIDES the missing authority).
var clusterIDIsDomainRe = regexp.MustCompile(
	`(ClusterId|clusterID|localClusterID)\s*[:=]\s*[^=].*(GetDomain\(\)|DefaultClusterDomain\(\)|ClusterDomain\b)` +
		`|(ClusterId|clusterID|ClusterID|domain)\s*=\s*"globular\.internal"`)

// isDomainLikeCoercionRe: the state.go guard that force-overwrites a UUID cluster
// id back to the domain — the first blocker to a minted-UUID cluster identity.
var isDomainLikeCoercionRe = regexp.MustCompile(`isDomainLike\(`)

type idClass string

const (
	classServiceChurn idClass = "service_instance_id_churn"
	classNodeMac      idClass = "node_id_from_mutable_attr"
	classClusterDomain idClass = "cluster_id_is_domain_or_default_fallback"
	classDomainCoerce idClass = "cluster_id_domain_coercion"
	classOK           idClass = "ok"
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
	case isDomainLikeCoercionRe.MatchString(l):
		return classDomainCoerce
	case clusterIDIsDomainRe.MatchString(l):
		return classClusterDomain
	}
	return classOK
}

// ── baseline (tracked identity-deviation debt) ───────────────────────────────
//
// Files with a KNOWN identity deviation from the 2026-07-05 audit. Each is debt
// a migration phase removes. Keyed by repo-relative path so line churn doesn't
// break the gate; a file drops out the moment its last deviation is fixed.
var identityDeviationBaseline = map[string]string{
	// Phase 4 — service-instance id = GenerateUUID(name:version:mac) (churns on upgrade/NIC change):
	"golang/authentication/authentication_server/server.go":     "service_instance_id_churn (Phase 4)",
	"golang/cluster_controller/cluster_controller_server/main.go": "service_instance_id_churn (Phase 4)",
	"golang/cluster_doctor/cluster_doctor_server/main.go":        "service_instance_id_churn (Phase 4)",
	"golang/conversation/conversation_server/server.go":          "service_instance_id_churn (Phase 4)",
	"golang/event/event_server/server.go":                        "service_instance_id_churn (Phase 4)",
	"golang/file/file_server/server.go":                          "service_instance_id_churn (Phase 4)",
	"golang/globular_service/cli_helpers.go":                     "service_instance_id_churn (Phase 4)",
	"golang/log/log_server/server.go":                            "service_instance_id_churn (Phase 4)",
	"golang/mail/mail_server/server.go":                          "service_instance_id_churn (Phase 4)",
	"golang/media/media_server/server.go":                        "service_instance_id_churn (Phase 4)",
	"golang/monitoring/monitoring_server/server.go":              "service_instance_id_churn (Phase 4)",
	"golang/persistence/persistence_server/server.go":            "service_instance_id_churn (Phase 4)",
	"golang/rbac/rbac_server/server.go":                          "service_instance_id_churn (Phase 4)",
	"golang/search/search_server/server.go":                      "service_instance_id_churn (Phase 4)",
	"golang/sql/sql_server/server.go":                            "service_instance_id_churn (Phase 4)",
	"golang/storage/storage_server/server.go":                    "service_instance_id_churn (Phase 4)",
	"golang/title/title_server/server.go":                        "service_instance_id_churn (Phase 4)",
	"golang/torrent/torrent_server/server.go":                    "service_instance_id_churn (Phase 4)",
	// Phase 2 — node_id derived from MAC/hostname (and resource/peers.go diverges v3/MD5):
	"golang/node_agent/node_agent_server/identity/validation.go": "node_id_from_mutable_attr (Phase 2 — canonical scheme)",
	"golang/resource/resource_server/peers.go":                   "node_id_from_mutable_attr (Phase 2 — DIVERGENT v3/MD5, primary fix)",
	// Phase 1 — cluster_id = domain / "globular.internal" fallback / isDomainLike coercion:
	"golang/cluster_controller/cluster_controller_server/server.go":                 "cluster_id_is_domain + node_id_from_mac (Phase 1/2)",
	"golang/cluster_controller/cluster_controller_server/state.go":                  "cluster_id_is_domain + isDomainLike coercion (Phase 1 — first blocker)",
	"golang/cluster_controller/cluster_controller_server/reconcile_nodes.go":        "cluster_id_is_domain (Phase 1)",
	"golang/cluster_controller/cluster_controller_server/workflow_client.go":        "cluster_id_is_domain (Phase 1)",
	"golang/cluster_controller/cluster_controller_server/workflow_execute.go":       "cluster_id_is_domain (Phase 1)",
	"golang/dns/dns_client/dns_client.go":                                           "cluster_id_is_domain (Phase 1)",
	"golang/globularcli/workflow_cmds.go":                                           "cluster_id_is_domain (Phase 1)",
	"golang/mcp/tools_workflow.go":                                                  "cluster_id_is_domain (Phase 1)",
	"golang/node_agent/node_agent_server/main.go":                                   "cluster_id_is_domain (Phase 1)",
	"golang/node_agent/node_agent_server/scylla_manager_agent_config.go":            "cluster_id_is_domain fallback (Phase 1 — keep domain-seeded token, drop hardcoded fallback)",
	"golang/workflow/workflow_server/import_day0.go":                                "cluster_id_is_domain (Phase 1)",
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
		{`return &ClusterValidator{localClusterID: domain}`, classOK}, // assigns from var 'domain', not GetDomain()/ClusterDomain
		{`cv := &ClusterValidator{localClusterID: config.GetDomain()}`, classClusterDomain},
		{`if cfg.ClusterDomain == "" { cfg.ClusterDomain = domain }`, classOK}, // domain is the DNS attribute here, legit
		{`clusterID = "globular.internal"`, classClusterDomain},
		{`if state.ClusterId == "" || !isDomainLike(state.ClusterId) {`, classDomainCoerce},
		{`id := Utility.GenerateUUID(publisher + "%" + name + "%" + version)`, classOK}, // content-address, no Mac
		{`key := Utility.GenerateUUID("A:" + domain)`, classOK}, // DNS record natural key
		{`// srv.Id = GenerateUUID(Name + ":" + Version + ":" + Mac)`, classOK}, // comment
	}
	for _, c := range cases {
		if got := classifyIdentityLine(c.line); got != c.want {
			t.Errorf("classifyIdentityLine(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

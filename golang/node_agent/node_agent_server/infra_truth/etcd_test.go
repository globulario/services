package infra_truth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// etcdJoiningDesired returns a desired state for a node joining an existing
// etcd cluster.
func etcdJoiningDesired() *InfraDesiredState {
	return &InfraDesiredState{
		Component:               ComponentEtcd,
		NodeID:                  "globule-ryzen",
		ClusterID:               "test-cluster",
		Source:                  SourceComputedFromMembership,
		ExpectedListenAddresses: []string{"10.0.0.63"},
		ExpectedPeers:           []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		ExpectedSeeds:           []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		ExpectedClusterName:     "test-cluster-etcd-cluster",
		BootstrapIntent:         BootstrapJoining,
	}
}

// validEtcdRendered is a correctly rendered etcd config for a joining node.
func validEtcdRendered() *EtcdRenderedConfig {
	return &EtcdRenderedConfig{
		Path:                     EtcdConfigPath,
		Present:                  true,
		Name:                     "globule-ryzen",
		DataDir:                  "/var/lib/globular/etcd",
		ListenClientURLs:         []string{"https://10.0.0.63:2379"},
		AdvertiseClientURLs:      []string{"https://10.0.0.63:2379"},
		ListenPeerURLs:           []string{"https://10.0.0.63:2380"},
		InitialAdvertisePeerURLs: []string{"https://10.0.0.63:2380"},
		InitialClusterState:      "existing",
		InitialClusterToken:      "test-cluster-etcd-cluster",
		InitialCluster: map[string]string{
			"globule-ryzen": "https://10.0.0.63:2380",
			"globule-nuc":   "https://10.0.0.8:2380",
			"globule-dell":  "https://10.0.0.20:2380",
		},
		InitialClusterNames: []string{"globule-dell", "globule-nuc", "globule-ryzen"},
		PeerCertFile:        "/var/lib/globular/pki/issued/services/service.crt",
		PeerKeyFile:         "/var/lib/globular/pki/issued/services/service.key",
		PeerTrustedCA:       "/var/lib/globular/pki/ca.crt",
		ClientCertFile:      "/var/lib/globular/pki/issued/services/service.crt",
		ClientKeyFile:       "/var/lib/globular/pki/issued/services/service.key",
	}
}

// ── parser ───────────────────────────────────────────────────────────────────

func TestParseEtcdYAML_GeneratedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "etcd.yaml")
	content := `name: "globule-ryzen"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "https://10.0.0.63:2379"
advertise-client-urls: "https://10.0.0.63:2379"
listen-peer-urls: "https://10.0.0.63:2380"
initial-advertise-peer-urls: "https://10.0.0.63:2380"
initial-cluster: "globule-ryzen=https://10.0.0.63:2380,globule-nuc=https://10.0.0.8:2380"
initial-cluster-state: "existing"
initial-cluster-token: "test-cluster-etcd-cluster"

client-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key

peer-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key
  trusted-ca-file: /var/lib/globular/pki/ca.crt
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseEtcdYAML(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.Present {
		t.Fatal("expected Present=true")
	}
	if cfg.Name != "globule-ryzen" {
		t.Errorf("name=%q", cfg.Name)
	}
	if cfg.InitialClusterToken != "test-cluster-etcd-cluster" {
		t.Errorf("token=%q", cfg.InitialClusterToken)
	}
	if got := cfg.InitialCluster["globule-nuc"]; got != "https://10.0.0.8:2380" {
		t.Errorf("initial-cluster[nuc]=%q", got)
	}
	if len(cfg.InitialClusterNames) != 2 {
		t.Errorf("initial-cluster names=%v", cfg.InitialClusterNames)
	}
	if cfg.PeerTrustedCA == "" || cfg.PeerCertFile == "" || cfg.PeerKeyFile == "" {
		t.Errorf("peer TLS not parsed: %+v", cfg)
	}
	if h := hostFromURL(cfg.AdvertiseClientURLs[0]); h != "10.0.0.63" {
		t.Errorf("advertise host=%q", h)
	}
}

func TestParseEtcdYAML_Missing(t *testing.T) {
	cfg, err := parseEtcdYAML(filepath.Join(t.TempDir(), "no-such.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if cfg.Present {
		t.Fatal("expected Present=false for missing file")
	}
}

// ── attestation ──────────────────────────────────────────────────────────────

func TestAttestEtcdConfig_Valid(t *testing.T) {
	v := AttestEtcdConfig(etcdJoiningDesired(), validEtcdRendered())
	if len(v) != 0 {
		t.Fatalf("expected no violations, got %d: %+v", len(v), v)
	}
}

func TestAttestEtcdConfig_LoopbackPeerURL(t *testing.T) {
	r := validEtcdRendered()
	r.ListenPeerURLs = []string{"https://127.0.0.1:2380"}
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL etcd.loopback_forbidden, got %+v", v)
	}
}

// TestAttestEtcdConfig_UnspecifiedListenURLs_OK pins the Day-1 join contract:
// listen-peer-urls / listen-client-urls are bind addresses, and 0.0.0.0 (bind
// every interface) is the correct, renderer-produced value — it must NOT raise
// etcd.loopback_forbidden. A false CRITICAL here stalls a healthy joining etcd
// member and blocks the node's service-layer convergence (globule-nuc, 2026-06-20:
// node had a 2-member raft with a leader yet never installed any SERVICE package).
func TestAttestEtcdConfig_UnspecifiedListenURLs_OK(t *testing.T) {
	r := validEtcdRendered()
	r.ListenPeerURLs = []string{"https://0.0.0.0:2380"}
	r.ListenClientURLs = []string{"https://0.0.0.0:2379"}
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if containsViolation(v, "etcd.loopback_forbidden", SeverityCritical) {
		t.Fatalf("listen-*-urls=0.0.0.0 is a valid bind address and must not be flagged loopback_forbidden, got %+v", v)
	}
}

// TestAttestEtcdConfig_LoopbackListenClientURL keeps the genuine failure guarded:
// binding listen-client-urls to loopback alone isolates the member, so it must
// still be CRITICAL even though unspecified is now allowed on listen fields.
func TestAttestEtcdConfig_LoopbackListenClientURL(t *testing.T) {
	r := validEtcdRendered()
	r.ListenClientURLs = []string{"https://127.0.0.1:2379"}
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL etcd.loopback_forbidden for loopback listen-client-urls, got %+v", v)
	}
}

func TestAttestEtcdConfig_UnspecifiedAdvertiseClientURL(t *testing.T) {
	r := validEtcdRendered()
	r.AdvertiseClientURLs = []string{"https://0.0.0.0:2379"}
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL for unspecified advertise-client-urls, got %+v", v)
	}
}

func TestAttestEtcdConfig_SelfNotInInitialCluster(t *testing.T) {
	r := validEtcdRendered()
	delete(r.InitialCluster, "globule-ryzen")
	r.InitialClusterNames = []string{"globule-dell", "globule-nuc"}
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.config_valid", SeverityError) {
		t.Fatalf("expected ERROR etcd.config_valid for self absent from initial-cluster, got %+v", v)
	}
}

func TestAttestEtcdConfig_SelfOnlyInitialCluster_JoiningNode(t *testing.T) {
	r := validEtcdRendered()
	r.InitialCluster = map[string]string{"globule-ryzen": "https://10.0.0.63:2380"}
	r.InitialClusterNames = []string{"globule-ryzen"}
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.config_valid", SeverityError) {
		t.Fatalf("expected ERROR etcd.config_valid for self-only initial-cluster on a joining node, got %+v", v)
	}
}

func TestAttestEtcdConfig_SelfOnlyInitialCluster_FirstNodeAllowed(t *testing.T) {
	d := etcdJoiningDesired()
	d.BootstrapIntent = BootstrapFirstNode
	d.ExpectedPeers = []string{"10.0.0.63"}
	r := validEtcdRendered()
	r.InitialCluster = map[string]string{"globule-ryzen": "https://10.0.0.63:2380"}
	r.InitialClusterNames = []string{"globule-ryzen"}
	v := AttestEtcdConfig(d, r)
	if containsViolation(v, "etcd.config_valid", SeverityError) {
		t.Fatalf("self-only initial-cluster must be allowed for first-node, got %+v", v)
	}
}

func TestAttestEtcdConfig_EmptyName(t *testing.T) {
	r := validEtcdRendered()
	r.Name = ""
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.config_valid", SeverityError) {
		t.Fatalf("expected ERROR etcd.config_valid for empty name, got %+v", v)
	}
}

func TestAttestEtcdConfig_TokenMismatch(t *testing.T) {
	r := validEtcdRendered()
	r.InitialClusterToken = "some-other-token"
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.config_valid", SeverityError) {
		t.Fatalf("expected ERROR etcd.config_valid for token mismatch, got %+v", v)
	}
}

func TestAttestEtcdConfig_MissingPeerTrustedCA(t *testing.T) {
	r := validEtcdRendered()
	r.PeerTrustedCA = ""
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if !containsViolation(v, "etcd.config_valid", SeverityError) {
		t.Fatalf("expected ERROR etcd.config_valid for missing peer trusted-ca, got %+v", v)
	}
}

func TestAttestEtcdConfig_NotPresentNoViolations(t *testing.T) {
	v := AttestEtcdConfig(etcdJoiningDesired(), &EtcdRenderedConfig{Path: EtcdConfigPath, Present: false})
	if len(v) != 0 {
		t.Fatalf("absent config must yield no config violations, got %+v", v)
	}
}

func TestAttestEtcdConfig_RemediationTargetsOwnerNotManualEdit(t *testing.T) {
	r := validEtcdRendered()
	r.ListenPeerURLs = []string{"https://127.0.0.1:2380"} // force a violation
	v := AttestEtcdConfig(etcdJoiningDesired(), r)
	if len(v) == 0 {
		t.Fatal("expected at least one violation to inspect remediation")
	}
	for _, viol := range v {
		rem := viol.GetRemediation()
		if !strings.Contains(rem, "renderer") {
			t.Errorf("violation %q remediation must target the config owner (renderer); got: %q", viol.GetId(), rem)
		}
		if !strings.Contains(rem, "Do NOT hand-edit") {
			t.Errorf("violation %q remediation must forbid manual etcd.yaml edits; got: %q", viol.GetId(), rem)
		}
	}
}

// ── desired ──────────────────────────────────────────────────────────────────

func TestBuildEtcdDesiredState_RequiresMinimumFacts(t *testing.T) {
	if _, err := BuildEtcdDesiredState(EtcdDesiredInputs{LocalIP: "10.0.0.63"}); err == nil {
		t.Error("expected error when node id is empty")
	}
	if _, err := BuildEtcdDesiredState(EtcdDesiredInputs{NodeID: "n"}); err == nil {
		t.Error("expected error when local IP is empty")
	}
}

func TestBuildEtcdDesiredState_BootstrapIntent(t *testing.T) {
	joining, err := BuildEtcdDesiredState(EtcdDesiredInputs{
		NodeID: "n", LocalIP: "10.0.0.63", Peers: []string{"10.0.0.63", "10.0.0.8"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if joining.BootstrapIntent != BootstrapJoining {
		t.Errorf("expected joining, got %q", joining.BootstrapIntent)
	}
	// etcd has no separate seed concept: seeds mirror the peer set.
	if strings.Join(joining.ExpectedSeeds, ",") != strings.Join(joining.ExpectedPeers, ",") {
		t.Errorf("expected seeds==peers, got seeds=%v peers=%v", joining.ExpectedSeeds, joining.ExpectedPeers)
	}

	first, err := BuildEtcdDesiredState(EtcdDesiredInputs{
		NodeID: "n", LocalIP: "10.0.0.63", Peers: []string{"10.0.0.63"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.BootstrapIntent != BootstrapFirstNode {
		t.Errorf("expected first-node, got %q", first.BootstrapIntent)
	}
}

// ── lifecycle ────────────────────────────────────────────────────────────────

func reachableVoter() *EtcdRuntimeState {
	return &EtcdRuntimeState{DaemonActive: true, LocalReachable: true, HasLeader: true, MemberCount: 3}
}

func TestDeriveEtcdLifecycle_States(t *testing.T) {
	rendered := validEtcdRendered()
	critical := []*cluster_controllerpb.InfraViolation{newViolation("etcd.loopback_forbidden", SeverityCritical, "m", "e", "r")}
	errViol := []*cluster_controllerpb.InfraViolation{newViolation("etcd.config_valid", SeverityError, "m", "e", "r")}

	cases := []struct {
		name      string
		installed bool
		rendered  *EtcdRenderedConfig
		runtime   *EtcdRuntimeState
		viol      []*cluster_controllerpb.InfraViolation
		want      cluster_controllerpb.InfraLifecycleState
	}{
		{"not installed", false, nil, nil, nil, cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT},
		{"no config", true, &EtcdRenderedConfig{Present: false}, nil, nil, cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED},
		{"critical config stalls", true, rendered, reachableVoter(), critical, cluster_controllerpb.InfraLifecycleState_INFRA_STALLED},
		{"daemon down", true, rendered, &EtcdRuntimeState{DaemonActive: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED},
		{"corrupt stalls", true, rendered, &EtcdRuntimeState{DaemonActive: true, LocalReachable: true, HasLeader: true, Alarms: []string{"CORRUPT"}}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_STALLED},
		{"not reachable", true, rendered, &EtcdRuntimeState{DaemonActive: true, LocalReachable: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING},
		{"learner joining", true, rendered, &EtcdRuntimeState{DaemonActive: true, LocalReachable: true, HasLeader: true, IsLearner: true}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_JOINING},
		{"no leader degraded", true, rendered, &EtcdRuntimeState{DaemonActive: true, LocalReachable: true, HasLeader: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"nospace degraded", true, rendered, &EtcdRuntimeState{DaemonActive: true, LocalReachable: true, HasLeader: true, Alarms: []string{"NOSPACE"}}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"error violation degraded", true, rendered, reachableVoter(), errViol, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"member ready", true, rendered, reachableVoter(), nil, cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obs := deriveEtcdLifecycle(c.installed, etcdJoiningDesired(), c.rendered, c.runtime, c.viol, 0)
			if obs.GetState() != c.want {
				t.Fatalf("state=%s want=%s (blocking=%q)", obs.GetStateLabel(), lifecycleLabel(c.want), obs.GetBlockingReason())
			}
		})
	}
}

// ── ProbeStructured ──────────────────────────────────────────────────────────

func writeValidEtcdYAML(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "etcd.yaml")
	content := `name: "globule-ryzen"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "https://10.0.0.63:2379"
advertise-client-urls: "https://10.0.0.63:2379"
listen-peer-urls: "https://10.0.0.63:2380"
initial-advertise-peer-urls: "https://10.0.0.63:2380"
initial-cluster: "globule-ryzen=https://10.0.0.63:2380,globule-nuc=https://10.0.0.8:2380,globule-dell=https://10.0.0.20:2380"
initial-cluster-state: "existing"
initial-cluster-token: "test-cluster-etcd-cluster"

peer-transport-security:
  cert-file: /var/lib/globular/pki/issued/services/service.crt
  key-file: /var/lib/globular/pki/issued/services/service.key
  trusted-ca-file: /var/lib/globular/pki/ca.crt
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEtcdProbeStructured_NotInstalled(t *testing.T) {
	p := &EtcdProber{
		ConfigPath:      filepath.Join(t.TempDir(), "etcd.yaml"),
		DetectInstalled: func(context.Context) bool { return false },
		NowUnix:         func() int64 { return 0 },
	}
	res := p.ProbeStructured(context.Background(), etcdJoiningDesired(), nil)
	if res.GetInstalled() || res.GetHealthy() {
		t.Fatalf("expected not installed / not healthy, got %+v", res)
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT {
		t.Fatalf("lifecycle=%s", res.GetLifecycle().GetStateLabel())
	}
}

func TestEtcdProbeStructured_HealthyMember(t *testing.T) {
	path := writeValidEtcdYAML(t)
	var observedURL string
	p := &EtcdProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		Observe: func(_ context.Context, localURL string) *EtcdRuntimeState {
			observedURL = localURL
			return &EtcdRuntimeState{
				LocalReachable: true, HasLeader: true, IsLeader: true, MemberCount: 3,
				ObservedPeers: []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
			}
		},
	}
	res := p.ProbeStructured(context.Background(), etcdJoiningDesired(), nil)
	if !res.GetInstalled() || !res.GetConfigValid() || !res.GetHealthy() {
		t.Fatalf("expected installed+valid+healthy, got installed=%t valid=%t healthy=%t violations=%+v",
			res.GetInstalled(), res.GetConfigValid(), res.GetHealthy(), res.GetViolations())
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY {
		t.Fatalf("lifecycle=%s blocking=%q", res.GetLifecycle().GetStateLabel(), res.GetLifecycle().GetBlockingReason())
	}
	if !res.GetPeersMatch() {
		t.Errorf("expected peers_match=true")
	}
	// The observer must be handed the advertised client URL, not a loopback.
	if observedURL != "https://10.0.0.63:2379" {
		t.Errorf("observer dialed %q, expected the advertised client URL", observedURL)
	}
}

func TestEtcdProbeStructured_ObserverMissingIsExplicit(t *testing.T) {
	path := writeValidEtcdYAML(t)
	p := &EtcdProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		// Observe intentionally nil — runtime truth must be reported unavailable,
		// never fabricated as healthy.
	}
	res := p.ProbeStructured(context.Background(), etcdJoiningDesired(), nil)
	if res.GetHealthy() {
		t.Fatal("a member with no runtime observation must not be reported healthy")
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING {
		t.Fatalf("expected DAEMON_STARTING when runtime is unobserved, got %s", res.GetLifecycle().GetStateLabel())
	}
	if len(res.GetErrors()) == 0 {
		t.Error("expected an explicit error documenting the missing observer")
	}
}

func TestEtcdProbeStructured_DesiredUnavailableIsViolation(t *testing.T) {
	path := writeValidEtcdYAML(t)
	p := &EtcdProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return false },
		NowUnix:         func() int64 { return 0 },
	}
	res := p.ProbeStructured(context.Background(), nil, context.DeadlineExceeded)
	if !containsViolation(res.GetViolations(), "infra.desired_state_unavailable", SeverityError) {
		t.Fatalf("expected infra.desired_state_unavailable violation, got %+v", res.GetViolations())
	}
}

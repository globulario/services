package infra_truth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// minioDistributedDesired returns a 3-node distributed desired state, 1 drive/node.
func minioDistributedDesired() *MinioDesired {
	d, err := BuildMinioDesiredState(MinioDesiredInputs{
		NodeID:        "globule-ryzen",
		ClusterID:     "test-cluster",
		LocalIP:       "10.0.0.63",
		Mode:          MinioModeDistributed,
		Nodes:         []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		DrivesPerNode: 1,
	})
	if err != nil {
		panic(err)
	}
	return d
}

// validMinioDistributedRendered is the matching distributed env (3 volume URLs).
func validMinioDistributedRendered() *MinioRenderedConfig {
	return &MinioRenderedConfig{
		Path:        MinioConfigPath,
		Present:     true,
		Mode:        MinioModeDistributed,
		VolumeCount: 3,
		Volumes: []string{
			"https://10.0.0.63:9000/var/lib/globular/minio/data",
			"https://10.0.0.8:9000/var/lib/globular/minio/data",
			"https://10.0.0.20:9000/var/lib/globular/minio/data",
		},
		Endpoints:   []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		HasRootUser: true,
		CICD:        true,
	}
}

// ── parser ───────────────────────────────────────────────────────────────────

func TestParseMinioEnv_Distributed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minio.env")
	content := `MINIO_VOLUMES=https://10.0.0.63:9000/var/lib/globular/minio/data https://10.0.0.8:9000/var/lib/globular/minio/data https://10.0.0.20:9000/var/lib/globular/minio/data
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=secret
MINIO_CI_CD=1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseMinioEnv(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.Present || cfg.Mode != MinioModeDistributed {
		t.Fatalf("present=%t mode=%q", cfg.Present, cfg.Mode)
	}
	if cfg.VolumeCount != 3 {
		t.Errorf("volume_count=%d", cfg.VolumeCount)
	}
	if len(cfg.Endpoints) != 3 || cfg.Endpoints[0] != "10.0.0.63" {
		t.Errorf("endpoints=%v", cfg.Endpoints)
	}
	if !cfg.HasRootUser {
		t.Error("expected HasRootUser=true (presence only — value never captured)")
	}
}

func TestParseMinioEnv_Standalone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minio.env")
	content := "MINIO_VOLUMES=/var/lib/globular/minio/data\nMINIO_ROOT_USER=minioadmin\nMINIO_ROOT_PASSWORD=x\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := parseMinioEnv(path)
	if cfg.Mode != MinioModeStandalone {
		t.Errorf("mode=%q want standalone", cfg.Mode)
	}
	if cfg.VolumeCount != 1 || len(cfg.Endpoints) != 0 {
		t.Errorf("volume_count=%d endpoints=%v", cfg.VolumeCount, cfg.Endpoints)
	}
}

func TestParseMinioEnv_Missing(t *testing.T) {
	cfg, err := parseMinioEnv(filepath.Join(t.TempDir(), "no-such.env"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if cfg.Present {
		t.Fatal("expected Present=false for missing file")
	}
}

// ── desired ──────────────────────────────────────────────────────────────────

func TestBuildMinioDesiredState_VolumeCount(t *testing.T) {
	d := minioDistributedDesired()
	if got := d.ExpectedVolumeCount(); got != 3 {
		t.Errorf("expected 3 volumes (3 nodes × 1 drive), got %d", got)
	}
	multi, _ := BuildMinioDesiredState(MinioDesiredInputs{
		NodeID: "n", LocalIP: "10.0.0.63",
		Mode: MinioModeDistributed, Nodes: []string{"10.0.0.63", "10.0.0.8"}, DrivesPerNode: 4,
	})
	if got := multi.ExpectedVolumeCount(); got != 8 {
		t.Errorf("expected 8 volumes (2 nodes × 4 drives), got %d", got)
	}
}

func TestBuildMinioDesiredState_RequiresMinimumFacts(t *testing.T) {
	if _, err := BuildMinioDesiredState(MinioDesiredInputs{LocalIP: "10.0.0.63"}); err == nil {
		t.Error("expected error when node id empty")
	}
	if _, err := BuildMinioDesiredState(MinioDesiredInputs{NodeID: "n"}); err == nil {
		t.Error("expected error when local IP empty")
	}
}

// ── attestation ──────────────────────────────────────────────────────────────

func TestAttestMinioConfig_Valid(t *testing.T) {
	v := AttestMinioConfig(minioDistributedDesired(), validMinioDistributedRendered())
	if len(v) != 0 {
		t.Fatalf("expected no violations, got %d: %+v", len(v), v)
	}
}

func TestAttestMinioConfig_LoopbackVolume(t *testing.T) {
	r := validMinioDistributedRendered()
	r.Volumes[0] = "https://127.0.0.1:9000/var/lib/globular/minio/data"
	r.Endpoints = []string{"127.0.0.1", "10.0.0.8", "10.0.0.20"}
	v := AttestMinioConfig(minioDistributedDesired(), r)
	if !containsViolation(v, "minio.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL minio.loopback_forbidden, got %+v", v)
	}
}

func TestAttestMinioConfig_SplitBrainStandaloneInCluster(t *testing.T) {
	// desired distributed, rendered standalone (single local path) → CRITICAL.
	r := &MinioRenderedConfig{
		Path: MinioConfigPath, Present: true, Mode: MinioModeStandalone,
		VolumeCount: 1, Volumes: []string{"/var/lib/globular/minio/data"},
	}
	v := AttestMinioConfig(minioDistributedDesired(), r)
	if !containsViolation(v, "minio.topology_matches_desired", SeverityCritical) {
		t.Fatalf("expected CRITICAL minio.topology_matches_desired (split-brain), got %+v", v)
	}
}

func TestAttestMinioConfig_DriveCountMismatch(t *testing.T) {
	// desired 3 volumes, rendered 2 → format.json blast-radius CRITICAL.
	r := validMinioDistributedRendered()
	r.Volumes = r.Volumes[:2]
	r.VolumeCount = 2
	r.Endpoints = []string{"10.0.0.63", "10.0.0.8"}
	v := AttestMinioConfig(minioDistributedDesired(), r)
	if !containsViolation(v, "minio.topology_matches_desired", SeverityCritical) {
		t.Fatalf("expected CRITICAL minio.topology_matches_desired (drive count), got %+v", v)
	}
}

func TestAttestMinioConfig_SelfNotInPool(t *testing.T) {
	r := validMinioDistributedRendered()
	r.Volumes[0] = "https://10.0.0.99:9000/var/lib/globular/minio/data" // self (.63) gone, .99 instead
	r.Endpoints = []string{"10.0.0.99", "10.0.0.8", "10.0.0.20"}
	v := AttestMinioConfig(minioDistributedDesired(), r)
	if !containsViolation(v, "minio.config_valid", SeverityError) {
		t.Fatalf("expected ERROR minio.config_valid for self absent from pool, got %+v", v)
	}
}

func TestAttestMinioConfig_EmptyVolumes(t *testing.T) {
	r := &MinioRenderedConfig{Path: MinioConfigPath, Present: true, Mode: MinioModeStandalone, VolumeCount: 0}
	v := AttestMinioConfig(minioDistributedDesired(), r)
	if !containsViolation(v, "minio.config_valid", SeverityError) {
		t.Fatalf("expected ERROR minio.config_valid for empty volumes, got %+v", v)
	}
}

func TestAttestMinioConfig_NotPresentNoViolations(t *testing.T) {
	v := AttestMinioConfig(minioDistributedDesired(), &MinioRenderedConfig{Path: MinioConfigPath, Present: false})
	if len(v) != 0 {
		t.Fatalf("absent config must yield no config violations, got %+v", v)
	}
}

func TestAttestMinioConfig_RemediationTargetsOwnerNotManualEdit(t *testing.T) {
	r := validMinioDistributedRendered()
	r.Volumes[0] = "https://127.0.0.1:9000/data" // force a violation
	r.Endpoints = []string{"127.0.0.1", "10.0.0.8", "10.0.0.20"}
	v := AttestMinioConfig(minioDistributedDesired(), r)
	if len(v) == 0 {
		t.Fatal("expected at least one violation")
	}
	for _, viol := range v {
		rem := viol.GetRemediation()
		if !strings.Contains(rem, "RenderMinioEnv") && !strings.Contains(rem, "ObjectStoreDesiredState") {
			t.Errorf("violation %q remediation must target the config owner; got: %q", viol.GetId(), rem)
		}
		if !strings.Contains(rem, "Do NOT hand-edit") {
			t.Errorf("violation %q remediation must forbid manual minio.env edits; got: %q", viol.GetId(), rem)
		}
	}
}

// ── lifecycle ────────────────────────────────────────────────────────────────

func TestDeriveMinioLifecycle_States(t *testing.T) {
	rendered := validMinioDistributedRendered()
	critical := []*cluster_controllerpb.InfraViolation{newViolation("minio.topology_matches_desired", SeverityCritical, "m", "e", "r")}
	errViol := []*cluster_controllerpb.InfraViolation{newViolation("minio.config_valid", SeverityError, "m", "e", "r")}

	live := func() *MinioRuntimeState {
		return &MinioRuntimeState{DaemonActive: true, Live: true, WriteQuorum: true, ReadQuorum: true}
	}

	cases := []struct {
		name      string
		installed bool
		rendered  *MinioRenderedConfig
		runtime   *MinioRuntimeState
		viol      []*cluster_controllerpb.InfraViolation
		want      cluster_controllerpb.InfraLifecycleState
	}{
		{"not installed", false, nil, nil, nil, cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT},
		{"no config", true, &MinioRenderedConfig{Present: false}, nil, nil, cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED},
		{"critical config stalls", true, rendered, live(), critical, cluster_controllerpb.InfraLifecycleState_INFRA_STALLED},
		{"daemon down", true, rendered, &MinioRuntimeState{DaemonActive: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED},
		{"not live", true, rendered, &MinioRuntimeState{DaemonActive: true, Live: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING},
		{"no write quorum degraded", true, rendered, &MinioRuntimeState{DaemonActive: true, Live: true, WriteQuorum: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"no read quorum degraded", true, rendered, &MinioRuntimeState{DaemonActive: true, Live: true, WriteQuorum: true, ReadQuorum: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"error violation degraded", true, rendered, live(), errViol, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"member ready", true, rendered, live(), nil, cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obs := deriveMinioLifecycle(c.installed, minioDistributedDesired(), c.rendered, c.runtime, c.viol, 0)
			if obs.GetState() != c.want {
				t.Fatalf("state=%s want=%s (blocking=%q)", obs.GetStateLabel(), lifecycleLabel(c.want), obs.GetBlockingReason())
			}
		})
	}
}

// ── ProbeStructured ──────────────────────────────────────────────────────────

func writeValidMinioEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "minio.env")
	content := `MINIO_VOLUMES=https://10.0.0.63:9000/var/lib/globular/minio/data https://10.0.0.8:9000/var/lib/globular/minio/data https://10.0.0.20:9000/var/lib/globular/minio/data
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=secret
MINIO_CI_CD=1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMinioProbeStructured_HealthyMember(t *testing.T) {
	path := writeValidMinioEnv(t)
	var observedBase string
	p := &MinioProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		Observe: func(_ context.Context, base string) *MinioRuntimeState {
			observedBase = base
			return &MinioRuntimeState{Live: true, WriteQuorum: true, ReadQuorum: true}
		},
	}
	res := p.ProbeStructured(context.Background(), minioDistributedDesired(), nil)
	if !res.GetInstalled() || !res.GetConfigValid() || !res.GetHealthy() {
		t.Fatalf("expected installed+valid+healthy, got installed=%t valid=%t healthy=%t violations=%+v",
			res.GetInstalled(), res.GetConfigValid(), res.GetHealthy(), res.GetViolations())
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY {
		t.Fatalf("lifecycle=%s blocking=%q", res.GetLifecycle().GetStateLabel(), res.GetLifecycle().GetBlockingReason())
	}
	if observedBase != "https://10.0.0.63:9000" {
		t.Errorf("observer dialed %q, expected the local node health base URL", observedBase)
	}
}

func TestMinioProbeStructured_NoWriteQuorumDegraded(t *testing.T) {
	path := writeValidMinioEnv(t)
	p := &MinioProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		Observe: func(_ context.Context, _ string) *MinioRuntimeState {
			return &MinioRuntimeState{Live: true, WriteQuorum: false}
		},
	}
	res := p.ProbeStructured(context.Background(), minioDistributedDesired(), nil)
	if res.GetHealthy() {
		t.Fatal("a member with no write quorum must not be reported healthy")
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED {
		t.Fatalf("expected DEGRADED, got %s", res.GetLifecycle().GetStateLabel())
	}
}

func TestMinioProbeStructured_ObserverMissingIsExplicit(t *testing.T) {
	path := writeValidMinioEnv(t)
	p := &MinioProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		// Observe intentionally nil.
	}
	res := p.ProbeStructured(context.Background(), minioDistributedDesired(), nil)
	if res.GetHealthy() {
		t.Fatal("a member with no runtime observation must not be reported healthy")
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING {
		t.Fatalf("expected DAEMON_STARTING when runtime unobserved, got %s", res.GetLifecycle().GetStateLabel())
	}
	if len(res.GetErrors()) == 0 {
		t.Error("expected an explicit error documenting the missing observer")
	}
}

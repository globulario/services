package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func newTestInfraRelease(pub, component, ver, digest string) *cluster_controllerpb.InfrastructureRelease {
	return &cluster_controllerpb.InfrastructureRelease{
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			PublisherID: pub,
			Component:   component,
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{
			ResolvedVersion:        ver,
			ResolvedArtifactDigest: digest,
		},
	}
}

func TestCompileInfrastructurePlan_Basic(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "etcd", "3.5.14", "sha256abc")
	plan, err := CompileInfrastructurePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Reason != "infrastructure_release" {
		t.Errorf("reason = %q, want infrastructure_release", plan.Reason)
	}
	if plan.NodeId != "node-1" {
		t.Errorf("node_id = %q, want node-1", plan.NodeId)
	}

	// Steps: service.stop, artifact.fetch, artifact.verify, infrastructure.install, package.report_state, service.restart
	if len(plan.Spec.Steps) != 6 {
		t.Fatalf("steps = %d, want 6", len(plan.Spec.Steps))
	}
	expectedActions := []string{
		"service.stop", "artifact.fetch", "artifact.verify",
		"infrastructure.install", "package.report_state", "service.restart",
	}
	for i, want := range expectedActions {
		if plan.Spec.Steps[i].Action != want {
			t.Errorf("step[%d].action = %q, want %q", i, plan.Spec.Steps[i].Action, want)
		}
	}

	// No rollback when no installed version.
	if len(plan.Spec.Rollback) != 0 {
		t.Errorf("rollback steps = %d, want 0", len(plan.Spec.Rollback))
	}

	// Lock format.
	if len(plan.Locks) != 1 || plan.Locks[0] != "infrastructure:etcd" {
		t.Errorf("locks = %v, want [infrastructure:etcd]", plan.Locks)
	}

	// Default unit name.
	stopStep := plan.Spec.Steps[0]
	unit := stopStep.Args.GetFields()["unit"].GetStringValue()
	if unit != "globular-etcd.service" {
		t.Errorf("unit = %q, want globular-etcd.service", unit)
	}
}

func TestCompileInfrastructurePlan_WithRollback(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "minio", "2024.01.01", "def456")
	plan, err := CompileInfrastructurePlan("node-1", rel, "2023.12.01", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// Rollback: service.stop, artifact.fetch, artifact.verify, infrastructure.install, package.report_state, service.restart
	if len(plan.Spec.Rollback) != 6 {
		t.Fatalf("rollback steps = %d, want 6", len(plan.Spec.Rollback))
	}

	// Verify rollback installs previous version.
	installStep := plan.Spec.Rollback[3]
	ver := installStep.Args.GetFields()["version"].GetStringValue()
	if ver != "2023.12.01" {
		t.Errorf("rollback version = %q, want 2023.12.01", ver)
	}
}

func TestCompileInfrastructurePlan_SameVersionNoRollback(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "etcd", "3.5.14", "aaa")
	plan, err := CompileInfrastructurePlan("node-1", rel, "3.5.14", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Spec.Rollback) != 0 {
		t.Errorf("rollback steps = %d, want 0 (same version)", len(plan.Spec.Rollback))
	}
}

func TestCompileInfrastructurePlan_CustomUnit(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "envoy", "1.28.0", "abc123")
	rel.Spec.Unit = "globular-envoy.service" // envoy uses the globular- prefixed unit name

	plan, err := CompileInfrastructurePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	stopStep := plan.Spec.Steps[0]
	unit := stopStep.Args.GetFields()["unit"].GetStringValue()
	if unit != "globular-envoy.service" {
		t.Errorf("unit = %q, want globular-envoy.service", unit)
	}
}

func TestCompileInfrastructurePlan_DataDirs(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "etcd", "3.5.14", "abc123")
	rel.Spec.DataDirs = "/var/lib/globular/etcd,/var/lib/globular/etcd/wal"

	plan, err := CompileInfrastructurePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// infrastructure.install step should have data_dirs.
	installStep := plan.Spec.Steps[3]
	dataDirs := installStep.Args.GetFields()["data_dirs"].GetStringValue()
	if dataDirs != "/var/lib/globular/etcd,/var/lib/globular/etcd/wal" {
		t.Errorf("data_dirs = %q", dataDirs)
	}
}

func TestCompileInfrastructurePlan_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		rel  *cluster_controllerpb.InfrastructureRelease
	}{
		{"nil release", nil},
		{"nil spec", &cluster_controllerpb.InfrastructureRelease{
			Status: &cluster_controllerpb.InfrastructureReleaseStatus{},
		}},
		{"empty publisher", newTestInfraRelease("", "etcd", "3.5.14", "abc")},
		{"empty component", newTestInfraRelease("pub", "", "3.5.14", "abc")},
		{"empty version", newTestInfraRelease("pub", "etcd", "", "abc")},
		{"empty digest", newTestInfraRelease("pub", "etcd", "3.5.14", "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileInfrastructurePlan("node-1", tc.rel, "", "cluster-1")
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestComputeInfrastructureDesiredHash_Deterministic(t *testing.T) {
	h1 := ComputeInfrastructureDesiredHash("pub", "etcd", "3.5.14")
	h2 := ComputeInfrastructureDesiredHash("pub", "etcd", "3.5.14")
	if h1 != h2 {
		t.Errorf("non-deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}
	if strings.ToLower(h1) != h1 {
		t.Error("hash should be lowercase hex")
	}

	h3 := ComputeInfrastructureDesiredHash("pub", "etcd", "3.5.15")
	if h1 == h3 {
		t.Error("different versions should produce different hashes")
	}
}

func TestCompileInfrastructurePlan_NodeVersionOverride(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "etcd", "3.5.14", "abc123")
	rel.Spec.NodeAssignments = []*cluster_controllerpb.NodeAssignment{
		{NodeID: "node-1", Version: "3.5.15"},
	}

	plan, err := CompileInfrastructurePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// install step should use overridden version.
	installStep := plan.Spec.Steps[3]
	ver := installStep.Args.GetFields()["version"].GetStringValue()
	if ver != "3.5.15" {
		t.Errorf("version = %q, want 3.5.15 (node override)", ver)
	}
}

func TestCompileInfrastructurePlan_ReportStateKind(t *testing.T) {
	rel := newTestInfraRelease("core@globular.io", "etcd", "3.5.14", "abc123")
	plan, err := CompileInfrastructurePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// package.report_state step should have kind=INFRASTRUCTURE
	reportStep := plan.Spec.Steps[4]
	kind := reportStep.Args.GetFields()["kind"].GetStringValue()
	if kind != "INFRASTRUCTURE" {
		t.Errorf("kind = %q, want INFRASTRUCTURE", kind)
	}
}

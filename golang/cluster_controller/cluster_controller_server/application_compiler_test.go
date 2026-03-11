package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func newTestAppRelease(pub, name, ver, digest string) *cluster_controllerpb.ApplicationRelease {
	return &cluster_controllerpb.ApplicationRelease{
		Spec: &cluster_controllerpb.ApplicationReleaseSpec{
			PublisherID: pub,
			AppName:     name,
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ApplicationReleaseStatus{
			ResolvedVersion:        ver,
			ResolvedArtifactDigest: digest,
		},
	}
}

func TestCompileApplicationPlan_Basic(t *testing.T) {
	rel := newTestAppRelease("core@globular.io", "webadmin", "1.2.0", "abc123")
	plan, err := CompileApplicationPlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Reason != "application_release" {
		t.Errorf("reason = %q, want application_release", plan.Reason)
	}
	if plan.NodeId != "node-1" {
		t.Errorf("node_id = %q, want node-1", plan.NodeId)
	}

	// Should have: artifact.fetch, artifact.verify, application.install, package.report_state
	if len(plan.Spec.Steps) != 4 {
		t.Fatalf("steps = %d, want 4", len(plan.Spec.Steps))
	}
	actions := []string{"artifact.fetch", "artifact.verify", "application.install", "package.report_state"}
	for i, want := range actions {
		if plan.Spec.Steps[i].Action != want {
			t.Errorf("step[%d].action = %q, want %q", i, plan.Spec.Steps[i].Action, want)
		}
	}

	// No rollback when installedVersion is empty.
	if len(plan.Spec.Rollback) != 0 {
		t.Errorf("rollback steps = %d, want 0", len(plan.Spec.Rollback))
	}

	// Lock format.
	if len(plan.Locks) != 1 || plan.Locks[0] != "application:webadmin" {
		t.Errorf("locks = %v, want [application:webadmin]", plan.Locks)
	}
}

func TestCompileApplicationPlan_WithRollback(t *testing.T) {
	rel := newTestAppRelease("core@globular.io", "webadmin", "2.0.0", "def456")
	plan, err := CompileApplicationPlan("node-1", rel, "1.5.0", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// Rollback: artifact.fetch, artifact.verify, application.install, package.report_state
	if len(plan.Spec.Rollback) != 4 {
		t.Fatalf("rollback steps = %d, want 4", len(plan.Spec.Rollback))
	}

	// Verify rollback installs previous version.
	installStep := plan.Spec.Rollback[2]
	ver := installStep.Args.GetFields()["version"].GetStringValue()
	if ver != "1.5.0" {
		t.Errorf("rollback version = %q, want 1.5.0", ver)
	}
}

func TestCompileApplicationPlan_SameVersionNoRollback(t *testing.T) {
	rel := newTestAppRelease("core@globular.io", "webadmin", "1.0.0", "aaa")
	plan, err := CompileApplicationPlan("node-1", rel, "1.0.0", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Spec.Rollback) != 0 {
		t.Errorf("rollback steps = %d, want 0 (same version)", len(plan.Spec.Rollback))
	}
}

func TestCompileApplicationPlan_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		rel  *cluster_controllerpb.ApplicationRelease
	}{
		{"nil release", nil},
		{"nil spec", &cluster_controllerpb.ApplicationRelease{
			Status: &cluster_controllerpb.ApplicationReleaseStatus{},
		}},
		{"empty publisher", newTestAppRelease("", "app", "1.0.0", "abc")},
		{"empty app_name", newTestAppRelease("pub", "", "1.0.0", "abc")},
		{"empty version", newTestAppRelease("pub", "app", "", "abc")},
		{"empty digest", newTestAppRelease("pub", "app", "1.0.0", "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileApplicationPlan("node-1", tc.rel, "", "cluster-1")
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestCompileApplicationPlan_RouteAndIndex(t *testing.T) {
	rel := newTestAppRelease("core@globular.io", "webadmin", "1.0.0", "abc123")
	rel.Spec.Route = "/admin"
	rel.Spec.IndexFile = "app.html"

	plan, err := CompileApplicationPlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// Check application.install step has route and index_file.
	installStep := plan.Spec.Steps[2]
	fields := installStep.Args.GetFields()
	if fields["route"].GetStringValue() != "/admin" {
		t.Errorf("route = %q, want /admin", fields["route"].GetStringValue())
	}
	if fields["index_file"].GetStringValue() != "app.html" {
		t.Errorf("index_file = %q, want app.html", fields["index_file"].GetStringValue())
	}
}

func TestComputeApplicationDesiredHash_Deterministic(t *testing.T) {
	h1 := ComputeApplicationDesiredHash("pub", "app", "1.0.0")
	h2 := ComputeApplicationDesiredHash("pub", "app", "1.0.0")
	if h1 != h2 {
		t.Errorf("non-deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}
	if strings.ToLower(h1) != h1 {
		t.Error("hash should be lowercase hex")
	}

	// Different version → different hash.
	h3 := ComputeApplicationDesiredHash("pub", "app", "2.0.0")
	if h1 == h3 {
		t.Error("different versions should produce different hashes")
	}
}

func TestCompileApplicationPlan_NodeVersionOverride(t *testing.T) {
	rel := newTestAppRelease("core@globular.io", "webadmin", "1.0.0", "abc123")
	rel.Spec.NodeAssignments = []*cluster_controllerpb.NodeAssignment{
		{NodeID: "node-1", Version: "1.1.0"},
	}

	plan, err := CompileApplicationPlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatal(err)
	}

	// install step should use overridden version.
	installStep := plan.Spec.Steps[2]
	ver := installStep.Args.GetFields()["version"].GetStringValue()
	if ver != "1.1.0" {
		t.Errorf("version = %q, want 1.1.0 (node override)", ver)
	}
}

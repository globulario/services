package main

import "testing"

// ── identitiesMatch ──────────────────────────────────────────────────────────

func TestIdentitiesMatch_ExactMatch(t *testing.T) {
	d := DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3}
	i := InstalledIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3}
	if !identitiesMatch(d, i) {
		t.Error("exact match should return true")
	}
}

func TestIdentitiesMatch_VersionMismatch(t *testing.T) {
	d := DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3}
	i := InstalledIdentity{Name: "dns", Kind: "SERVICE", Version: "1.1.0", BuildNumber: 3}
	if identitiesMatch(d, i) {
		t.Error("different version should not match")
	}
}

func TestIdentitiesMatch_BuildMismatch(t *testing.T) {
	// Same version, wrong build → NOT at desired (Guardrail 3).
	d := DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 6}
	i := InstalledIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 5}
	if identitiesMatch(d, i) {
		t.Error("same version but wrong build should NOT match")
	}
}

func TestIdentitiesMatch_ZeroDesiredBuild(t *testing.T) {
	// Desired build=0 (unspecified) — version match is sufficient.
	d := DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 0}
	i := InstalledIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 5}
	if !identitiesMatch(d, i) {
		t.Error("desired build=0 should match any build with same version")
	}
}

func TestIdentitiesMatch_SemverNormalization(t *testing.T) {
	// "v1.2.0" and "1.2.0" should match via canonical semver.
	d := DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "v1.2.0", BuildNumber: 0}
	i := InstalledIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 0}
	if !identitiesMatch(d, i) {
		t.Error("v-prefixed and non-prefixed versions should match")
	}
}

// ── ComputeServiceProjection ─────────────────────────────────────────────────

func TestComputeServiceProjection_AllConverged(t *testing.T) {
	// desired == installed on all nodes → rollout full.
	desired := &DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3}
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3},
		"node-b": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3},
		"node-c": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3},
	}
	s := ComputeServiceProjection(desired, installed)
	if s.NodesAtDesired != 3 {
		t.Errorf("expected 3 at desired, got %d", s.NodesAtDesired)
	}
	if s.NodesTotal != 3 {
		t.Errorf("expected 3 total, got %d", s.NodesTotal)
	}
	if s.Status != ProjectionConverged {
		t.Errorf("expected converged, got %s", s.Status)
	}
}

func TestComputeServiceProjection_ConvergedDespiteUnhealthyRuntime(t *testing.T) {
	// desired matches installed but runtime unhealthy → rollout still full.
	// (Runtime health is not an input to projection.)
	desired := &DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3}
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3},
		"node-b": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3},
	}
	// No runtime health input exists in the function signature.
	// This test proves that projection is structurally immune to health influence.
	s := ComputeServiceProjection(desired, installed)
	if s.NodesAtDesired != 2 || s.Status != ProjectionConverged {
		t.Errorf("converged despite unhealthy runtime: at=%d status=%s", s.NodesAtDesired, s.Status)
	}
}

func TestComputeServiceProjection_ConvergedDespiteRunningWorkflow(t *testing.T) {
	// desired matches installed but workflow still running → rollout still full.
	// (Workflow state is not an input to projection.)
	desired := &DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3}
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 3},
	}
	s := ComputeServiceProjection(desired, installed)
	if s.NodesAtDesired != 1 || s.Status != ProjectionConverged {
		t.Errorf("converged despite running workflow: at=%d status=%s", s.NodesAtDesired, s.Status)
	}
}

func TestComputeServiceProjection_SameVersionWrongBuild(t *testing.T) {
	// Same version but wrong build → not at desired.
	desired := &DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 6}
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 5},
		"node-b": {Name: "dns", Kind: "SERVICE", Version: "1.2.0", BuildNumber: 6},
	}
	s := ComputeServiceProjection(desired, installed)
	if s.NodesAtDesired != 1 {
		t.Errorf("expected 1 at desired (only node-b matches build), got %d", s.NodesAtDesired)
	}
	if s.Status != ProjectionProgressing {
		t.Errorf("expected progressing, got %s", s.Status)
	}
}

func TestComputeServiceProjection_Unmanaged(t *testing.T) {
	// No desired but installed exists → unmanaged.
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "1.1.0", BuildNumber: 1},
	}
	s := ComputeServiceProjection(nil, installed)
	if s.Status != ProjectionUnmanaged {
		t.Errorf("expected unmanaged, got %s", s.Status)
	}
}

func TestComputeServiceProjection_DesiredNotInstalled(t *testing.T) {
	// Desired exists, zero nodes have it → desired_not_installed.
	desired := &DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "2.0.0", BuildNumber: 1}
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "1.0.0", BuildNumber: 1},
		"node-b": {},
	}
	s := ComputeServiceProjection(desired, installed)
	if s.NodesAtDesired != 0 {
		t.Errorf("expected 0 at desired, got %d", s.NodesAtDesired)
	}
	if s.Status != ProjectionDesiredNotInstalled {
		t.Errorf("expected desired_not_installed, got %s", s.Status)
	}
}

func TestComputeServiceProjection_PartialRollout(t *testing.T) {
	// Partial node matches → partial rollout.
	desired := &DesiredIdentity{Name: "dns", Kind: "SERVICE", Version: "2.0.0", BuildNumber: 1}
	installed := map[string]InstalledIdentity{
		"node-a": {Name: "dns", Kind: "SERVICE", Version: "2.0.0", BuildNumber: 1},
		"node-b": {Name: "dns", Kind: "SERVICE", Version: "1.0.0", BuildNumber: 1},
		"node-c": {Name: "dns", Kind: "SERVICE", Version: "2.0.0", BuildNumber: 1},
	}
	s := ComputeServiceProjection(desired, installed)
	if s.NodesAtDesired != 2 {
		t.Errorf("expected 2 at desired, got %d", s.NodesAtDesired)
	}
	if s.NodesTotal != 3 {
		t.Errorf("expected 3 total, got %d", s.NodesTotal)
	}
	if s.Status != ProjectionProgressing {
		t.Errorf("expected progressing, got %s", s.Status)
	}
}

package main

import (
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// makeManifest is a test helper that constructs a minimal ArtifactManifest.
func makeManifest(publisher, name, version string, kind repopb.ArtifactKind, hardDeps []string, runtimeUses []string) *repopb.ArtifactManifest {
	var deps []*repopb.ArtifactDependencyRef
	for _, d := range hardDeps {
		deps = append(deps, &repopb.ArtifactDependencyRef{Name: d})
	}
	return &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: publisher,
			Name:        name,
			Version:     version,
			Kind:        kind,
		},
		HardDeps:    deps,
		RuntimeUses: runtimeUses,
	}
}

// ── INV_C_NO_DEP_ON_APPLICATION ───────────────────────────────────────────

func TestArtifactLaw_NoDepOnApplication_Pass(t *testing.T) {
	// A SERVICE depending on INFRASTRUCTURE is fine.
	catalog := []*repopb.ArtifactManifest{
		makeManifest("core", "etcd", "1.0.0", repopb.ArtifactKind_INFRASTRUCTURE, nil, nil),
	}
	incoming := makeManifest("core", "authentication", "1.0.0", repopb.ArtifactKind_SERVICE,
		[]string{"etcd"}, nil)

	violations := NewArtifactLawValidator(incoming, catalog).Validate()
	if len(violations) != 0 {
		t.Errorf("expected no violations, got: %v", violations)
	}
}

func TestArtifactLaw_NoDepOnApplication_Fail(t *testing.T) {
	// SERVICE depending on APPLICATION — must be caught.
	catalog := []*repopb.ArtifactManifest{
		makeManifest("core", "my-app", "1.0.0", repopb.ArtifactKind_APPLICATION, nil, nil),
	}
	incoming := makeManifest("core", "authentication", "1.0.0", repopb.ArtifactKind_SERVICE,
		[]string{"my-app"}, nil)

	violations := NewArtifactLawValidator(incoming, catalog).Validate()
	if len(violations) == 0 {
		t.Fatal("expected INV_C_NO_DEP_ON_APPLICATION violation, got none")
	}
	found := false
	for _, v := range violations {
		if v.Rule == "INV_C_NO_DEP_ON_APPLICATION" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INV_C_NO_DEP_ON_APPLICATION, got: %v", violations)
	}
}

func TestArtifactLaw_ApplicationMayHardDepOnService(t *testing.T) {
	// APPLICATION depending on SERVICE/INFRASTRUCTURE is fine — it is a leaf consumer.
	catalog := []*repopb.ArtifactManifest{
		makeManifest("core", "authentication", "1.0.0", repopb.ArtifactKind_SERVICE, nil, nil),
		makeManifest("core", "etcd", "1.0.0", repopb.ArtifactKind_INFRASTRUCTURE, nil, nil),
	}
	incoming := makeManifest("core", "admin-ui", "1.0.0", repopb.ArtifactKind_APPLICATION,
		[]string{"authentication", "etcd"}, nil)

	violations := NewArtifactLawValidator(incoming, catalog).Validate()
	// Should have no INV_C violations (APPLICATION can depend on non-APPLICATION).
	for _, v := range violations {
		if v.Rule == "INV_C_NO_DEP_ON_APPLICATION" {
			t.Errorf("APPLICATION should be allowed to hard_dep on SERVICE/INFRA, got violation: %v", v)
		}
	}
}

// ── LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS ───────────────────────────────────

func TestArtifactLaw_CommandNoRuntimeDeps_Pass(t *testing.T) {
	incoming := makeManifest("core", "globularcli", "1.0.0", repopb.ArtifactKind_COMMAND,
		nil, nil)

	violations := NewArtifactLawValidator(incoming, nil).Validate()
	if len(violations) != 0 {
		t.Errorf("expected no violations for COMMAND with empty runtime_uses, got: %v", violations)
	}
}

func TestArtifactLaw_CommandNoRuntimeDeps_Fail(t *testing.T) {
	incoming := makeManifest("core", "globularcli", "1.0.0", repopb.ArtifactKind_COMMAND,
		nil, []string{"repository.PackageRepository"})

	violations := NewArtifactLawValidator(incoming, nil).Validate()
	if len(violations) == 0 {
		t.Fatal("expected LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS violation, got none")
	}
	found := false
	for _, v := range violations {
		if v.Rule == "LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS, got: %v", violations)
	}
}

func TestArtifactLaw_ServiceMayHaveRuntimeUses(t *testing.T) {
	// SERVICE declaring runtime_uses is fine.
	incoming := makeManifest("core", "authentication", "1.0.0", repopb.ArtifactKind_SERVICE,
		nil, []string{"rbac.RbacService", "resource.ResourceService"})

	violations := NewArtifactLawValidator(incoming, nil).Validate()
	if len(violations) != 0 {
		t.Errorf("SERVICE should be allowed to declare runtime_uses, got: %v", violations)
	}
}

// ── INV_D_HARD_DEPS_ACYCLIC ───────────────────────────────────────────────

func TestArtifactLaw_HardDepsAcyclic_Pass(t *testing.T) {
	// Linear chain: authentication → rbac → etcd (no cycles).
	catalog := []*repopb.ArtifactManifest{
		makeManifest("core", "etcd", "1.0.0", repopb.ArtifactKind_INFRASTRUCTURE, nil, nil),
		makeManifest("core", "rbac", "1.0.0", repopb.ArtifactKind_SERVICE, []string{"etcd"}, nil),
	}
	incoming := makeManifest("core", "authentication", "1.0.0", repopb.ArtifactKind_SERVICE,
		[]string{"rbac"}, nil)

	violations := NewArtifactLawValidator(incoming, catalog).Validate()
	for _, v := range violations {
		if v.Rule == "INV_D_HARD_DEPS_ACYCLIC" {
			t.Errorf("no cycle expected in linear chain, got violation: %v", v)
		}
	}
}

func TestArtifactLaw_HardDepsAcyclic_Fail_DirectCycle(t *testing.T) {
	// A → B, B → A: direct cycle.
	catalog := []*repopb.ArtifactManifest{
		makeManifest("core", "b", "1.0.0", repopb.ArtifactKind_SERVICE, []string{"a"}, nil),
	}
	incoming := makeManifest("core", "a", "1.0.0", repopb.ArtifactKind_SERVICE,
		[]string{"b"}, nil)

	violations := NewArtifactLawValidator(incoming, catalog).Validate()
	found := false
	for _, v := range violations {
		if v.Rule == "INV_D_HARD_DEPS_ACYCLIC" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INV_D_HARD_DEPS_ACYCLIC for A→B→A cycle, got: %v", violations)
	}
}

func TestArtifactLaw_HardDepsAcyclic_Fail_TransitiveCycle(t *testing.T) {
	// A → B → C → A: transitive cycle.
	catalog := []*repopb.ArtifactManifest{
		makeManifest("core", "b", "1.0.0", repopb.ArtifactKind_SERVICE, []string{"c"}, nil),
		makeManifest("core", "c", "1.0.0", repopb.ArtifactKind_SERVICE, []string{"a"}, nil),
	}
	incoming := makeManifest("core", "a", "1.0.0", repopb.ArtifactKind_SERVICE,
		[]string{"b"}, nil)

	violations := NewArtifactLawValidator(incoming, catalog).Validate()
	found := false
	for _, v := range violations {
		if v.Rule == "INV_D_HARD_DEPS_ACYCLIC" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INV_D_HARD_DEPS_ACYCLIC for transitive cycle A→B→C→A, got: %v", violations)
	}
}

func TestArtifactLaw_HardDepsAcyclic_NoCatalog(t *testing.T) {
	// No catalog — single artifact with no deps is always valid.
	incoming := makeManifest("core", "authentication", "1.0.0", repopb.ArtifactKind_SERVICE, nil, nil)
	violations := NewArtifactLawValidator(incoming, nil).Validate()
	if len(violations) != 0 {
		t.Errorf("standalone artifact with no catalog should have no violations, got: %v", violations)
	}
}

// ── ArtifactViolation.Error ───────────────────────────────────────────────

func TestArtifactViolation_ErrorFormat(t *testing.T) {
	v := ArtifactViolation{Rule: "TEST_RULE", Artifact: "core/foo@1.0.0", Detail: "something wrong"}
	s := v.Error()
	if !strings.Contains(s, "TEST_RULE") || !strings.Contains(s, "core/foo@1.0.0") {
		t.Errorf("unexpected Error() format: %s", s)
	}
}

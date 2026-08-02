#!/usr/bin/env python3
"""Apply the bounded review fixes required before PR #222 can merge."""

from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    (ROOT / path).write_text(content, encoding="utf-8")


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{label}: expected exactly one match, found {count}")
    return text.replace(old, new, 1)


def patch_workflow_writers() -> None:
    path = "golang/cluster_controller/cluster_controller_server/workflow_release.go"
    text = read(path)

    anchor = '''func releaseResourceType(pkgKind string) string {
	if strings.ToUpper(pkgKind) == "INFRASTRUCTURE" {
		return "InfrastructureRelease"
	}
	return "ServiceRelease"
}
'''
    helper = anchor + '''
// applyWorkflowRelease is the workflow-status persistence choke point. Service
// releases must pass through applyServiceRelease so stale pre-upgrade objects
// carrying unsupported NodeAssignments cannot be silently re-persisted by a
// status callback. Infrastructure releases retain the generic owner path.
func (srv *server) applyWorkflowRelease(ctx context.Context, resourceType string, obj interface{}) error {
	if rel, ok := obj.(*cluster_controllerpb.ServiceRelease); ok {
		_, err := srv.applyServiceRelease(ctx, rel)
		return err
	}
	_, err := srv.resources.Apply(ctx, resourceType, obj)
	return err
}
'''
    text = replace_once(text, anchor, helper, "workflow release helper")

    direct_rel = '_, err = srv.resources.Apply(ctx, resourceType, rel)'
    rel_count = text.count(direct_rel)
    if rel_count != 4:
        raise RuntimeError(f"workflow direct rel writes: expected 4, found {rel_count}")
    text = text.replace(direct_rel, 'err = srv.applyWorkflowRelease(ctx, resourceType, rel)')

    direct_obj = '_, err = srv.resources.Apply(ctx, resourceType, obj)'
    obj_count = text.count(direct_obj)
    if obj_count != 1:
        raise RuntimeError(f"workflow direct obj writes: expected 1, found {obj_count}")
    text = text.replace(direct_obj, 'err = srv.applyWorkflowRelease(ctx, resourceType, obj)', 1)

    if 'srv.resources.Apply(ctx, resourceType, rel)' in text or 'srv.resources.Apply(ctx, resourceType, obj)' in text:
        raise RuntimeError("workflow release writer still bypasses applyWorkflowRelease")
    write(path, text)


def patch_canonical_placement() -> None:
    path = "golang/cluster_controller/cluster_controller_server/placement_grants.go"
    text = read(path)
    old = '''func authorizedForNode(pkg string, nodeProfiles []string, grants map[string]bool) bool {
	var catProfiles []string
	if cat := CatalogByName(pkg); cat != nil {
		catProfiles = cat.Profiles
	}
	if placementAllows(catProfiles, nodeProfiles) {
		return true
	}
	return grants[canonicalServiceName(pkg)]
}
'''
    new = '''func authorizedForNode(pkg string, nodeProfiles []string, grants map[string]bool) bool {
	canonical := canonicalServiceName(pkg)
	if canonical == "" {
		return true // preserve unknown-to-catalog behavior
	}
	cat := CatalogByName(canonical)
	if cat == nil {
		return true // unknown-to-catalog packages are outside profile placement
	}
	if placementAllows(cat.Profiles, nodeProfiles) {
		return true
	}
	return grants[canonical]
}
'''
    text = replace_once(text, old, new, "canonical authorizedForNode")
    old = '''func isOrphanedInstallForNode(name string, nodeProfiles []string, grants map[string]bool) bool {
	return isOrphanedInstall(name, nodeProfiles) && !grants[canonicalServiceName(name)]
}
'''
    new = '''func isOrphanedInstallForNode(name string, nodeProfiles []string, grants map[string]bool) bool {
	canonical := canonicalServiceName(name)
	return isOrphanedInstall(canonical, nodeProfiles) && !grants[canonical]
}
'''
    text = replace_once(text, old, new, "canonical orphan predicate")
    write(path, text)

    test_path = "golang/cluster_controller/cluster_controller_server/placement_grants_test.go"
    tests = read(test_path)
    anchor = '''	// explicit-only (grant) → authorized, NOT orphan.
'''
    addition = '''	// Supported noncanonical identities must resolve through the same catalog
	// authority before profile evaluation. Otherwise CatalogByName(raw) misses
	// and the unknown-package fallback incorrectly authorizes the workload.
	for _, alias := range []string{"torrent_server", "globular-torrent.service"} {
		if authorizedForNode(alias, node, noGrants) {
			t.Errorf("noncanonical %q must remain profile-unauthorized after canonicalization", alias)
		}
		if !isOrphanedInstallForNode(alias, node, noGrants) {
			t.Errorf("noncanonical %q must resolve to the canonical orphan", alias)
		}
	}

'''
    tests = replace_once(tests, anchor, addition + anchor, "noncanonical placement test")
    write(test_path, tests)


def patch_auto_registration_generation() -> None:
    path = "golang/cluster_controller/cluster_controller_server/handlers_status.go"
    text = read(path)
    old = '''			Status:         "recovering",
			Profiles:       []string{}, // do not assume privileged profiles
			Metadata:       make(map[string]string),
'''
    new = '''			Status:              "recovering",
			Profiles:            []string{}, // do not assume privileged profiles
			PlacementGeneration: 1,          // restored node has an established empty placement set
			Metadata:            make(map[string]string),
'''
    text = replace_once(text, old, new, "status auto-registration generation")
    write(path, text)

    test_path = "golang/cluster_controller/cluster_controller_server/placement_generation_test.go"
    tests = read(test_path)
    append = '''
// ReportNodeStatus is a second production node constructor used after
// controller-state loss. It must establish placement generation together with
// its intentionally empty profile set, just like the signed join constructor.
func TestStatusAutoRegistrationEstablishesPlacementGeneration(t *testing.T) {
	data, err := os.ReadFile("handlers_status.go")
	if err != nil {
		t.Fatalf("read handlers_status.go: %v", err)
	}
	text := string(data)
	anchor := "ReportNodeStatus: auto-registering unknown node"
	start := strings.Index(text, anchor)
	if start < 0 {
		t.Fatalf("ReportNodeStatus auto-registration constructor not found")
	}
	end := strings.Index(text[start:], "srv.state.Nodes[nodeID] = node")
	if end < 0 {
		t.Fatalf("ReportNodeStatus auto-registration constructor boundary not found")
	}
	constructor := text[start : start+end]
	if !strings.Contains(constructor, "PlacementGeneration: 1") {
		t.Fatalf("status auto-registration must establish PlacementGeneration=1")
	}
}
'''
    if "TestStatusAutoRegistrationEstablishesPlacementGeneration" in tests:
        raise RuntimeError("auto-registration generation test already exists")
    write(test_path, tests + append)


def patch_writer_ratchet_test() -> None:
    path = "golang/cluster_controller/cluster_controller_server/handlers_status_grant_reject_test.go"
    text = read(path)
    old_import = '''import (
	"testing"
'''
    new_import = '''import (
	"os"
	"strings"
	"testing"
'''
    text = replace_once(text, old_import, new_import, "writer ratchet imports")
    append = '''
// Workflow callbacks are writers too. They must not bypass the canonical
// ServiceRelease validation choke point when persisting status updates.
func TestWorkflowReleaseWritersUseCanonicalChokePoint(t *testing.T) {
	data, err := os.ReadFile("workflow_release.go")
	if err != nil {
		t.Fatalf("read workflow_release.go: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"srv.resources.Apply(ctx, resourceType, rel)",
		"srv.resources.Apply(ctx, resourceType, obj)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("workflow callback bypasses canonical ServiceRelease validation: %s", forbidden)
		}
	}
	if !strings.Contains(text, "srv.applyServiceRelease(ctx, rel)") {
		t.Fatalf("workflow release persistence must route ServiceRelease through applyServiceRelease")
	}
}
'''
    if "TestWorkflowReleaseWritersUseCanonicalChokePoint" in text:
        raise RuntimeError("workflow writer ratchet test already exists")
    write(path, text + append)


if __name__ == "__main__":
    patch_workflow_writers()
    patch_canonical_placement()
    patch_auto_registration_generation()
    patch_writer_ratchet_test()
    print("PR #222 review fixes applied")

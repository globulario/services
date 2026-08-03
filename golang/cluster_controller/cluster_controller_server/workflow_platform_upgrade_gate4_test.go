package main

// Gate-4 regression for the stuck-drift storm.
//
// A PUBLISHED manifest is cluster-wide (ScyllaDB); the blob is node-local POSIX
// CAS. Admitting a package because its manifest resolves dispatches
// release.apply.package into a download that can never succeed, and the drift
// re-dispatches every reconcile cycle forever — globular-cli@1.2.289 reached
// 914 consecutive cycles across three nodes.
//
// The resolver is gate 4's input: returning "" for an unretrievable artifact is
// what makes evaluateUpgradeDecisions classify it missing_in_repo and refuse.

import "testing"

func gate4Nodes() []NodeView {
	return []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"core"},
		InstalledVersions: map[string]string{"globular-cli": "1.2.288"},
	}}
}

func gate4BOM() []BOMPackage {
	return []BOMPackage{{Name: "globular-cli", Kind: "COMMAND", Version: "1.2.289"}}
}

// Blob present → the package is planned. Without this the fix would simply
// block everything and "no storm" would be meaningless.
func TestGate4_AdmitsWhenBlobRetrievable(t *testing.T) {
	resolve := LocalBuildIDResolver(func(name, version string) string { return "build-1" })
	_, upgrades := evaluateUpgradeDecisions(gate4Nodes(), gate4BOM(), resolve, placementFromBOM(gate4BOM()))
	if len(upgrades) != 1 {
		t.Fatalf("retrievable artifact must be planned: got %d upgrades, want 1", len(upgrades))
	}
}

// Manifest resolves but the blob is missing on the serving node: the resolver
// returns "" and the package must NOT be dispatched.
func TestGate4_RefusesWhenBlobMissing(t *testing.T) {
	resolve := LocalBuildIDResolver(func(name, version string) string { return "" })
	audit, upgrades := evaluateUpgradeDecisions(gate4Nodes(), gate4BOM(), resolve, placementFromBOM(gate4BOM()))

	if len(upgrades) != 0 {
		t.Fatalf("published manifest with no retrievable blob was dispatched (%d upgrades) — this is the 914-cycle storm", len(upgrades))
	}
	// The package must stay visibly blocked, not silently vanish from the audit.
	var found bool
	for _, d := range audit {
		if d.PackageName == "globular-cli" {
			found = true
			if d.Action != "missing_in_repo" {
				t.Errorf("action = %q, want missing_in_repo", d.Action)
			}
			if d.Reason == "" {
				t.Error("a refused package must carry a reason so the operator can see why it is blocked")
			}
		}
	}
	if !found {
		t.Error("refused package absent from audit — blocked work must remain visible")
	}
}

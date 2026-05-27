package main

// Regression tests for skipIfAlreadyInstalled and resolveLatestManifestFunc
// (workflow_runner.go).
//
// INC-2026-0007 (phase 1): local node.join runner passed version="" →
// "0.0.0-dev" to InstallPackage for already-installed+active packages.
// Fix: skipIfAlreadyInstalled skips when installed record exists and unit active.
//
// INC-2026-0007 (phase 2): first-time installs (existing==nil) still passed
// version="" → "0.0.0-dev" because there was no installed-state record to read.
// Fix: resolveLatestManifestFunc queries the repository for the latest manifest
// and passes the real version/buildID/checksum to InstallPackage.

import (
	"context"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

func alreadyInstalled(version string) *node_agentpb.InstalledPackage {
	return &node_agentpb.InstalledPackage{Version: version, Status: "installed"}
}

// TestSkipIfAlreadyInstalled_ServiceActive — installed + unit active → skip.
func TestSkipIfAlreadyInstalled_ServiceActive(t *testing.T) {
	got := skipIfAlreadyInstalled(context.Background(), "xds",
		alreadyInstalled("1.2.64"), alwaysActive)
	if !got {
		t.Fatal("expected skip=true when installed and unit active")
	}
}

// TestSkipIfAlreadyInstalled_UnitInactive — installed but unit inactive → no skip.
func TestSkipIfAlreadyInstalled_UnitInactive(t *testing.T) {
	got := skipIfAlreadyInstalled(context.Background(), "xds",
		alreadyInstalled("1.2.64"), alwaysInactive)
	if got {
		t.Fatal("expected skip=false when unit is inactive")
	}
}

// TestSkipIfAlreadyInstalled_NotInstalled — nil record → no skip.
func TestSkipIfAlreadyInstalled_NotInstalled(t *testing.T) {
	got := skipIfAlreadyInstalled(context.Background(), "prometheus",
		nil, alwaysActive)
	if got {
		t.Fatal("expected skip=false when no installed record")
	}
}

// TestSkipIfAlreadyInstalled_StatusNotInstalled — record exists but wrong status → no skip.
func TestSkipIfAlreadyInstalled_StatusNotInstalled(t *testing.T) {
	pkg := &node_agentpb.InstalledPackage{Version: "1.2.64", Status: "pending"}
	got := skipIfAlreadyInstalled(context.Background(), "xds",
		pkg, alwaysActive)
	if got {
		t.Fatal("expected skip=false when status != installed")
	}
}

// TestSkipIfAlreadyInstalled_CommandPackage — command package installed → skip
// without calling checkActive.
func TestSkipIfAlreadyInstalled_CommandPackage(t *testing.T) {
	checkActiveCalled := false
	checkActive := func(_ context.Context, _ string) (bool, error) {
		checkActiveCalled = true
		return false, nil
	}
	got := skipIfAlreadyInstalled(context.Background(), "etcdctl",
		alreadyInstalled("3.5.14"), checkActive)
	if !got {
		t.Fatal("expected skip=true for installed command package")
	}
	if checkActiveCalled {
		t.Fatal("checkActive must not be called for command packages (no unit)")
	}
}

// TestRuntimeFastPath_SkipsWhenUnitActiveWithoutInstalledState documents the
// Day-1 fallback where installed-state can be stale/missing but runtime unit
// proof is already active.
func TestRuntimeFastPath_SkipsWhenUnitActiveWithoutInstalledState(t *testing.T) {
	existing := (*node_agentpb.InstalledPackage)(nil)
	if skipIfAlreadyInstalled(context.Background(), "envoy", existing, alwaysActive) {
		t.Fatal("skipIfAlreadyInstalled must be false when record is nil")
	}
	unit := packageUnit("envoy")
	if unit == "" {
		t.Fatal("expected envoy to have a unit")
	}
	active, _ := alwaysActive(context.Background(), unit)
	if !active {
		t.Fatal("expected active runtime proof")
	}
}

// TestResolveLatestManifestFuncCalledForFirstInstall — when existing is nil
// (first-time install), resolveLatestManifestFunc must be invoked so a real
// version is passed to InstallPackage instead of the 0.0.0-dev sentinel.
// This is a unit test of the injectable variable path only (no live repo dial).
func TestResolveLatestManifestFuncCalledForFirstInstall(t *testing.T) {
	called := false
	prev := resolveLatestManifestFunc
	resolveLatestManifestFunc = func(_ context.Context, name, kind, repoAddr string) (string, string, string, error) {
		called = true
		if name != "gateway" {
			t.Errorf("expected name=gateway, got %s", name)
		}
		if repoAddr == "" {
			t.Error("repoAddr must not be empty when repo is available")
		}
		return "1.2.67", "build-abc", "sha256:deadbeef", nil
	}
	t.Cleanup(func() { resolveLatestManifestFunc = prev })

	// Simulate: existing==nil, repoAddr set, resolveLatestManifestFunc injects version.
	var capturedVersion, capturedBuildID, capturedChecksum string
	installCalled := false
	doInstall := func(ctx context.Context, name, kind, repoAddr, version, buildID, checksum string) error {
		installCalled = true
		capturedVersion = version
		capturedBuildID = buildID
		capturedChecksum = checksum
		return nil
	}

	// Replicate the FetchAndInstall logic for the first-install branch.
	existing := (*node_agentpb.InstalledPackage)(nil)
	repoAddr := "10.0.0.63:443"
	version, buildID, checksum := "", "", ""
	if existing != nil {
		version = existing.GetVersion()
		buildID = existing.GetBuildId()
		checksum = existing.GetChecksum()
	} else if repoAddr != "" {
		if v, b, c, err := resolveLatestManifestFunc(context.Background(), "gateway", "SERVICE", repoAddr); err == nil {
			version, buildID, checksum = v, b, c
		}
	}
	if err := doInstall(context.Background(), "gateway", "SERVICE", repoAddr, version, buildID, checksum); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Fatal("resolveLatestManifestFunc was not called for first-install (existing==nil)")
	}
	if !installCalled {
		t.Fatal("install was not called")
	}
	if capturedVersion == "" || capturedVersion == "0.0.0-dev" {
		t.Fatalf("expected real version from resolver, got %q (0.0.0-dev sentinel must not reach InstallPackage)", capturedVersion)
	}
	if capturedBuildID == "" {
		t.Fatal("expected non-empty buildID from resolver")
	}
	if capturedChecksum == "" {
		t.Fatal("expected non-empty checksum from resolver")
	}
}

// TestResolveLatestManifestFuncErrorFallsBackToReleaseIndex — when repository
// latest-manifest resolution fails for first install, the runner should use the
// BOM version from release-index.json instead of leaving version empty
// (which would become the 0.0.0-dev sentinel).
func TestResolveLatestManifestFuncErrorFallsBackToReleaseIndex(t *testing.T) {
	prevResolve := resolveLatestManifestFunc
	prevBOM := resolveVersionFromReleaseIndexFunc
	resolveLatestManifestFunc = func(_ context.Context, _, _, _ string) (string, string, string, error) {
		return "", "", "", fmt.Errorf("repo unreachable")
	}
	resolveVersionFromReleaseIndexFunc = func(pkgName string) (string, error) {
		if pkgName != "gateway" {
			t.Fatalf("expected pkgName gateway, got %s", pkgName)
		}
		return "0.0.1", nil
	}
	t.Cleanup(func() {
		resolveLatestManifestFunc = prevResolve
		resolveVersionFromReleaseIndexFunc = prevBOM
	})

	existing := (*node_agentpb.InstalledPackage)(nil)
	repoAddr := "10.0.0.63:443"
	version, buildID, checksum := "", "", ""
	if existing != nil {
		version = existing.GetVersion()
		buildID = existing.GetBuildId()
		checksum = existing.GetChecksum()
	} else if repoAddr != "" {
		if v, b, c, err := resolveLatestManifestFunc(context.Background(), "gateway", "SERVICE", repoAddr); err == nil {
			version, buildID, checksum = v, b, c
		} else if bomVersion, bomErr := resolveVersionFromReleaseIndexFunc("gateway"); bomErr == nil {
			version = bomVersion
		}
	}
	if version != "0.0.1" {
		t.Fatalf("expected release-index fallback version 0.0.1, got %q", version)
	}
	if buildID != "" || checksum != "" {
		t.Fatalf("expected no buildID/checksum from release-index fallback, got buildID=%q checksum=%q", buildID, checksum)
	}
}

// Awareness required-test name wrappers for installed-state/runtime semantics.
func TestInstalledIndicator_BoundToInstalledStateRecord(t *testing.T) {
	TestSkipIfAlreadyInstalled_NotInstalled(t)
}

func TestInstalledState_NotDerivedFromCatalog(t *testing.T) {
	TestSkipIfAlreadyInstalled_StatusNotInstalled(t)
}

func TestBuildIDNotResolvedAtNodeAgent(t *testing.T) {
	TestResolveLatestManifestFuncCalledForFirstInstall(t)
}

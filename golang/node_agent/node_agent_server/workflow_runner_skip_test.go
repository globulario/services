package main

// Regression tests for skipIfAlreadyInstalled (workflow_runner.go).
//
// INC-2026-0007: local node.join runner passed version="" → "0.0.0-dev" to
// InstallPackage. artifact.fetch refused to reuse the staged cache file without
// a manifest for that non-existent version, causing install_mesh to fail on
// every rejoin attempt even when all 5 mesh packages were already installed and
// running. The fix: check installed state + unit active before calling
// InstallPackage, and skip if the package is healthy.

import (
	"context"
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

package main

// Regression coverage for the COMMAND-kind blind spot (2026-07-10): the drift
// scanner and the SCAR-2 convergence observer looked up installed_state for
// SERVICE then INFRASTRUCTURE only, so COMMAND packages (mc, etcdctl, restic,
// rclone, sctool, sha256sum, …) were reported as missing_package forever even
// though the node-agent had recorded them installed — a permanent
// workflow.drift_stuck loop with no possible convergence.
// Contract:  installed_state kind MUST be one of SERVICE|INFRASTRUCTURE|COMMAND
//            (installed_state/schema.go); the scanner must see every legal kind.

import (
	"context"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestInstalledPackageAnyKind_SeesCommandKind(t *testing.T) {
	ctx := context.Background()
	orig := getInstalledPackageFn
	defer func() { getInstalledPackageFn = orig }()

	// Simulate the incident state: the record exists ONLY under kind COMMAND.
	var kindsQueried []string
	getInstalledPackageFn = func(_ context.Context, nodeID, kind, name string) (*node_agentpb.InstalledPackage, error) {
		kindsQueried = append(kindsQueried, kind)
		if kind == "COMMAND" && name == "etcdctl" {
			return &node_agentpb.InstalledPackage{Name: name, Kind: kind, Version: "3.5.14"}, nil
		}
		return nil, nil
	}

	pkg, err := installedPackageAnyKind(ctx, "n1", "etcdctl")
	if err != nil {
		t.Fatal(err)
	}
	if pkg == nil {
		t.Fatalf("COMMAND-kind record must be visible; kinds queried=%v", kindsQueried)
	}
	if pkg.GetVersion() != "3.5.14" {
		t.Fatalf("unexpected version %q", pkg.GetVersion())
	}
	// etcdctl is a catalog COMMAND component: COMMAND must be tried first so
	// the common case costs one etcd read, not three.
	if len(kindsQueried) == 0 || kindsQueried[0] != "COMMAND" {
		t.Fatalf("catalog COMMAND component must probe COMMAND first, got %v", kindsQueried)
	}
}

func TestInstalledPackageAnyKind_UncataloguedFallsThroughAllKinds(t *testing.T) {
	ctx := context.Background()
	orig := getInstalledPackageFn
	defer func() { getInstalledPackageFn = orig }()

	var kindsQueried []string
	getInstalledPackageFn = func(_ context.Context, _, kind, _ string) (*node_agentpb.InstalledPackage, error) {
		kindsQueried = append(kindsQueried, kind)
		return nil, nil
	}

	pkg, err := installedPackageAnyKind(ctx, "n1", "not-in-catalog")
	if err != nil || pkg != nil {
		t.Fatalf("expected miss without error, pkg=%v err=%v", pkg, err)
	}
	want := map[string]bool{"SERVICE": true, "INFRASTRUCTURE": true, "COMMAND": true}
	for _, k := range kindsQueried {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Fatalf("all legal kinds must be probed on a miss; not probed: %v (queried %v)", want, kindsQueried)
	}
}

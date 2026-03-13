package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestCompileReleasePlan_IncludesSuccessProbes(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "authentication",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	probes := plan.GetSpec().GetSuccessProbes()
	if len(probes) == 0 {
		t.Fatal("expected at least one success probe")
	}

	// Authentication is a known gRPC service, so we should have both a TCP and gRPC health probe.
	var hasTCP, hasGRPCHealth bool
	for _, p := range probes {
		if p.GetType() == "probe.service_config_tcp" || p.GetType() == "probe.tcp" {
			hasTCP = true
		}
		if p.GetType() == "probe.grpc_health" {
			hasGRPCHealth = true
		}
	}

	if !hasTCP {
		t.Error("expected a TCP probe for authentication service")
	}
	if !hasGRPCHealth {
		t.Error("expected a gRPC health probe for authentication service")
	}
}

func TestCompileReleasePlan_GRPCProbePort(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "rbac",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, p := range plan.GetSpec().GetSuccessProbes() {
		if p.GetType() == "probe.grpc_health" {
			addr := p.GetArgs().GetFields()["address"].GetStringValue()
			if !strings.Contains(addr, "10104") {
				t.Errorf("expected RBAC gRPC probe on port 10104, got address %q", addr)
			}
			return
		}
	}
	t.Error("expected a gRPC health probe for rbac service")
}

func TestCompileReleasePlan_NonGRPCService_NoHealthProbe(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "envoy",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
	}

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	for _, p := range plan.GetSpec().GetSuccessProbes() {
		if p.GetType() == "probe.grpc_health" {
			t.Error("envoy should not have a gRPC health probe (it's not a Globular gRPC service)")
		}
	}
}

func TestBuildSuccessProbes_KnownServices(t *testing.T) {
	knownServices := []string{"authentication", "event", "file", "rbac", "resource", "repository", "dns"}

	for _, svc := range knownServices {
		unit := serviceUnitForCanonical(svc)
		probes := buildSuccessProbes(unit, svc)
		if len(probes) < 2 {
			t.Errorf("service %q: expected at least 2 probes (TCP + gRPC health), got %d", svc, len(probes))
		}
	}
}

func TestBuildSuccessProbes_UnknownService(t *testing.T) {
	probes := buildSuccessProbes("globular-custom.service", "custom")
	if len(probes) != 1 {
		t.Errorf("unknown service: expected 1 probe (TCP only), got %d", len(probes))
	}
}

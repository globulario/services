package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

func TestResolveIntent_CoreProfile(t *testing.T) {
	intent, err := ResolveNodeIntent("test-node", []string{"core"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Core infra must include infrastructure-kind components.
	infraNames := intent.DesiredInfraNames
	for _, want := range []string{"etcd", "minio"} {
		if !contains(infraNames, want) {
			t.Errorf("core infra missing %q, got %v", want, infraNames)
		}
	}

	// dns, monitoring, event, rbac, file are KindWorkload (ManagedUnit=true)
	// — they appear in ResolvedComponents but NOT in DesiredInfraNames.
	for _, wl := range []string{"event", "rbac", "file", "dns", "monitoring"} {
		if contains(infraNames, wl) {
			t.Errorf("%q should not be in infra (it's KindWorkload), got %v", wl, infraNames)
		}
		if !contains(intent.ResolvedComponents, wl) {
			t.Errorf("%q should be in ResolvedComponents", wl)
		}
	}

	// Core deliberately excludes local-db; ScyllaDB belongs to
	// control-plane/storage/scylla/database profiles.
	if contains(infraNames, "scylladb") {
		t.Errorf("core infra must not include scylladb, got %v", infraNames)
	}

	// Core-only nodes may run AI router, but not services that require a local
	// ScyllaDB runtime dependency.
	if !contains(intent.ResolvedComponents, "ai-router") {
		t.Error("core should resolve ai-router")
	}
	for _, notWant := range []string{"ai-memory", "ai-executor", "ai-watcher", "workflow", "scylla-manager", "scylla-manager-agent"} {
		if contains(intent.ResolvedComponents, notWant) {
			t.Errorf("core must not resolve local-db-backed component %q; resolved=%v", notWant, intent.ResolvedComponents)
		}
	}
}

func TestResolveIntent_DatabaseProfile(t *testing.T) {
	// Simulate scylladb + event healthy.
	units := []unitStatusRecord{
		{Name: "scylla-server.service", State: "active"},
		{Name: "globular-event.service", State: "active"},
		{Name: "globular-ai-memory.service", State: "active"},
		{Name: "globular-ai-executor.service", State: "active"},
	}
	intent, err := ResolveNodeIntent("test-node", []string{"database"}, units, nil)
	if err != nil {
		t.Fatal(err)
	}

	// ScyllaDB should be in infra.
	if !contains(intent.DesiredInfraNames, "scylladb") {
		t.Error("database infra should include scylladb")
	}

	// AI services should be resolved.
	if !contains(intent.ResolvedComponents, "ai-memory") {
		t.Error("database should resolve ai-memory")
	}
	if !contains(intent.ResolvedComponents, "ai-executor") {
		t.Error("database should resolve ai-executor")
	}
	if !contains(intent.ResolvedComponents, "ai-watcher") {
		t.Error("database should resolve ai-watcher")
	}

	// With all deps healthy, ai-memory should be in desired workloads.
	if !contains(intent.DesiredWorkloadNames, "ai-memory") {
		t.Errorf("ai-memory should be desired (deps healthy), got workloads=%v blocked=%v",
			intent.DesiredWorkloadNames, intent.BlockedWorkloads)
	}
}

func TestResolveIntent_GatewayProfile(t *testing.T) {
	intent, err := ResolveNodeIntent("test-node", []string{"gateway"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Gateway infra.
	for _, want := range []string{"gateway", "envoy", "xds"} {
		if !contains(intent.DesiredInfraNames, want) {
			t.Errorf("gateway infra missing %q, got %v", want, intent.DesiredInfraNames)
		}
	}

	// Gateway should NOT include AI services.
	if contains(intent.ResolvedComponents, "ai-memory") {
		t.Error("gateway should NOT resolve ai-memory")
	}
	if contains(intent.ResolvedComponents, "scylladb") {
		t.Error("gateway should NOT resolve scylladb")
	}
}

func TestResolveIntent_StorageDatabaseProfile(t *testing.T) {
	intent, err := ResolveNodeIntent("test-node", []string{"storage", "database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should have both minio and scylladb.
	if !contains(intent.DesiredInfraNames, "minio") {
		t.Error("storage+database should include minio")
	}
	if !contains(intent.DesiredInfraNames, "scylladb") {
		t.Error("storage+database should include scylladb")
	}

	// Should have AI services.
	if !contains(intent.ResolvedComponents, "ai-memory") {
		t.Error("storage+database should resolve ai-memory")
	}
}

func TestResolveIntent_CapabilityResolution(t *testing.T) {
	// "database" profile requires "local-db" capability → scylladb provides it.
	intent, err := ResolveNodeIntent("test-node", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	foundLocalDB := false
	for _, cap := range intent.RequiredCapabilities {
		if cap == CapLocalDB {
			foundLocalDB = true
		}
	}
	if !foundLocalDB {
		t.Error("database profile should require local-db capability")
	}

	if !contains(intent.DesiredInfraNames, "scylladb") {
		t.Error("local-db capability should resolve to scylladb")
	}
}

func TestResolveIntent_MissingInfraTriggersInstall(t *testing.T) {
	// ai-memory needs scylladb. Even if scylladb isn't healthy yet,
	// it should appear in DesiredInfra (to be installed).
	intent, err := ResolveNodeIntent("test-node", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(intent.DesiredInfraNames, "scylladb") {
		t.Error("scylladb should be in DesiredInfra for database profile")
	}

	// ai-memory should be blocked (scylladb not healthy), not in desired workloads.
	if contains(intent.DesiredWorkloadNames, "ai-memory") {
		t.Error("ai-memory should be blocked when scylladb is not healthy")
	}
	found := false
	for _, b := range intent.BlockedWorkloads {
		if b.Name == "ai-memory" {
			found = true
			if !contains(b.MissingDeps, "scylladb") {
				t.Errorf("ai-memory should be blocked on scylladb, got missing deps: %v", b.MissingDeps)
			}
		}
	}
	if !found {
		t.Error("ai-memory should appear in BlockedWorkloads")
	}
}

func TestResolveIntent_TransitiveDependencyExpansion(t *testing.T) {
	// ai-watcher → ai-executor → ai-memory → scylladb, event
	// On a database profile, all should be resolved.
	intent, err := ResolveNodeIntent("test-node", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// event is a runtime dep of ai-memory, but event is infra only on core/compute.
	// On a database-only profile, event is pulled in via dependency expansion.
	if !contains(intent.ResolvedComponents, "event") {
		t.Errorf("event should be pulled in transitively, resolved=%v", intent.ResolvedComponents)
	}
}

func TestResolveIntent_UnknownProfile(t *testing.T) {
	_, err := ResolveNodeIntent("test-node", []string{"does-not-exist"}, nil, nil)
	if err == nil {
		t.Error("expected error for unknown profile")
	}
}

func TestGateDependencies_ScyllaNotHealthy(t *testing.T) {
	desired := map[string]string{
		"ai_memory": "1.0.0",
		"event":     "1.0.0",
	}
	units := []unitStatusRecord{
		{Name: "globular-event.service", State: "active"},
		// scylla-server.service NOT active
	}
	filtered, blocked := GateDependencies(desired, units, nil, nil)
	if _, ok := filtered["ai_memory"]; ok {
		t.Error("ai_memory should be blocked when scylladb is not healthy")
	}
	if _, ok := filtered["event"]; !ok {
		t.Error("event should pass (it's infra)")
	}
	if len(blocked) != 1 || normalizeComponentName(blocked[0].Name) != "ai-memory" {
		t.Errorf("expected ai-memory blocked, got %v", blocked)
	}
}

func TestClassifierBlocksWhenScyllaPortClosed(t *testing.T) {
	TestGateDependencies_ScyllaNotHealthy(t)
}

func TestGateDependencies_AllDepsHealthy(t *testing.T) {
	desired := map[string]string{
		"ai_memory": "1.0.0",
	}
	units := []unitStatusRecord{
		{Name: "scylla-server.service", State: "active"},
		{Name: "globular-event.service", State: "active"},
	}
	filtered, blocked := GateDependencies(desired, units, nil, nil)
	if _, ok := filtered["ai_memory"]; !ok {
		t.Error("ai_memory should pass when all deps healthy")
	}
	if len(blocked) != 0 {
		t.Errorf("no services should be blocked, got %v", blocked)
	}
}

func TestGateDependencies_TransitiveDep(t *testing.T) {
	desired := map[string]string{
		"ai_watcher": "1.0.0",
	}
	units := []unitStatusRecord{
		{Name: "globular-event.service", State: "active"},
		// ai-executor not active
	}
	filtered, blocked := GateDependencies(desired, units, nil, nil)
	if _, ok := filtered["ai_watcher"]; ok {
		t.Error("ai_watcher should be blocked when ai-executor is not healthy")
	}
	if len(blocked) != 1 {
		t.Errorf("expected 1 blocked, got %v", blocked)
	}
}

func TestNodeScope_GatewayExcludesAI(t *testing.T) {
	intent, err := ResolveNodeIntent("test-node", []string{"gateway"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	desired := map[string]string{
		"ai_memory": "1.0.0",
		"gateway":   "1.0.0",
		"envoy":     "1.0.0",
	}
	filtered := FilterDesiredByIntent(desired, intent)
	if _, ok := filtered["ai_memory"]; ok {
		t.Error("gateway node should not get ai_memory")
	}
	// gateway and envoy are infra, might or might not be in desired services
	// (they're managed via unit actions, not desired services).
	// The key assertion: ai_memory is excluded.
}

func TestFilterDesiredByIntent_NilIntent(t *testing.T) {
	desired := map[string]string{"foo": "1.0.0"}
	filtered := FilterDesiredByIntent(desired, nil)
	if len(filtered) != 1 {
		t.Error("nil intent should pass all through")
	}
}

func TestNodeIntentIncludesService(t *testing.T) {
	intent, _ := ResolveNodeIntent("test", []string{"database"}, nil, nil)
	node := &nodeState{
		NodeID:         "test",
		Profiles:       []string{"database"},
		ResolvedIntent: intent,
	}

	if !NodeIntentIncludesService(node, "ai_memory") {
		t.Error("database node should include ai_memory")
	}
	if NodeIntentIncludesService(node, "blog") {
		t.Error("database node should NOT include blog")
	}
}

func TestResolveIntent_RequiredCapabilities(t *testing.T) {
	intent, err := ResolveNodeIntent("test", []string{"core"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	caps := make([]string, len(intent.RequiredCapabilities))
	for i, c := range intent.RequiredCapabilities {
		caps[i] = string(c)
	}
	sort.Strings(caps)

	// Core profile requires: config-store, dns, event-bus, monitoring, object-store
	// Note: local-db (ScyllaDB) is NOT in "core" — it is in control-plane/storage/scylla/database.
	want := []string{"config-store", "dns", "event-bus", "monitoring", "object-store"}
	if len(caps) != len(want) {
		t.Fatalf("core caps: got %v want %v", caps, want)
	}
	for i := range want {
		if caps[i] != want[i] {
			t.Errorf("cap[%d]: got %q want %q", i, caps[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Day 1 phase tests
// ---------------------------------------------------------------------------

func TestDay1Phase_JoinedNode(t *testing.T) {
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapAdmitted,
	}
	phase, _ := ComputeDay1Phase(node)
	if phase != Day1Joined {
		t.Errorf("admitted node: got %q want %q", phase, Day1Joined)
	}
}

func TestDay1Phase_InfraNotInstalled(t *testing.T) {
	// Node with core profile, bootstrap done, but no units reporting.
	intent, _ := ResolveNodeIntent("n1", []string{"core"}, nil, nil)
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"core"},
		ResolvedIntent: intent,
		Units:          nil, // nothing running
	}
	phase, reason := ComputeDay1Phase(node)
	if phase != Day1InfraPlanned {
		t.Errorf("got phase %q want %q (reason: %s)", phase, Day1InfraPlanned, reason)
	}
}

func TestDay1Phase_MinioNonMember_DoesNotReportInfraNotInstalled(t *testing.T) {
	units := []unitStatusRecord{
		{Name: "globular-etcd.service", State: "active"},
		{Name: "scylla-server.service", State: "active"},
		{Name: "globular-gateway.service", State: "active"},
		{Name: "globular-xds.service", State: "active"},
		{Name: "globular-envoy.service", State: "active"},
		{Name: "globular-alertmanager.service", State: "active"},
		{Name: "globular-node-exporter.service", State: "active"},
		{Name: "globular-prometheus.service", State: "active"},
		{Name: "globular-scylla-manager.service", State: "active"},
		{Name: "globular-scylla-manager-agent.service", State: "active"},
		{Name: "globular-oxigraph.service", State: "active"},
		// minio/sidekick intentionally absent while node is non_member.
	}
	intent, err := ResolveNodeIntent("n1", []string{"core", "storage", "control-plane", "gateway"}, units, nil)
	if err != nil {
		t.Fatal(err)
	}
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"core", "storage", "control-plane", "gateway"},
		MinioJoinPhase: MinioJoinNonMember,
		ResolvedIntent: intent,
		Units:          units,
	}
	phase, reason := ComputeDay1Phase(node)
	if phase == Day1InfraPlanned {
		t.Fatalf("unexpected infra planned for non_member node: %s", reason)
	}
	if strings.Contains(reason, "infra not installed:") {
		t.Fatalf("reason should not report minio/sidekick as not installed for non_member: %s", reason)
	}
}

func TestFilterIntentByDesiredRemovesUndesiredCatalogWorkloads(t *testing.T) {
	// media-server supplies title; database supplies workflow. Core alone must
	// not pull workflow because workflow requires local ScyllaDB.
	intent, err := ResolveNodeIntent("n1", []string{"media-server", "database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	filtered := FilterIntentByDesired(intent, map[string]string{
		"dns":            "1.0.56+b1",
		"log":            "1.0.56+b1",
		"event":          "1.0.56+b1",
		"rbac":           "1.0.56+b1",
		"resource":       "1.0.56+b1",
		"authentication": "1.0.56+b1",
		"repository":     "1.0.56+b1",
		"monitoring":     "1.0.56+b1",
		"workflow":       "1.0.56+b1",
		"title":          "1.0.56+b1",
	}, nil, nil)

	if contains(filtered.ResolvedComponents, "blog") || contains(filtered.ResolvedComponents, "catalog") {
		t.Fatalf("filtered intent should not include unpublished catalog-only services: %v", filtered.ResolvedComponents)
	}
	if !contains(filtered.ResolvedComponents, "workflow") || !contains(filtered.ResolvedComponents, "title") {
		t.Fatalf("filtered intent should retain desired services, got %v", filtered.ResolvedComponents)
	}
}

func TestDay1Phase_FilteredIntentIgnoresUndesiredCatalogWorkloads(t *testing.T) {
	units := []unitStatusRecord{
		{Name: "globular-dns.service", State: "active"},
		{Name: "globular-log.service", State: "active"},
		{Name: "globular-event.service", State: "active"},
		{Name: "globular-rbac.service", State: "active"},
		{Name: "globular-resource.service", State: "active"},
		{Name: "globular-authentication.service", State: "active"},
		{Name: "globular-repository.service", State: "active"},
		{Name: "globular-monitoring.service", State: "active"},
		{Name: "globular-title.service", State: "active"},
		{Name: "globular-minio.service", State: "active"},
	}
	intent, err := ResolveNodeIntent("n1", []string{"core"}, units, nil)
	if err != nil {
		t.Fatal(err)
	}
	intent = FilterIntentByDesired(intent, map[string]string{
		"dns":            "1.0.56+b1",
		"log":            "1.0.56+b1",
		"event":          "1.0.56+b1",
		"rbac":           "1.0.56+b1",
		"resource":       "1.0.56+b1",
		"authentication": "1.0.56+b1",
		"repository":     "1.0.56+b1",
		"monitoring":     "1.0.56+b1",
		"title":          "1.0.56+b1",
		"minio":          "RELEASE.2025-09-07T16-13-09Z",
	}, nil, nil)

	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"core"},
		ResolvedIntent: intent,
		Units:          units,
	}
	phase, reason := ComputeDay1Phase(node)
	if phase != Day1Ready {
		t.Fatalf("got phase %q want %q (reason: %s)", phase, Day1Ready, reason)
	}
}

func TestDay1Phase_InfraHealthyWorkloadsBlocked(t *testing.T) {
	// Database node with scylladb healthy but event not healthy → workloads blocked.
	units := []unitStatusRecord{
		{Name: "scylla-server.service", State: "active"},
		// event NOT active
	}
	intent, _ := ResolveNodeIntent("n1", []string{"database"}, units, nil)
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"database"},
		ResolvedIntent: intent,
		Units:          units,
	}
	phase, _ := ComputeDay1Phase(node)
	if phase != Day1WorkloadBlocked {
		t.Errorf("got phase %q want %q", phase, Day1WorkloadBlocked)
	}
}

// TestDay1Phase_KeepalivdInactive_DoesNotBlockInfra is the regression test for
// the invariant: keepalived is Optional=true, so a node with control-plane or
// gateway profile must not be stuck at infra_installed when keepalived is
// inactive (e.g. no VIP has been configured by the operator).
func TestDay1Phase_KeepaliveInactive_DoesNotBlockInfra(t *testing.T) {
	// Simulate a gateway+control-plane node with all required infra active
	// EXCEPT keepalived, which is inactive because no VIP is configured.
	units := []unitStatusRecord{
		{Name: "globular-etcd.service", State: "active"},
		{Name: "globular-minio.service", State: "active"},
		{Name: "scylla-server.service", State: "active"},
		{Name: "globular-gateway.service", State: "active"},
		{Name: "globular-xds.service", State: "active"},
		{Name: "globular-envoy.service", State: "active"},
		{Name: "globular-alertmanager.service", State: "active"},
		{Name: "globular-node-exporter.service", State: "active"},
		{Name: "globular-prometheus.service", State: "active"},
		{Name: "globular-sidekick.service", State: "active"},
		{Name: "globular-scylla-manager.service", State: "active"},
		{Name: "globular-scylla-manager-agent.service", State: "active"},
		// keepalived.service intentionally absent — no VIP configured
	}
	intent, err := ResolveNodeIntent("n1", []string{"core", "control-plane", "gateway"}, units, nil)
	if err != nil {
		t.Fatal(err)
	}

	// keepalived must be in desired_infra so the node-agent knows about it.
	found := false
	for _, name := range intent.DesiredInfraNames {
		if name == "keepalived" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("keepalived should still appear in desired_infra even when Optional=true")
	}

	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"core", "control-plane", "gateway"},
		ResolvedIntent: intent,
		Units:          units,
	}
	phase, reason := ComputeDay1Phase(node)
	// Phase must NOT be infra_installed — keepalived absence must not block.
	if phase == Day1InfraInstalled {
		t.Errorf("keepalived inactive should not block infra health check, got phase=%q reason=%q", phase, reason)
	}
}

func TestDay1Phase_Ready(t *testing.T) {
	// Database node with all deps healthy.
	units := []unitStatusRecord{
		{Name: "scylla-server.service", State: "active"},
		{Name: "globular-event.service", State: "active"},
		{Name: "globular-ai-memory.service", State: "active"},
		{Name: "globular-ai-executor.service", State: "active"},
		{Name: "globular-ai-watcher.service", State: "active"},
		{Name: "globular-workflow.service", State: "active"},
	}
	intent, _ := ResolveNodeIntent("n1", []string{"database"}, units, nil)
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"database"},
		ResolvedIntent: intent,
		Units:          units,
	}
	phase, _ := ComputeDay1Phase(node)
	if phase != Day1Ready {
		t.Errorf("got phase %q want %q", phase, Day1Ready)
	}
}

func TestDay1Phase_BootstrapFailed(t *testing.T) {
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapFailed,
		BootstrapError: "etcd join timeout",
	}
	phase, _ := ComputeDay1Phase(node)
	if phase != Day1InfraBlocked {
		t.Errorf("got phase %q want %q", phase, Day1InfraBlocked)
	}
}

func TestDay1Phase_GatewayNoAIWorkloads(t *testing.T) {
	// Gateway node should NOT include ai-memory in its intent.
	intent, _ := ResolveNodeIntent("gw1", []string{"gateway"}, nil, nil)
	for _, name := range intent.ResolvedComponents {
		if name == "ai-memory" || name == "ai-executor" || name == "ai-watcher" {
			t.Errorf("gateway node should not include %q", name)
		}
	}
	// Gateway should include envoy, xds, gateway.
	for _, want := range []string{"envoy", "xds", "gateway"} {
		if !contains(intent.ResolvedComponents, want) {
			t.Errorf("gateway node missing %q", want)
		}
	}
}

func TestDay1Phase_StorageMinio(t *testing.T) {
	intent, _ := ResolveNodeIntent("s1", []string{"storage"}, nil, nil)
	if !contains(intent.DesiredInfraNames, "minio") {
		t.Errorf("storage node should have minio in infra, got %v", intent.DesiredInfraNames)
	}
	if contains(intent.ResolvedComponents, "ai-memory") {
		t.Error("storage node should not have ai-memory")
	}
}

func TestDay1Phase_JoinedNotReady(t *testing.T) {
	// A joined node that hasn't completed bootstrap is NOT day1-ready.
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapEtcdJoining,
	}
	phase, _ := ComputeDay1Phase(node)
	if day1PhaseReady(phase) {
		t.Error("node in etcd_joining should not be day1-ready")
	}
}

// ---------------------------------------------------------------------------
// Day-1 workload dependency seeding regression tests
// ---------------------------------------------------------------------------

// TestDay1WorkloadDepSeeding_EventDoesNotBlockRollout is the primary regression
// test for the Day-1 stall: mcp is in desired-state but its runtime dep (event)
// is not healthy, which used to hard-block mcp and make Day-1 appear stuck
// forever.
//
// Invariant: event-bus availability is observability-side, not authority-side;
// it must not prevent rollout of otherwise installable workloads.
func TestDay1WorkloadDepSeeding_EventDoesNotBlockRollout(t *testing.T) {
	intent, err := ResolveNodeIntent("joining-node", []string{"control-plane"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Desired-state contains mcp but NOT event — the under-populated state that
	// used to cause the Day-1 stall.
	desiredMCPOnly := map[string]string{
		"mcp": "1.0.56+b1",
	}

	filtered := FilterIntentByDesired(intent, desiredMCPOnly, nil, nil)
	for i := range filtered.BlockedWorkloads {
		if normalizeComponentName(filtered.BlockedWorkloads[i].Name) == "mcp" {
			t.Fatalf("mcp must not be blocked only because event is absent/unhealthy: kind=%s reason=%s",
				filtered.BlockedWorkloads[i].Kind, filtered.BlockedWorkloads[i].Reason)
		}
	}

	// ComputeDay1Phase must not return Day1WorkloadBlocked because of event.
	node := &nodeState{
		NodeID:         "joining-node",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"compute"},
		ResolvedIntent: filtered,
		Units:          nil,
	}
	phase, reason := ComputeDay1Phase(node)
	if phase == Day1WorkloadBlocked {
		t.Fatalf("event-only dependency must not produce Day1WorkloadBlocked: %s", reason)
	}

	// Second reconcile: event is now seeded into desired-state but is not yet
	// healthy. Event is an observability bus, not a rollout authority; it must
	// not hard-block mcp or any other workload from being installed.
	desiredBoth := map[string]string{
		"mcp":   "1.0.56+b1",
		"event": "1.0.56+b1",
	}
	filtered2 := FilterIntentByDesired(intent, desiredBoth, nil, nil)
	for i := range filtered2.BlockedWorkloads {
		if normalizeComponentName(filtered2.BlockedWorkloads[i].Name) == "mcp" {
			t.Fatalf("mcp must not be blocked only because event is desired but unhealthy: kind=%s reason=%s",
				filtered2.BlockedWorkloads[i].Kind, filtered2.BlockedWorkloads[i].Reason)
		}
	}

	// Third reconcile: event is installed and healthy. mcp must be unblocked.
	healthyUnits := []unitStatusRecord{
		{Name: "globular-event.service", State: "active"},
		{Name: "globular-mcp.service", State: "active"},
	}
	intent3, _ := ResolveNodeIntent("joining-node", []string{"compute"}, healthyUnits, desiredBoth)
	filtered3 := FilterIntentByDesired(intent3, desiredBoth, nil, nil)
	for _, bw := range filtered3.BlockedWorkloads {
		if normalizeComponentName(bw.Name) == "mcp" {
			t.Errorf("mcp should be unblocked when event is healthy, still blocked: kind=%s reason=%s", bw.Kind, bw.Reason)
		}
	}
}

// TestDay1WorkloadDepSeeding_TransitiveChain verifies that transitive dep chains
// are all classified as seeding when none of the deps are in desired-state.
// e.g. ai-watcher → ai-executor → ai-memory → scylladb, event
func TestDay1WorkloadDepSeeding_TransitiveChain(t *testing.T) {
	intent, err := ResolveNodeIntent("n1", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Only ai-watcher in desired-state; its full dep chain is absent.
	desiredWatcherOnly := map[string]string{
		"ai-watcher": "1.0.0",
	}
	filtered := FilterIntentByDesired(intent, desiredWatcherOnly, nil, nil)

	// Every blocked workload whose deps are absent from desired must be seeding,
	// never dependency_not_ready.
	for _, bw := range filtered.BlockedWorkloads {
		if bw.Kind == "dependency_not_ready" {
			// Only acceptable if every blocking dep IS in desired-state.
			for _, dep := range bw.MissingDeps {
				if _, ok := desiredWatcherOnly[normalizeComponentName(dep)]; !ok {
					t.Errorf("workload %s blocked as dependency_not_ready but dep %s absent from desired-state — should be seeding",
						bw.Name, dep)
				}
			}
		}
	}
}

// TestDay1Phase_SeedingIsNotTerminal verifies that Day1WorkloadBlocked is not
// returned when all blocked workloads have unseeded (transient) deps.
func TestDay1Phase_SeedingIsNotTerminal(t *testing.T) {
	intent, err := ResolveNodeIntent("n1", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Minimal desired-state — only mcp, its dep event is missing.
	filtered := FilterIntentByDesired(intent, map[string]string{"mcp": "1.0.0"}, nil, nil)

	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"database"},
		ResolvedIntent: filtered,
	}
	phase, _ := ComputeDay1Phase(node)
	if phase == Day1WorkloadBlocked {
		t.Error("Day1WorkloadBlocked must not be returned when all blocks are transient (deps being seeded)")
	}
}

// TestDay1Phase_UnresolvableDep verifies that when a desired workload's runtime
// dep cannot be seeded (version unresolvable), Day-1 must reach
// Day1WorkloadBlocked — not stay as Day1WorkloadsPlanned indefinitely.
//
// Scenario:
//   - desired-state contains ai-memory
//   - scylladb is absent from desired-state
//   - scylladb cannot be resolved from installed_state (unresolvable set)
//   - GateDependencies must classify the block as missing_desired_dependency_unresolvable
//   - ComputeDay1Phase must return Day1WorkloadBlocked
func TestDay1Phase_UnresolvableDep(t *testing.T) {
	intent, err := ResolveNodeIntent("n1", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate materializeMissingInfraDesired failing to resolve scylladb's version.
	unresolvable := map[string]bool{"scylladb": true}

	// Pass unresolvable directly to FilterIntentByDesired — this is the actual
	// production path in reconcile_nodes.go. The block kind must be
	// missing_desired_dependency_unresolvable without any manual injection.
	filtered := FilterIntentByDesired(intent, map[string]string{"ai-memory": "1.0.0"}, nil, unresolvable)

	var aiMemoryBlock *BlockedWorkload
	for i := range filtered.BlockedWorkloads {
		if normalizeComponentName(filtered.BlockedWorkloads[i].Name) == "ai-memory" {
			aiMemoryBlock = &filtered.BlockedWorkloads[i]
		}
	}
	if aiMemoryBlock == nil {
		t.Fatal("ai-memory not found in FilterIntentByDesired blocked list")
	}
	if aiMemoryBlock.Kind != "missing_desired_dependency_unresolvable" {
		t.Errorf("expected missing_desired_dependency_unresolvable from FilterIntentByDesired, got %q", aiMemoryBlock.Kind)
	}

	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"database"},
		ResolvedIntent: filtered,
	}
	phase, reason := ComputeDay1Phase(node)
	if phase != Day1WorkloadBlocked {
		t.Errorf("expected Day1WorkloadBlocked when dep is unresolvable, got %s", phase)
	}
	if !strings.Contains(reason, "missing desired dependency unresolvable") {
		t.Errorf("reason should mention unresolvable dep, got %q", reason)
	}
	if !strings.Contains(reason, "scylladb") {
		t.Errorf("reason should name the unresolvable dep, got %q", reason)
	}
	if !strings.Contains(reason, "ai-memory") {
		t.Errorf("reason should name the requiring service, got %q", reason)
	}
}

// TestGateDependencies_UnresolvableMarksThatKind verifies that when a dep is
// both absent from desired-state AND in the unresolvable set, GateDependencies
// classifies the block as missing_desired_dependency_unresolvable (not seeding).
func TestGateDependencies_UnresolvableMarksThatKind(t *testing.T) {
	desired := map[string]string{"ai-memory": "1.0.0"}
	unresolvable := map[string]bool{"scylladb": true}

	_, blocked := GateDependencies(desired, nil, nil, unresolvable)

	if len(blocked) == 0 {
		t.Fatal("ai-memory should be blocked when scylladb is not healthy")
	}
	if blocked[0].Kind != "missing_desired_dependency_unresolvable" {
		t.Errorf("expected missing_desired_dependency_unresolvable, got %q", blocked[0].Kind)
	}
}

// TestGateDependencies_SeedingStillTransientWithoutUnresolvable checks that
// when a dep is absent from desired-state and NOT in unresolvable, the block
// kind remains dependency_seeding_in_progress (can still converge).
func TestGateDependencies_SeedingStillTransientWithoutUnresolvable(t *testing.T) {
	desired := map[string]string{"ai-memory": "1.0.0"}

	_, blocked := GateDependencies(desired, nil, nil, nil)

	if len(blocked) == 0 {
		t.Fatal("ai-memory should be blocked when scylladb is not healthy")
	}
	if blocked[0].Kind != "dependency_seeding_in_progress" {
		t.Errorf("expected dependency_seeding_in_progress, got %q", blocked[0].Kind)
	}
}

// errorOnApplyStore wraps a real memStore and injects an error on Apply for a
// specific resource type, allowing tests to simulate storage failures.
type errorOnApplyStore struct {
	resourcestore.Store
	failType string
}

func (e *errorOnApplyStore) Apply(ctx context.Context, typ string, obj interface{}) (interface{}, error) {
	if typ == e.failType {
		return nil, fmt.Errorf("injected Apply failure for %s", typ)
	}
	return e.Store.Apply(ctx, typ, obj)
}

// TestMaterializeDeps_InfraApplyFailureMarksUnresolvable verifies that when an
// infra/command dep's version resolves but InfrastructureRelease Apply fails,
// materializeMissingInfraDesired marks that dep as unresolvable — ensuring
// Day-1 reaches Day1WorkloadBlocked instead of looping as WorkloadsPlanned.
//
// Uses ai-memory (KindWorkload, depends on scylladb KindInfrastructure) as the
// workload under test.
func TestMaterializeDeps_InfraApplyFailureMarksUnresolvable(t *testing.T) {
	// Set up a server with a ready node that has scylladb installed.
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"ready-node": {
				Status:            "ready",
				Day1Phase:         Day1Ready,
				InstalledVersions: map[string]string{"event": "1.0.0", "scylladb": "6.2.1"},
			},
		},
	}
	// Use a store that fails Apply for InfrastructureRelease.
	base := resourcestore.NewMemStore()
	srv.resources = &errorOnApplyStore{Store: base, failType: "InfrastructureRelease"}

	// desired-state: ai-memory plus its non-Scylla runtime dependency; scylladb
	// is absent so this test isolates the infra Apply failure.
	desiredCanon := map[string]string{"ai-memory": "1.0.0", "event": "1.0.0"}

	intent, err := ResolveNodeIntent("n1", []string{"database"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, unresolvable := srv.materializeMissingInfraDesired(context.Background(), intent, desiredCanon)

	scyllaCanon := normalizeComponentName("scylladb")
	if !unresolvable[scyllaCanon] {
		t.Errorf("scylladb should be marked unresolvable after Apply failure; got map: %v", unresolvable)
	}

	// Verify the full Day-1 path: FilterIntentByDesired with unresolvable set
	// must produce Day1WorkloadBlocked (not Day1WorkloadsPlanned).
	filtered := FilterIntentByDesired(intent, desiredCanon, nil, unresolvable)
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"database"},
		ResolvedIntent: filtered,
	}
	phase, reason := ComputeDay1Phase(node)
	if phase != Day1WorkloadBlocked {
		t.Errorf("expected Day1WorkloadBlocked when infra dep Apply fails, got %s (%s)", phase, reason)
	}
	if !strings.Contains(reason, "missing desired dependency unresolvable") {
		t.Errorf("reason should mention unresolvable dep, got %q", reason)
	}
}

// TestMaterializeDeps_MissingCatalogDepMarksUnresolvable verifies that when a
// workload's RuntimeLocalDependency references a name not in the component
// catalog, materializeMissingInfraDesired marks it unresolvable so Day-1
// reaches Day1WorkloadBlocked instead of looping as WorkloadsPlanned.
//
// We inject a synthetic catalog entry at the catalog slice level (bypassing
// ResolveNodeIntent which validates deps) and construct the intent manually.
// This simulates the real scenario: a renamed or removed dep that is still
// referenced in a deployed workload's catalog metadata.
func TestMaterializeDeps_MissingCatalogDepMarksUnresolvable(t *testing.T) {
	const ghostDep = "ghost-dep-that-does-not-exist"
	const synthName = "test-synthetic-workload"

	// Inject synthetic catalog entry directly — bypassing ResolveNodeIntent
	// validation so we can represent a stale/renamed dep reference.
	synthetic := &Component{
		Name:                     synthName,
		Kind:                     KindWorkload,
		Unit:                     "globular-test-synthetic-workload.service",
		Profiles:                 []string{"compute"},
		RuntimeLocalDependencies: []string{ghostDep},
	}
	catalog = append(catalog, synthetic)
	catalogIndex[synthName] = synthetic
	defer func() {
		catalog = catalog[:len(catalog)-1]
		delete(catalogIndex, synthName)
	}()

	srv := &server{}
	srv.state = &controllerState{Nodes: map[string]*nodeState{}}
	srv.resources = resourcestore.NewMemStore()

	// Build a minimal intent manually — the synthetic workload is desired and
	// its ghost dep triggers the "not in catalog" BFS branch.
	desiredCanon := map[string]string{synthName: "1.0.0"}
	intent := &NodeIntent{
		NodeID:               "n1",
		Profiles:             []string{"compute"},
		ResolvedComponents:   []string{synthName},
		DesiredWorkloads:     []*Component{synthetic},
		DesiredWorkloadNames: []string{synthName},
		BlockedWorkloads: []BlockedWorkload{{
			Name:        synthName,
			MissingDeps: []string{ghostDep},
			Reason:      "waiting for: " + ghostDep,
		}},
	}

	_, unresolvable := srv.materializeMissingInfraDesired(context.Background(), intent, desiredCanon)

	ghostCanon := normalizeComponentName(ghostDep)
	if !unresolvable[ghostCanon] {
		t.Errorf("dep %q missing from catalog should be marked unresolvable; got map: %v", ghostDep, unresolvable)
	}

	// Full Day-1 path: must be WorkloadBlocked, not WorkloadsPlanned.
	filtered := FilterIntentByDesired(intent, desiredCanon, nil, unresolvable)
	node := &nodeState{
		NodeID:         "n1",
		BootstrapPhase: BootstrapWorkloadReady,
		Profiles:       []string{"compute"},
		ResolvedIntent: filtered,
	}
	phase, reason := ComputeDay1Phase(node)
	if phase != Day1WorkloadBlocked {
		t.Errorf("expected Day1WorkloadBlocked for missing-catalog dep, got %s (%s)", phase, reason)
	}
	if !strings.Contains(reason, "missing desired dependency unresolvable") {
		t.Errorf("reason should mention unresolvable dep, got %q", reason)
	}
}

// TestMaterializeDeps_ExistingInfraReleaseReflectedInDesiredCanon verifies
// that when an InfrastructureRelease for a required infra dep already exists in
// the store, materializeMissingInfraDesired updates desiredCanon locally so
// FilterIntentByDesired classifies the block as dependency_not_ready (the dep
// is "desired but not yet healthy") rather than dependency_seeding_in_progress.
//
// Without this fix, loadDesiredServices (which only loads ServiceDesiredVersion)
// would not include the InfrastructureRelease, leaving desiredCanon unaware
// that scylladb is desired — causing perpetual seeding_in_progress.
func TestMaterializeDeps_ExistingInfraReleaseReflectedInDesiredCanon(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"ready-node": {
				Status:            "ready",
				Day1Phase:         Day1Ready,
				InstalledVersions: map[string]string{"scylladb": "6.2.1"},
			},
		},
	}
	store := resourcestore.NewMemStore()
	srv.resources = store

	// Pre-populate the store with an existing InfrastructureRelease for scylladb.
	relName := defaultPublisherID() + "/scylladb"
	_, err := store.Apply(context.Background(), "InfrastructureRelease", &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: relName},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			PublisherID: defaultPublisherID(),
			Component:   "scylladb",
			Version:     "6.2.1",
		},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{},
	})
	if err != nil {
		t.Fatalf("pre-populate InfrastructureRelease: %v", err)
	}

	// desired-state: only ai-memory; scylladb absent from ServiceDesiredVersion.
	desiredCanon := map[string]string{"ai-memory": "1.0.0"}

	intent, err := ResolveNodeIntent("n1", []string{"compute"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, unresolvable := srv.materializeMissingInfraDesired(context.Background(), intent, desiredCanon)

	// scylladb must NOT be unresolvable — the existing release was found.
	scyllaCanon := normalizeComponentName("scylladb")
	if unresolvable[scyllaCanon] {
		t.Error("scylladb should not be unresolvable when an existing InfrastructureRelease is present")
	}

	// desiredCanon must now include scylladb (reflected locally from the existing release).
	if desiredCanon[scyllaCanon] == "" {
		t.Errorf("scylladb should be in desiredCanon after materialize; got %v", desiredCanon)
	}

	// FilterIntentByDesired should classify the scylladb block as
	// dependency_not_ready (desired but not yet healthy), not seeding_in_progress.
	filtered := FilterIntentByDesired(intent, desiredCanon, nil, unresolvable)
	var aiBlock *BlockedWorkload
	for i := range filtered.BlockedWorkloads {
		if normalizeComponentName(filtered.BlockedWorkloads[i].Name) == "ai-memory" {
			aiBlock = &filtered.BlockedWorkloads[i]
		}
	}
	if aiBlock == nil {
		// ai-memory may have been moved to desired workloads if all deps resolved —
		// that is also acceptable (means scylladb is now in desired and healthy).
		return
	}
	if aiBlock.Kind == "dependency_seeding_in_progress" {
		t.Errorf("ai-memory block should be dependency_not_ready (scylladb is desired), got dependency_seeding_in_progress")
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// TestFilterVersionsForNode_ExcludesUnauthorizedServices locks SCAR-4
// (controller.desired_services_hash_must_be_profile_scoped): the per-node desired
// hash set must be scoped to what the node's profiles AUTHORIZE (via the shared
// placement law book), so it is comparable to the node-scoped applied hash. A
// service the catalog forbids on the node's profiles must be excluded; authorized
// services and catalog-UNKNOWN services must be kept.
func TestFilterVersionsForNode_ExcludesUnauthorizedServices(t *testing.T) {
	// Node with no media-server / compute / ai profile (globule-nuc's shape).
	node := &nodeState{Profiles: []string{"control-plane", "core", "gateway", "storage"}}
	desired := map[string]string{
		"rbac":               "1.2.272", // catalog [core,compute] -> authorized via core -> kept
		"media":              "1.2.272", // catalog [media-server] -> unauthorized here -> excluded
		"title":              "1.2.272", // catalog [media-server] -> excluded
		"torrent":            "1.2.272", // catalog [media-server] -> excluded
		"zzz-unknown-widget": "9.9.9",   // no catalog entry -> unknown != unauthorized -> kept
	}
	got := filterVersionsForNode(desired, node)

	for _, kept := range []string{"rbac", "zzz-unknown-widget"} {
		if _, ok := got[canonicalServiceName(kept)]; !ok {
			t.Errorf("authorized/unknown service %q must be kept in the per-node desired set; got %v", kept, got)
		}
	}
	for _, dropped := range []string{"media", "title", "torrent"} {
		if _, ok := got[canonicalServiceName(dropped)]; ok {
			t.Errorf("media-server-only service %q must be excluded on a non-media-server node; got %v", dropped, got)
		}
	}

	// The SAME services ARE authorized (kept) on a node that carries media-server —
	// the exclusion is profile-driven, not a blanket drop.
	mediaNode := &nodeState{Profiles: []string{"control-plane", "core", "media-server", "storage"}}
	got2 := filterVersionsForNode(map[string]string{"media": "1", "rbac": "1"}, mediaNode)
	if _, ok := got2[canonicalServiceName("media")]; !ok {
		t.Errorf("media must be kept on a media-server node; got %v", got2)
	}
}

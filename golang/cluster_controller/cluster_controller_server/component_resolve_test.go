package main

import (
	"sort"
	"testing"
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

	// dns, discovery, monitoring, event, rbac, file are KindWorkload (ManagedUnit=true)
	// — they appear in ResolvedComponents but NOT in DesiredInfraNames.
	for _, wl := range []string{"event", "rbac", "file", "dns", "discovery", "monitoring"} {
		if contains(infraNames, wl) {
			t.Errorf("%q should not be in infra (it's KindWorkload), got %v", wl, infraNames)
		}
		if !contains(intent.ResolvedComponents, wl) {
			t.Errorf("%q should be in ResolvedComponents", wl)
		}
	}

	// Core now includes ScyllaDB (CapLocalDB) — all nodes running services need it.
	if !contains(infraNames, "scylladb") {
		t.Error("core infra should include scylladb")
	}

	// Workloads are blocked when units aren't reporting (no healthy units).
	// But they should still be in ResolvedComponents.
	if !contains(intent.ResolvedComponents, "ai-router") {
		t.Error("core should resolve ai-router")
	}
	if !contains(intent.ResolvedComponents, "ai-memory") {
		t.Error("core should resolve ai-memory (core now includes ScyllaDB)")
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
	filtered, blocked := GateDependencies(desired, units, nil)
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

func TestGateDependencies_AllDepsHealthy(t *testing.T) {
	desired := map[string]string{
		"ai_memory": "1.0.0",
	}
	units := []unitStatusRecord{
		{Name: "scylla-server.service", State: "active"},
		{Name: "globular-event.service", State: "active"},
	}
	filtered, blocked := GateDependencies(desired, units, nil)
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
	filtered, blocked := GateDependencies(desired, units, nil)
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

	// Core profile requires: config-store, dns, event-bus, monitoring, object-store, service-discovery
	// Note: local-db (ScyllaDB) is NOT in "core" — it is in control-plane/storage/scylla/database.
	want := []string{"config-store", "dns", "event-bus", "monitoring", "object-store", "service-discovery"}
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

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

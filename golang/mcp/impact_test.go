package main

import (
	"context"
	"testing"
)

// ── Mock DependencySource ────────────────────────────────────────────────────

type mockDeps struct {
	deps  map[string][]ServiceDependency
	ports map[string][]int
}

func (m *mockDeps) Dependencies(service string) []ServiceDependency {
	return m.deps[service]
}

func (m *mockDeps) DefaultPorts(service string) []int {
	return m.ports[service]
}

func (m *mockDeps) ReverseDeps(service string, installed []string) []string {
	installedSet := make(map[string]bool, len(installed))
	for _, svc := range installed {
		installedSet[svc] = true
	}
	var dependents []string
	for svc, deps := range m.deps {
		if !installedSet[svc] {
			continue
		}
		for _, d := range deps {
			if d.Name == service && d.Required {
				dependents = append(dependents, svc)
				break
			}
		}
	}
	return dependents
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestComputeDecision_MissingDependency(t *testing.T) {
	report := &ImpactReport{
		DependenciesMissing: []string{"persistence"},
		RiskLevel:           "high",
	}
	decision := &AdmissionDecision{Status: "allow"}

	computeDecision(report, decision)

	if decision.Status != "requires_remediation" {
		t.Fatalf("expected requires_remediation, got %s", decision.Status)
	}
	if len(decision.MissingRequirements) != 1 || decision.MissingRequirements[0] != "persistence" {
		t.Fatalf("expected missing requirement 'persistence', got %v", decision.MissingRequirements)
	}
	if len(decision.SuggestedRemediation) == 0 {
		t.Fatal("expected suggested remediation")
	}
}

func TestComputeDecision_PortConflict(t *testing.T) {
	report := &ImpactReport{
		PortConflicts: []int{10601},
		RiskLevel:     "high",
	}
	decision := &AdmissionDecision{Status: "allow"}

	computeDecision(report, decision)

	if decision.Status != "block" {
		t.Fatalf("expected block, got %s", decision.Status)
	}
	if len(decision.Reasons) == 0 {
		t.Fatal("expected reasons for block")
	}
}

func TestComputeDecision_SafeInstall(t *testing.T) {
	report := &ImpactReport{
		PackageFound:  true,
		PackageStatus: "found",
		RiskLevel:     "low",
	}
	decision := &AdmissionDecision{Status: "allow"}

	computeDecision(report, decision)

	if decision.Status != "allow" {
		t.Fatalf("expected allow, got %s", decision.Status)
	}
}

func TestComputeDecision_PolicyGated(t *testing.T) {
	report := &ImpactReport{
		PackageFound:     true,
		PackageStatus:    "found",
		ServicesAffected: []string{"ldap"},
		RiskLevel:        "medium",
	}
	decision := &AdmissionDecision{Status: "allow"}

	computeDecision(report, decision)

	if decision.Status != "allow_with_approval" {
		t.Fatalf("expected allow_with_approval, got %s", decision.Status)
	}
	if !decision.RequiresApproval {
		t.Fatal("expected requires_approval to be true")
	}
}

func TestComputeDecision_PackageNotFound(t *testing.T) {
	report := &ImpactReport{
		PackageFound:  false,
		PackageStatus: "not_found",
		RiskLevel:     "low",
	}
	decision := &AdmissionDecision{Status: "allow"}

	computeDecision(report, decision)

	if decision.Status != "block" {
		t.Fatalf("expected block, got %s", decision.Status)
	}
}

func TestPlannerEvaluate_NoDeps_NoClients(t *testing.T) {
	deps := &mockDeps{
		deps:  map[string][]ServiceDependency{},
		ports: map[string][]int{"event": {10102}},
	}

	planner := NewPlanner(deps, nil)
	// Use a command without an approval policy to test the "allow" path
	plan := OperationPlan{
		Command:       "generate service",
		TargetService: "event",
		TargetVersion: "",
		Timestamp:     "2026-03-18T00:00:00Z",
	}

	report, decision := planner.Evaluate(context.Background(), plan)

	// No deps, no clients → package check unavailable, no conflicts
	if report.PackageStatus != "check_unavailable" {
		t.Fatalf("expected check_unavailable, got %s", report.PackageStatus)
	}
	// Decision should be allow (degraded gracefully — no deps, no conflicts)
	if decision.Status != "allow" {
		t.Fatalf("expected allow, got %s", decision.Status)
	}
}

func TestPlannerEvaluate_MissingDep_NoClients(t *testing.T) {
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"ldap": {
				{Name: "persistence", Required: true},
				{Name: "authentication", Required: true},
			},
		},
		ports: map[string][]int{"ldap": {10601}},
	}

	planner := NewPlanner(deps, nil)
	plan := OperationPlan{
		Command:       "services desired set",
		TargetService: "ldap",
		TargetVersion: "",
		Timestamp:     "2026-03-18T00:00:00Z",
	}

	report, decision := planner.Evaluate(context.Background(), plan)

	// With no client pool, getInstalledServices returns nil → all deps missing
	if len(report.DependenciesMissing) != 2 {
		t.Fatalf("expected 2 missing deps, got %d: %v", len(report.DependenciesMissing), report.DependenciesMissing)
	}
	if decision.Status != "requires_remediation" {
		t.Fatalf("expected requires_remediation, got %s", decision.Status)
	}
}

func TestNormalizePlan_DesiredSet(t *testing.T) {
	plan := NormalizePlan("globular services desired set ldap 0.0.1", nil)

	if plan.TargetService != "ldap" {
		t.Fatalf("expected target service 'ldap', got %q", plan.TargetService)
	}
	if plan.TargetVersion != "0.0.1" {
		t.Fatalf("expected target version '0.0.1', got %q", plan.TargetVersion)
	}
}

func TestNormalizePlan_DesiredRemove(t *testing.T) {
	plan := NormalizePlan("globular services desired remove ldap", nil)

	if plan.TargetService != "ldap" {
		t.Fatalf("expected target service 'ldap', got %q", plan.TargetService)
	}
}

func TestNormalizePlan_PkgPublish(t *testing.T) {
	plan := NormalizePlan("globular pkg publish echo", nil)

	if plan.TargetService != "echo" {
		t.Fatalf("expected target service 'echo', got %q", plan.TargetService)
	}
}

func TestNormalizePlan_WithArgs(t *testing.T) {
	plan := NormalizePlan("globular services desired set", []string{"ldap", "0.0.2"})

	if plan.TargetService != "ldap" {
		t.Fatalf("expected target service 'ldap', got %q", plan.TargetService)
	}
	if plan.TargetVersion != "0.0.2" {
		t.Fatalf("expected target version '0.0.2', got %q", plan.TargetVersion)
	}
}

// ── Descriptor & Transitive Tests ────────────────────────────────────────────

// mockDescriptorLookup implements DescriptorLookup for testing.
type mockDescriptorLookup struct {
	descriptors map[string]*ServiceDescriptor
}

func (m *mockDescriptorLookup) Descriptor(_ context.Context, service string) (*ServiceDescriptor, error) {
	desc, ok := m.descriptors[service]
	if !ok {
		return nil, ErrNoDescriptor
	}
	return desc, nil
}

func TestResolveTransitiveDeps_LinearChain(t *testing.T) {
	// A → B → C (no cycle)
	lookup := func(svc string) (*ServiceDescriptor, error) {
		switch svc {
		case "A":
			return &ServiceDescriptor{Name: "A", Requires: []string{"B"}}, nil
		case "B":
			return &ServiceDescriptor{Name: "B", Requires: []string{"C"}}, nil
		case "C":
			return &ServiceDescriptor{Name: "C"}, nil
		}
		return nil, ErrNoDescriptor
	}

	deps, cycle := ResolveTransitiveDeps("A", lookup)

	if cycle {
		t.Fatal("unexpected cycle")
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 transitive deps [B, C], got %v", deps)
	}
	// B should come before C (BFS order)
	if deps[0] != "B" || deps[1] != "C" {
		t.Fatalf("expected [B, C], got %v", deps)
	}
}

func TestResolveTransitiveDeps_CycleDetection(t *testing.T) {
	// A → B → A (cycle back to root)
	lookup := func(svc string) (*ServiceDescriptor, error) {
		switch svc {
		case "A":
			return &ServiceDescriptor{Name: "A", Requires: []string{"B"}}, nil
		case "B":
			return &ServiceDescriptor{Name: "B", Requires: []string{"A"}}, nil
		}
		return nil, ErrNoDescriptor
	}

	deps, cycle := ResolveTransitiveDeps("A", lookup)

	if !cycle {
		t.Fatal("expected cycle detection")
	}
	if len(deps) != 1 || deps[0] != "B" {
		t.Fatalf("expected [B], got %v", deps)
	}
}

func TestResolveTransitiveDeps_Diamond(t *testing.T) {
	// A → B, A → C, B → D, C → D (diamond, no cycle)
	lookup := func(svc string) (*ServiceDescriptor, error) {
		switch svc {
		case "A":
			return &ServiceDescriptor{Name: "A", Requires: []string{"B", "C"}}, nil
		case "B":
			return &ServiceDescriptor{Name: "B", Requires: []string{"D"}}, nil
		case "C":
			return &ServiceDescriptor{Name: "C", Requires: []string{"D"}}, nil
		case "D":
			return &ServiceDescriptor{Name: "D"}, nil
		}
		return nil, ErrNoDescriptor
	}

	deps, cycle := ResolveTransitiveDeps("A", lookup)

	if cycle {
		t.Fatal("unexpected cycle in diamond")
	}
	// D should appear exactly once
	dCount := 0
	for _, d := range deps {
		if d == "D" {
			dCount++
		}
	}
	if dCount != 1 {
		t.Fatalf("expected D exactly once, got deps: %v", deps)
	}
}

func TestPlannerWithDescriptor_UsesDescriptor(t *testing.T) {
	staticDeps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"ldap": {{Name: "static-dep", Required: true}},
		},
		ports: map[string][]int{},
	}
	descLookup := &mockDescriptorLookup{
		descriptors: map[string]*ServiceDescriptor{
			"ldap": {Name: "ldap", Requires: []string{"persistence", "authentication"}},
		},
	}

	planner := NewPlannerWithDescriptor(staticDeps, descLookup, nil)
	plan := OperationPlan{
		Command:       "generate service",
		TargetService: "ldap",
		Timestamp:     "2026-03-18T00:00:00Z",
	}

	report, _ := planner.Evaluate(context.Background(), plan)

	if report.DependencySource != "descriptor" {
		t.Fatalf("expected descriptor source, got %q", report.DependencySource)
	}
	// Should use descriptor requires, not static "static-dep"
	if containsStr(report.DependenciesRequired, "static-dep") {
		t.Fatal("should not use static deps when descriptor is available")
	}
	if !containsStr(report.DependenciesRequired, "persistence") {
		t.Fatal("expected 'persistence' from descriptor")
	}
}

func TestPlannerWithDescriptor_FallsBackToStatic(t *testing.T) {
	staticDeps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"ldap": {{Name: "persistence", Required: true}},
		},
		ports: map[string][]int{},
	}
	// Descriptor source that returns nothing
	descLookup := &mockDescriptorLookup{
		descriptors: map[string]*ServiceDescriptor{},
	}

	planner := NewPlannerWithDescriptor(staticDeps, descLookup, nil)
	plan := OperationPlan{
		Command:       "generate service",
		TargetService: "ldap",
		Timestamp:     "2026-03-18T00:00:00Z",
	}

	report, _ := planner.Evaluate(context.Background(), plan)

	if report.DependencySource != "static" {
		t.Fatalf("expected static fallback, got %q", report.DependencySource)
	}
	if !containsStr(report.DependenciesRequired, "persistence") {
		t.Fatal("expected 'persistence' from static fallback")
	}
}

// ── Reverse Dependency Tests (Phase 7C) ──────────────────────────────────────

func TestComputeDecision_ReverseDependents(t *testing.T) {
	report := &ImpactReport{
		ReverseDependents: []string{"ldap", "blog"},
		RiskLevel:         "high",
	}
	decision := &AdmissionDecision{Status: "allow"}

	computeDecision(report, decision)

	if decision.Status != "requires_remediation" {
		t.Fatalf("expected requires_remediation, got %s", decision.Status)
	}
	if len(decision.SuggestedRemediation) != 2 {
		t.Fatalf("expected 2 remediation suggestions, got %v", decision.SuggestedRemediation)
	}
}

func TestPlannerRemove_WithDependents(t *testing.T) {
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"persistence": nil,
			"ldap":        {{Name: "persistence", Required: true}},
			"blog":        {{Name: "persistence", Required: true}},
			"torrent":     nil,
		},
		ports: map[string][]int{},
	}

	planner := NewPlanner(deps, nil)
	plan := OperationPlan{
		Command:       "globular services desired remove persistence",
		TargetService: "persistence",
		Operation:     "remove",
		Timestamp:     "2026-03-18T00:00:00Z",
	}

	// Simulate installed services — ldap and blog are installed and depend on persistence
	// Since clients is nil, getInstalledServices returns nil → reverse check warns but doesn't block.
	// To test properly, we need to test computeDecision directly with reverse deps set.
	report, _ := planner.Evaluate(context.Background(), plan)

	// With nil clients, reverse check can't query installed services
	if len(report.Warnings) == 0 {
		t.Fatal("expected warning about reverse dep check unavailability")
	}
}

func TestReverseDeps_StaticSource(t *testing.T) {
	src := NewStaticDependencySource()

	// persistence is required by many services
	installed := []string{"ldap", "blog", "authentication", "torrent", "event"}
	dependents := src.ReverseDeps("persistence", installed)

	if len(dependents) == 0 {
		t.Fatal("expected dependents for persistence")
	}
	// ldap, blog, authentication should all depend on persistence
	for _, expected := range []string{"ldap", "blog", "authentication"} {
		found := false
		for _, d := range dependents {
			if d == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in dependents, got %v", expected, dependents)
		}
	}
	// torrent and event should NOT be dependents of persistence
	for _, notExpected := range []string{"torrent", "event"} {
		for _, d := range dependents {
			if d == notExpected {
				t.Fatalf("%q should not depend on persistence", notExpected)
			}
		}
	}
}

func TestReverseDeps_LeafService(t *testing.T) {
	src := NewStaticDependencySource()

	// torrent has no dependents
	installed := []string{"ldap", "blog", "persistence", "torrent"}
	dependents := src.ReverseDeps("torrent", installed)

	if len(dependents) != 0 {
		t.Fatalf("expected no dependents for torrent, got %v", dependents)
	}
}

func TestNormalizePlan_OperationType(t *testing.T) {
	tests := []struct {
		command string
		wantOp  string
	}{
		{"globular services desired set ldap 0.0.1", "install"},
		{"globular services desired remove ldap", "remove"},
		{"globular pkg publish echo", "publish"},
		{"globular pkg install ldap 0.0.1", "install"},
	}
	for _, tt := range tests {
		plan := NormalizePlan(tt.command, nil)
		if plan.Operation != tt.wantOp {
			t.Errorf("NormalizePlan(%q).Operation = %q, want %q", tt.command, plan.Operation, tt.wantOp)
		}
	}
}

// ── Remediation Plan Tests (Phase 7D) ────────────────────────────────────────

func TestBuildRemovalPlan_LeafService(t *testing.T) {
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"torrent":     nil,
			"persistence": nil,
		},
		ports: map[string][]int{},
	}

	plan := BuildRemovalPlan("torrent", []string{"torrent", "persistence"}, deps)

	if plan.Status != "ready" {
		t.Fatalf("expected ready, got %s", plan.Status)
	}
	if len(plan.OrderedSteps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.OrderedSteps))
	}
	if plan.OrderedSteps[0].Target != "torrent" {
		t.Fatalf("expected target torrent, got %s", plan.OrderedSteps[0].Target)
	}
}

func TestBuildRemovalPlan_SingleDependent(t *testing.T) {
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"persistence": nil,
			"ldap":        {{Name: "persistence", Required: true}},
		},
		ports: map[string][]int{},
	}

	plan := BuildRemovalPlan("persistence", []string{"persistence", "ldap"}, deps)

	if plan.Status != "ready" {
		t.Fatalf("expected ready, got %s", plan.Status)
	}
	if len(plan.OrderedSteps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.OrderedSteps))
	}
	// ldap must come before persistence
	if plan.OrderedSteps[0].Target != "ldap" {
		t.Fatalf("expected ldap first, got %s", plan.OrderedSteps[0].Target)
	}
	if plan.OrderedSteps[1].Target != "persistence" {
		t.Fatalf("expected persistence last, got %s", plan.OrderedSteps[1].Target)
	}
}

func TestBuildRemovalPlan_MultiLevelChain(t *testing.T) {
	// blog → file → persistence
	// ldap → persistence
	// search → persistence
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"persistence": nil,
			"file":        {{Name: "persistence", Required: true}},
			"ldap":        {{Name: "persistence", Required: true}},
			"search":      {{Name: "persistence", Required: true}},
			"blog":        {{Name: "file", Required: true}},
		},
		ports: map[string][]int{},
	}

	installed := []string{"persistence", "file", "ldap", "search", "blog"}
	plan := BuildRemovalPlan("persistence", installed, deps)

	if plan.Status != "ready" {
		t.Fatalf("expected ready, got %s", plan.Status)
	}

	// persistence must be last
	last := plan.OrderedSteps[len(plan.OrderedSteps)-1]
	if last.Target != "persistence" {
		t.Fatalf("expected persistence last, got %s", last.Target)
	}

	// blog must come before file (blog depends on file)
	blogIdx, fileIdx := -1, -1
	for i, s := range plan.OrderedSteps {
		if s.Target == "blog" {
			blogIdx = i
		}
		if s.Target == "file" {
			fileIdx = i
		}
	}
	if blogIdx > fileIdx {
		t.Fatalf("blog (idx %d) must come before file (idx %d)", blogIdx, fileIdx)
	}

	// file must come before persistence
	persIdx := len(plan.OrderedSteps) - 1
	if fileIdx > persIdx {
		t.Fatalf("file (idx %d) must come before persistence (idx %d)", fileIdx, persIdx)
	}
}

func TestBuildRemovalPlan_CycleDetection(t *testing.T) {
	// A depends on B, B depends on A → cycle
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"A": {{Name: "B", Required: true}},
			"B": {{Name: "A", Required: true}},
		},
		ports: map[string][]int{},
	}

	plan := BuildRemovalPlan("A", []string{"A", "B"}, deps)

	if plan.Status != "blocked" {
		t.Fatalf("expected blocked, got %s", plan.Status)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected cycle warning")
	}
}

func TestBuildRemovalPlan_MultipleLeaves(t *testing.T) {
	// blog, search, mail all depend on persistence (all leaves)
	deps := &mockDeps{
		deps: map[string][]ServiceDependency{
			"persistence": nil,
			"blog":        {{Name: "persistence", Required: true}},
			"search":      {{Name: "persistence", Required: true}},
			"mail":        {{Name: "persistence", Required: true}},
		},
		ports: map[string][]int{},
	}

	installed := []string{"persistence", "blog", "search", "mail"}
	plan := BuildRemovalPlan("persistence", installed, deps)

	if plan.Status != "ready" {
		t.Fatalf("expected ready, got %s", plan.Status)
	}
	if len(plan.OrderedSteps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(plan.OrderedSteps))
	}

	// Leaves should be sorted alphabetically (deterministic)
	if plan.OrderedSteps[0].Target != "blog" {
		t.Fatalf("expected blog first (alphabetical), got %s", plan.OrderedSteps[0].Target)
	}
	if plan.OrderedSteps[1].Target != "mail" {
		t.Fatalf("expected mail second, got %s", plan.OrderedSteps[1].Target)
	}
	if plan.OrderedSteps[2].Target != "search" {
		t.Fatalf("expected search third, got %s", plan.OrderedSteps[2].Target)
	}
	// persistence must be last
	if plan.OrderedSteps[3].Target != "persistence" {
		t.Fatalf("expected persistence last, got %s", plan.OrderedSteps[3].Target)
	}
}

// ── Remediation Execution Tests (Phase 7E) ───────────────────────────────────

func TestExecuteWorkflow_DryRun(t *testing.T) {
	plan := &RemediationPlan{
		TargetOperation: "remove persistence",
		Status:          "ready",
		OrderedSteps: []RemediationStep{
			{Order: 1, Action: "remove", Target: "blog", Reason: "leaf"},
			{Order: 2, Action: "remove", Target: "persistence", Reason: "target"},
		},
	}

	executor := NewRemediationExecutor(nil, false)
	wf := executor.Execute(context.Background(), plan, true, false)

	if wf.Status != "completed" {
		t.Fatalf("expected completed, got %s (error: %s)", wf.Status, wf.Error)
	}
	if len(wf.StepResults) != 2 {
		t.Fatalf("expected 2 results, got %d", len(wf.StepResults))
	}
	for _, sr := range wf.StepResults {
		if !sr.DryRun {
			t.Fatal("expected dry_run on all steps")
		}
		if !sr.Success {
			t.Fatalf("expected success for step %d, got error: %s", sr.StepOrder, sr.Error)
		}
	}
}

func TestExecuteWorkflow_BlockedPlan(t *testing.T) {
	plan := &RemediationPlan{
		TargetOperation: "remove A",
		Status:          "blocked",
		Reason:          "dependency cycle",
	}

	executor := NewRemediationExecutor(nil, false)
	wf := executor.Execute(context.Background(), plan, true, false)

	if wf.Status != "failed" {
		t.Fatalf("expected failed, got %s", wf.Status)
	}
	if wf.Error == "" {
		t.Fatal("expected error message for blocked plan")
	}
}

func TestExecuteWorkflow_ReadOnlyBlocks(t *testing.T) {
	plan := &RemediationPlan{
		TargetOperation: "remove torrent",
		Status:          "ready",
		OrderedSteps: []RemediationStep{
			{Order: 1, Action: "remove", Target: "torrent", Reason: "no dependents"},
		},
	}

	// readOnly=true, dryRun=false → should fail
	executor := NewRemediationExecutor(nil, true)
	wf := executor.Execute(context.Background(), plan, false, true)

	if wf.Status != "failed" {
		t.Fatalf("expected failed, got %s", wf.Status)
	}
	if wf.StepResults[0].Error == "" {
		t.Fatal("expected read-only error")
	}
}

func TestExecuteWorkflow_EmptyPlan(t *testing.T) {
	plan := &RemediationPlan{
		TargetOperation: "remove nothing",
		Status:          "ready",
		OrderedSteps:    nil,
	}

	executor := NewRemediationExecutor(nil, false)
	wf := executor.Execute(context.Background(), plan, true, false)

	if wf.Status != "completed" {
		t.Fatalf("expected completed, got %s", wf.Status)
	}
	if len(wf.StepResults) != 0 {
		t.Fatalf("expected 0 results, got %d", len(wf.StepResults))
	}
}

func TestBuildStepCommand(t *testing.T) {
	tests := []struct {
		step RemediationStep
		want string
	}{
		{RemediationStep{Action: "remove", Target: "ldap"}, "globular services desired remove ldap"},
		{RemediationStep{Action: "install", Target: "ldap"}, "globular services desired set ldap"},
		{RemediationStep{Action: "disable", Target: "ldap"}, "globular services disable ldap"},
	}
	for _, tt := range tests {
		got := buildStepCommand(tt.step)
		if got != tt.want {
			t.Errorf("buildStepCommand(%s %s) = %q, want %q", tt.step.Action, tt.step.Target, got, tt.want)
		}
	}
}

func TestStaticDependencySource(t *testing.T) {
	src := NewStaticDependencySource()

	// ldap should have dependencies
	deps := src.Dependencies("ldap")
	if len(deps) == 0 {
		t.Fatal("expected ldap to have dependencies")
	}

	// Check persistence is required
	found := false
	for _, d := range deps {
		if d.Name == "persistence" && d.Required {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ldap to require persistence")
	}

	// event should have ports
	ports := src.DefaultPorts("event")
	if len(ports) != 1 || ports[0] != 10102 {
		t.Fatalf("expected event port 10102, got %v", ports)
	}

	// unknown service → nil
	if deps := src.Dependencies("nonexistent"); deps != nil {
		t.Fatalf("expected nil deps for unknown service, got %v", deps)
	}
}

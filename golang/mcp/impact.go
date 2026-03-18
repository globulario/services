package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Core Types ───────────────────────────────────────────────────────────────

// OperationPlan is a normalized, structured representation of an intended action.
type OperationPlan struct {
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	TargetService string   `json:"target_service"`
	TargetVersion string   `json:"target_version,omitempty"`
	TargetNodes   []string `json:"target_nodes,omitempty"`
	Operation     string   `json:"operation,omitempty"` // "install", "remove", "publish", ""
	RequestedBy   string   `json:"requested_by,omitempty"`
	Timestamp     string   `json:"timestamp"`
}

// ImpactReport describes the predicted effects of applying an operation plan.
type ImpactReport struct {
	DependenciesRequired []string `json:"dependencies_required,omitempty"`
	DependenciesMissing  []string `json:"dependencies_missing,omitempty"`
	TransitiveDeps       []string `json:"transitive_deps,omitempty"`       // full transitive closure
	DependencySource     string   `json:"dependency_source,omitempty"`     // "descriptor" or "static"
	DependencyCycle      bool     `json:"dependency_cycle,omitempty"`      // true if cycle detected
	ReverseDependents    []string         `json:"reverse_dependents,omitempty"`   // services that depend on target
	RemediationPlan      *RemediationPlan `json:"remediation_plan,omitempty"`     // ordered safe removal plan
	ServicesAffected     []string         `json:"services_affected,omitempty"`
	PortsRequired        []int    `json:"ports_required,omitempty"`
	PortConflicts        []int    `json:"port_conflicts,omitempty"`
	PackageFound         bool     `json:"package_found"`
	PackageStatus        string   `json:"package_status,omitempty"` // "published", "staging", "not_found", "check_unavailable"
	RiskLevel            string   `json:"risk_level"`               // "low", "medium", "high"
	Warnings             []string `json:"warnings,omitempty"`
}

// AdmissionDecision is the final decision returned by the planner.
type AdmissionDecision struct {
	Status               string   `json:"status"` // "allow", "allow_with_approval", "block", "requires_remediation"
	Reasons              []string `json:"reasons,omitempty"`
	MissingRequirements  []string `json:"missing_requirements,omitempty"`
	SuggestedRemediation []string `json:"suggested_remediation,omitempty"`
	RequiresApproval     bool     `json:"requires_approval,omitempty"`
}

// ── Planner ──────────────────────────────────────────────────────────────────

// Planner evaluates an operation plan against the current cluster state
// and produces an impact report with an admission decision.
type Planner struct {
	deps       DependencySource
	descriptor DescriptorLookup // optional, nil = skip descriptor lookup
	clients    *clientPool
}

// NewPlanner creates a Planner with the given dependency source and client pool.
func NewPlanner(deps DependencySource, clients *clientPool) *Planner {
	return &Planner{deps: deps, clients: clients}
}

// NewPlannerWithDescriptor creates a Planner that tries descriptor-backed
// dependency lookup first, falling back to the static DependencySource.
func NewPlannerWithDescriptor(deps DependencySource, desc DescriptorLookup, clients *clientPool) *Planner {
	return &Planner{deps: deps, descriptor: desc, clients: clients}
}

// Evaluate runs the full admission pipeline for an operation plan.
func (p *Planner) Evaluate(ctx context.Context, plan OperationPlan) (*ImpactReport, *AdmissionDecision) {
	report := &ImpactReport{
		RiskLevel: "low",
	}
	decision := &AdmissionDecision{
		Status: "allow",
	}

	// Step 1: Check package existence in repository
	p.checkPackageExists(ctx, plan, report)

	// Step 2: Check dependency presence against installed cluster state
	p.checkDependencies(ctx, plan, report)

	// Step 3: Check port conflicts using installed services + static port map
	p.checkPortConflicts(ctx, plan, report)

	// Step 4: For remove operations, check reverse dependencies
	p.checkReverseDeps(ctx, plan, report)

	// Step 5: Check approval/policy using existing approval logic
	p.checkPolicy(plan, report)

	// Step 5: Compute admission decision from impact report
	computeDecision(report, decision)

	return report, decision
}

// ── Pipeline Steps ───────────────────────────────────────────────────────────

func (p *Planner) checkPackageExists(ctx context.Context, plan OperationPlan, report *ImpactReport) {
	if plan.TargetService == "" {
		return
	}

	if p.clients == nil {
		report.PackageStatus = "check_unavailable"
		report.Warnings = append(report.Warnings, "repository check unavailable: no client pool")
		return
	}

	queryCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
	defer cancel()

	conn, err := p.clients.get(queryCtx, repositoryEndpoint())
	if err != nil {
		report.PackageStatus = "check_unavailable"
		report.Warnings = append(report.Warnings, fmt.Sprintf("repository unavailable: %v", err))
		return
	}

	client := repositorypb.NewPackageRepositoryClient(conn)
	resp, err := client.SearchArtifacts(queryCtx, &repositorypb.SearchArtifactsRequest{
		Query:    plan.TargetService,
		PageSize: 10,
	})
	if err != nil {
		report.PackageStatus = "check_unavailable"
		report.Warnings = append(report.Warnings, fmt.Sprintf("repository query failed: %v", err))
		return
	}

	// Look for an exact name match in the results
	for _, artifact := range resp.GetArtifacts() {
		ref := artifact.GetRef()
		if ref != nil && ref.GetName() == plan.TargetService {
			report.PackageFound = true
			report.PackageStatus = "found"
			return
		}
	}

	report.PackageFound = false
	report.PackageStatus = "not_found"
}

func (p *Planner) checkDependencies(ctx context.Context, plan OperationPlan, report *ImpactReport) {
	if plan.TargetService == "" || plan.Operation == "remove" {
		return
	}

	// Try descriptor-first lookup with transitive resolution
	if p.descriptor != nil {
		desc, err := p.descriptor.Descriptor(ctx, plan.TargetService)
		if err == nil && desc != nil && len(desc.Requires) > 0 {
			report.DependencySource = "descriptor"
			report.DependenciesRequired = desc.Requires

			// Resolve transitive dependencies via BFS
			allDeps, cycle := ResolveTransitiveDeps(plan.TargetService, func(svc string) (*ServiceDescriptor, error) {
				return p.descriptor.Descriptor(ctx, svc)
			})
			if len(allDeps) > 0 {
				report.TransitiveDeps = allDeps
			}
			if cycle {
				report.DependencyCycle = true
				report.Warnings = append(report.Warnings, "dependency cycle detected in transitive graph")
			}

			// Check all transitive deps against installed services
			installed := p.getInstalledServices(ctx)
			installedSet := make(map[string]bool, len(installed))
			for _, svc := range installed {
				installedSet[svc] = true
			}

			// Check direct required deps
			for _, req := range desc.Requires {
				if !installedSet[req] {
					report.DependenciesMissing = append(report.DependenciesMissing, req)
				}
			}
			// Also check transitive deps
			for _, req := range allDeps {
				if !installedSet[req] && !containsStr(report.DependenciesMissing, req) {
					report.DependenciesMissing = append(report.DependenciesMissing, req)
				}
			}

			if len(report.DependenciesMissing) > 0 {
				report.RiskLevel = "high"
			}
			return
		}
		// Descriptor lookup failed or empty — fall through to static
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("descriptor lookup failed, using static fallback: %v", err))
		}
	}

	// Fallback: static dependency source
	report.DependencySource = "static"
	deps := p.deps.Dependencies(plan.TargetService)
	if len(deps) == 0 {
		return
	}

	for _, d := range deps {
		report.DependenciesRequired = append(report.DependenciesRequired, d.Name)
	}

	installed := p.getInstalledServices(ctx)
	installedSet := make(map[string]bool, len(installed))
	for _, svc := range installed {
		installedSet[svc] = true
	}

	for _, d := range deps {
		if !d.Required {
			continue
		}
		if !installedSet[d.Name] {
			report.DependenciesMissing = append(report.DependenciesMissing, d.Name)
		}
	}

	if len(report.DependenciesMissing) > 0 {
		report.RiskLevel = "high"
	}
}

func (p *Planner) checkPortConflicts(ctx context.Context, plan OperationPlan, report *ImpactReport) {
	if plan.TargetService == "" || plan.Operation == "remove" {
		return
	}

	needed := p.deps.DefaultPorts(plan.TargetService)
	if len(needed) == 0 {
		return
	}
	report.PortsRequired = needed

	// Derive ports in use from installed services + static port map.
	// The node agent Inventory doesn't expose port numbers directly,
	// so we map installed service names → known ports via DependencySource.
	installed := p.getInstalledServices(ctx)
	if installed == nil {
		report.Warnings = append(report.Warnings, "port conflict check unavailable: could not query installed services")
		return
	}

	usedSet := make(map[int]bool)
	for _, svc := range installed {
		for _, port := range p.deps.DefaultPorts(svc) {
			usedSet[port] = true
		}
	}

	for _, port := range needed {
		if usedSet[port] {
			report.PortConflicts = append(report.PortConflicts, port)
		}
	}

	if len(report.PortConflicts) > 0 {
		report.RiskLevel = "high"
	}
}

func (p *Planner) checkReverseDeps(ctx context.Context, plan OperationPlan, report *ImpactReport) {
	if plan.Operation != "remove" || plan.TargetService == "" {
		return
	}

	installed := p.getInstalledServices(ctx)
	if installed == nil {
		report.Warnings = append(report.Warnings, "reverse dependency check unavailable: could not query installed services")
		return
	}

	dependents := p.deps.ReverseDeps(plan.TargetService, installed)
	if len(dependents) > 0 {
		report.ReverseDependents = dependents
		report.RiskLevel = "high"
	}

	// Compute ordered remediation plan
	report.RemediationPlan = BuildRemovalPlan(plan.TargetService, installed, p.deps)
}

func (p *Planner) checkPolicy(plan OperationPlan, report *ImpactReport) {
	cmdPath := extractCommandPath(plan.Command, plan.Args)
	if approval := CheckApproval(cmdPath); approval != nil && approval.RequiresUserConfirmation {
		report.ServicesAffected = append(report.ServicesAffected, plan.TargetService)
		if report.RiskLevel == "low" {
			report.RiskLevel = "medium"
		}
	}
}

// ── Decision Computation ─────────────────────────────────────────────────────

func computeDecision(report *ImpactReport, decision *AdmissionDecision) {
	// Rule 1: Missing required dependencies → requires_remediation
	if len(report.DependenciesMissing) > 0 {
		decision.Status = "requires_remediation"
		decision.MissingRequirements = report.DependenciesMissing
		for _, dep := range report.DependenciesMissing {
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("required dependency %q is not installed", dep))
			decision.SuggestedRemediation = append(decision.SuggestedRemediation, fmt.Sprintf("install %s first", dep))
		}
		return
	}

	// Rule 2: Reverse dependents on remove → block with remediation
	if len(report.ReverseDependents) > 0 {
		decision.Status = "requires_remediation"
		decision.MissingRequirements = report.ReverseDependents
		for _, dep := range report.ReverseDependents {
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("service %q depends on this and is still installed", dep))
			decision.SuggestedRemediation = append(decision.SuggestedRemediation, fmt.Sprintf("remove %s first", dep))
		}
		return
	}

	// Rule 3: Port conflicts → block
	if len(report.PortConflicts) > 0 {
		decision.Status = "block"
		for _, port := range report.PortConflicts {
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("port %d is already in use", port))
		}
		return
	}

	// Rule 3: Package not found → block
	if report.PackageStatus == "not_found" {
		decision.Status = "block"
		decision.Reasons = append(decision.Reasons, "package not found in repository")
		decision.SuggestedRemediation = append(decision.SuggestedRemediation, "publish the package first: globular pkg publish")
		return
	}

	// Rule 4: Policy/approval required → allow_with_approval
	if len(report.ServicesAffected) > 0 {
		decision.Status = "allow_with_approval"
		decision.RequiresApproval = true
		decision.Reasons = append(decision.Reasons, "operation requires approval per policy")
		return
	}

	// Default: allow
	decision.Status = "allow"
}

// ── Cluster State Helpers ────────────────────────────────────────────────────

// getInstalledServices queries the node agent for currently installed service names.
func (p *Planner) getInstalledServices(ctx context.Context) []string {
	if p.clients == nil {
		return nil
	}

	queryCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
	defer cancel()

	conn, err := p.clients.get(queryCtx, nodeAgentEndpoint())
	if err != nil {
		return nil
	}

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	resp, err := client.ListInstalledPackages(queryCtx, &node_agentpb.ListInstalledPackagesRequest{})
	if err != nil {
		return nil
	}

	var services []string
	for _, pkg := range resp.GetPackages() {
		name := pkg.GetName()
		if name != "" {
			services = append(services, name)
		}
	}
	return services
}

// ── Plan Normalization ───────────────────────────────────────────────────────

// NormalizePlan parses a CLI command string into a structured OperationPlan.
func NormalizePlan(command string, args []string) OperationPlan {
	plan := OperationPlan{
		Command:   command,
		Args:      args,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Parse target service and version from common command patterns.
	allTokens := strings.Fields(command)
	allTokens = append(allTokens, args...)

	// Strip "globular" prefix
	if len(allTokens) > 0 && (allTokens[0] == "globular" || allTokens[0] == "globular-cli") {
		allTokens = allTokens[1:]
	}

	// Filter out flags to get positional args
	var positional []string
	for _, t := range allTokens {
		if strings.HasPrefix(t, "-") {
			break
		}
		positional = append(positional, t)
	}

	// Pattern: "services desired set <service> <version>"
	if len(positional) >= 4 && positional[0] == "services" && positional[1] == "desired" && positional[2] == "set" {
		plan.TargetService = positional[3]
		plan.Operation = "install"
		if len(positional) >= 5 {
			plan.TargetVersion = positional[4]
		}
	}

	// Pattern: "services desired remove <service>"
	if len(positional) >= 4 && positional[0] == "services" && positional[1] == "desired" && positional[2] == "remove" {
		plan.TargetService = positional[3]
		plan.Operation = "remove"
	}

	// Pattern: "pkg install <service>"
	if len(positional) >= 3 && positional[0] == "pkg" && positional[1] == "install" {
		plan.TargetService = positional[2]
		plan.Operation = "install"
		if len(positional) >= 4 {
			plan.TargetVersion = positional[3]
		}
	}

	// Pattern: "pkg publish <service>"
	if len(positional) >= 3 && positional[0] == "pkg" && positional[1] == "publish" {
		plan.TargetService = positional[2]
		plan.Operation = "publish"
	}

	return plan
}

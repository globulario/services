package main

// artifact_law_validator.go — Artifact law enforcement (PR 3).
//
// ArtifactLawValidator applies named, testable rules to an incoming manifest
// before it can be promoted to PUBLISHED. Each rule corresponds to one of the
// formal invariants defined in the repository lifecycle model.
//
// Rules:
//
//   INV_C_NO_DEP_ON_APPLICATION
//     No artifact may list an APPLICATION kind in its hard_deps.
//     APPLICATION packages are leaf consumers — nothing in the cluster depends
//     on them. If you hard-dep on an application, you have inverted the graph.
//
//   LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS
//     COMMAND artifacts must not declare cluster runtime_uses.
//     Commands are standalone binaries. They have no service graph relationships.
//     Packaging-time or local OS requirements are fine, but cluster service deps
//     would create invisible operational coupling with no enforcement mechanism.
//
//   INV_D_HARD_DEPS_ACYCLIC
//     The hard_deps graph (across all PUBLISHED artifacts + the incoming one)
//     must be acyclic. A cycle means two packages each require the other to be
//     installed first — an install deadlock.
//
// The validator is intentionally stateless and pure: given a manifest + catalog
// snapshot, it returns a (possibly empty) slice of violations. The caller
// decides what to do with them (block promotion, warn, record in workflow).

import (
	"fmt"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ArtifactViolation describes a single law violation found during validation.
type ArtifactViolation struct {
	Rule     string // rule identifier, e.g. "INV_C_NO_DEP_ON_APPLICATION"
	Artifact string // human key of the violating artifact, e.g. "core@globular.io/etcd@1.2.3"
	Detail   string // human-readable explanation
}

func (v ArtifactViolation) Error() string {
	return fmt.Sprintf("[%s] %s: %s", v.Rule, v.Artifact, v.Detail)
}

// ArtifactLawValidator validates a single manifest against the formal artifact laws.
type ArtifactLawValidator struct {
	// incoming is the manifest about to be promoted.
	incoming *repopb.ArtifactManifest
	// catalog is a snapshot of all currently-PUBLISHED manifests, excluding the incoming one.
	// Used for cross-artifact rules (dep kind lookup, cycle detection).
	catalog []*repopb.ArtifactManifest
}

// NewArtifactLawValidator constructs a validator.
func NewArtifactLawValidator(incoming *repopb.ArtifactManifest, catalog []*repopb.ArtifactManifest) *ArtifactLawValidator {
	return &ArtifactLawValidator{incoming: incoming, catalog: catalog}
}

// Validate runs all rules and returns all violations found. An empty slice means the manifest is valid.
func (v *ArtifactLawValidator) Validate() []ArtifactViolation {
	var violations []ArtifactViolation
	violations = append(violations, v.ruleNoDepOnApplication()...)
	violations = append(violations, v.ruleCommandNoClusterRuntimeDeps()...)
	violations = append(violations, v.ruleHardDepsAcyclic()...)
	return violations
}

// artifactLabel returns a human-readable identifier for a manifest.
func artifactLabel(m *repopb.ArtifactManifest) string {
	ref := m.GetRef()
	return fmt.Sprintf("%s/%s@%s", ref.GetPublisherId(), ref.GetName(), ref.GetVersion())
}

// ── Rule: INV_C_NO_DEP_ON_APPLICATION ─────────────────────────────────────

// ruleNoDepOnApplication enforces Invariant C:
// no artifact's hard_deps may reference an APPLICATION kind artifact.
func (v *ArtifactLawValidator) ruleNoDepOnApplication() []ArtifactViolation {
	const rule = "INV_C_NO_DEP_ON_APPLICATION"
	if len(v.incoming.GetHardDeps()) == 0 {
		return nil
	}

	// Build a lookup: canonical name → kind, from the catalog.
	kindByName := make(map[string]repopb.ArtifactKind, len(v.catalog))
	for _, m := range v.catalog {
		kindByName[canonicalName(m.GetRef().GetName())] = m.GetRef().GetKind()
	}

	var violations []ArtifactViolation
	label := artifactLabel(v.incoming)
	for _, dep := range v.incoming.GetHardDeps() {
		depName := canonicalName(dep.GetName())
		if kind, ok := kindByName[depName]; ok && kind == repopb.ArtifactKind_APPLICATION {
			violations = append(violations, ArtifactViolation{
				Rule:     rule,
				Artifact: label,
				Detail: fmt.Sprintf(
					"hard_dep %q is an APPLICATION — applications are leaf consumers; nothing may depend on them",
					dep.GetName()),
			})
		}
	}
	return violations
}

// ── Rule: LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS ──────────────────────────────

// ruleCommandNoClusterRuntimeDeps enforces the COMMAND isolation law:
// COMMAND artifacts must not declare runtime_uses (cluster service deps).
func (v *ArtifactLawValidator) ruleCommandNoClusterRuntimeDeps() []ArtifactViolation {
	const rule = "LAW_COMMAND_NO_CLUSTER_RUNTIME_DEPS"
	if v.incoming.GetRef().GetKind() != repopb.ArtifactKind_COMMAND {
		return nil
	}
	if len(v.incoming.GetRuntimeUses()) == 0 {
		return nil
	}
	return []ArtifactViolation{{
		Rule:     rule,
		Artifact: artifactLabel(v.incoming),
		Detail: fmt.Sprintf(
			"COMMAND artifact declares cluster runtime_uses %v — commands are standalone binaries with no service graph relationships",
			v.incoming.GetRuntimeUses()),
	}}
}

// ── Rule: INV_D_HARD_DEPS_ACYCLIC ─────────────────────────────────────────

// ruleHardDepsAcyclic enforces Invariant D: the hard_deps graph must be a DAG.
//
// Algorithm: build an adjacency list from (catalog + incoming), then run DFS
// from every node. A back edge in DFS indicates a cycle.
func (v *ArtifactLawValidator) ruleHardDepsAcyclic() []ArtifactViolation {
	const rule = "INV_D_HARD_DEPS_ACYCLIC"

	// Combine incoming with catalog for a complete graph snapshot.
	all := make([]*repopb.ArtifactManifest, 0, len(v.catalog)+1)
	all = append(all, v.catalog...)
	all = append(all, v.incoming)

	// Build adjacency list: canonical name → set of dep canonical names.
	// We use canonical name as the node key (ignoring version for cycle purposes —
	// version-pinned cross-version deps are structurally identical for cycle detection).
	adj := make(map[string][]string, len(all))
	for _, m := range all {
		name := canonicalName(m.GetRef().GetName())
		if _, exists := adj[name]; !exists {
			adj[name] = nil
		}
		for _, dep := range m.GetHardDeps() {
			depName := canonicalName(dep.GetName())
			adj[name] = append(adj[name], depName)
		}
	}

	// DFS cycle detection.
	const (
		unvisited = 0
		inStack   = 1
		done      = 2
	)
	color := make(map[string]int, len(adj))
	var cycles []string

	var dfs func(node string, path []string)
	dfs = func(node string, path []string) {
		if color[node] == done {
			return
		}
		if color[node] == inStack {
			// Found a cycle — record the cycle path.
			cycleStart := -1
			for i, n := range path {
				if n == node {
					cycleStart = i
					break
				}
			}
			cyclePath := path
			if cycleStart >= 0 {
				cyclePath = path[cycleStart:]
			}
			cycles = append(cycles, strings.Join(append(cyclePath, node), " → "))
			return
		}
		color[node] = inStack
		for _, neighbor := range adj[node] {
			dfs(neighbor, append(path, node))
		}
		color[node] = done
	}

	for node := range adj {
		if color[node] == unvisited {
			dfs(node, nil)
		}
	}

	if len(cycles) == 0 {
		return nil
	}

	var violations []ArtifactViolation
	for _, cycle := range cycles {
		violations = append(violations, ArtifactViolation{
			Rule:     rule,
			Artifact: artifactLabel(v.incoming),
			Detail:   fmt.Sprintf("hard_deps cycle detected: %s", cycle),
		})
	}
	return violations
}

// canonicalName lower-cases and normalises a package name for graph key use.
func canonicalName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

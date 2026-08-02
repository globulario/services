// Package definitions contains regression tests that validate the workflow
// definition YAML files against the codebase's canonical authorities.
//
// nodejoin_profile_gating_test.go guards the Tier-4 profile-gating contract of
// node.join.yaml (scar 2026-07-10): a hardcoded, non-profile-gated 24-package
// workload list installed ~10 profile-orphan packages on every joining node
// and pushed the serial install past its timeout, failing the whole join on
// the final package. Tier 4 is now split into steps whose `when` gates mirror
// component_catalog.ProfilePackages — the single placement authority. These
// tests fail if the YAML and the catalog drift apart.
package definitions

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/globulario/services/golang/component_catalog"
	"gopkg.in/yaml.v3"
)

// workloadStepPrefix identifies the Tier-4 workload install steps.
const workloadStepPrefix = "install_workloads"

// allTier4Workloads freezes the full workload coverage of the join flow: the
// union of all Tier-4 step package lists must equal exactly this set, so a
// regrouping can never silently drop (or duplicate) a workload. When a
// workload service is added to or removed from the platform, update the
// catalog (profilemap.go), node.join.yaml, and this list together.
var allTier4Workloads = []string{
	"ai-executor", "ai-memory", "ai-router", "ai-watcher",
	"backup-manager", "blog", "catalog", "cluster-controller",
	"cluster-doctor", "conversation", "echo", "file", "ldap", "log",
	"mail", "mcp", "media", "persistence", "search", "sql", "storage",
	"title", "torrent", "workflow",
}

type defFile struct {
	Spec struct {
		Steps []defStep `yaml:"steps"`
	} `yaml:"spec"`
}

type defStep struct {
	ID   string `yaml:"id"`
	When *struct {
		AnyOf []struct {
			Expr string `yaml:"expr"`
		} `yaml:"anyOf"`
	} `yaml:"when"`
	With struct {
		Packages []defPackage `yaml:"packages"`
	} `yaml:"with"`
	Verification struct {
		With struct {
			Packages []defPackage `yaml:"packages"`
		} `yaml:"with"`
	} `yaml:"verification"`
}

type defPackage struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"`
}

var gateExprRe = regexp.MustCompile(`^contains\(inputs\.node_profiles,\s*'([^']+)'\)$`)

func loadNodeJoin(t *testing.T) []defStep {
	t.Helper()
	data, err := os.ReadFile("node.join.yaml")
	if err != nil {
		t.Fatalf("reading node.join.yaml: %v", err)
	}
	var def defFile
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("parsing node.join.yaml: %v", err)
	}
	if len(def.Spec.Steps) == 0 {
		t.Fatal("node.join.yaml has no steps — parse failure or file moved")
	}
	return def.Spec.Steps
}

func workloadSteps(t *testing.T) []defStep {
	t.Helper()
	var out []defStep
	for _, s := range loadNodeJoin(t) {
		if strings.HasPrefix(s.ID, workloadStepPrefix) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %s* steps found in node.join.yaml", workloadStepPrefix)
	}
	return out
}

// gateProfiles extracts the profile names from a step's when.anyOf gate.
func gateProfiles(t *testing.T, s defStep) []string {
	t.Helper()
	if s.When == nil || len(s.When.AnyOf) == 0 {
		t.Fatalf("step %s: Tier-4 workload steps MUST carry a when.anyOf profile gate", s.ID)
	}
	var profiles []string
	for _, e := range s.When.AnyOf {
		m := gateExprRe.FindStringSubmatch(strings.TrimSpace(e.Expr))
		if m == nil {
			t.Fatalf("step %s: unrecognized gate expr %q — expected contains(inputs.node_profiles, '<profile>')", s.ID, e.Expr)
		}
		profiles = append(profiles, m[1])
	}
	return profiles
}

// TestNodeJoin_WorkloadStepsGateMatchesCatalog verifies, for every Tier-4
// step, that its when-gate profile set EQUALS the catalog profile set of
// every package it installs (component_catalog.ProfilesForPackage). Equality
// is required in both directions:
//   - gate ⊆ package profiles: the step never fires on a node whose profiles
//     don't claim the package (no profile-orphan installs);
//   - gate ⊇ package profiles: no node entitled to the package misses it
//     because the gate is narrower than the catalog.
func TestNodeJoin_WorkloadStepsGateMatchesCatalog(t *testing.T) {
	for _, s := range workloadSteps(t) {
		gate := append([]string(nil), gateProfiles(t, s)...)
		sort.Strings(gate)

		if len(s.With.Packages) == 0 {
			t.Errorf("step %s: no packages", s.ID)
			continue
		}
		for _, p := range s.With.Packages {
			if p.Kind != "SERVICE" {
				t.Errorf("step %s: package %s has kind %s — Tier-4 installs SERVICE workloads only", s.ID, p.Name, p.Kind)
			}
			want := component_catalog.ProfilesForPackage(p.Name)
			if len(want) == 0 {
				t.Errorf("step %s: package %s is not in component_catalog.ProfilePackages — unknown package or catalog drift", s.ID, p.Name)
				continue
			}
			if strings.Join(gate, ",") != strings.Join(want, ",") {
				t.Errorf("step %s: gate profiles %v != catalog profiles %v for package %s\n"+
					"regroup the package into a step whose gate matches its catalog profile set",
					s.ID, gate, want, p.Name)
			}
		}
	}
}

// TestNodeJoin_WorkloadCoverageCompleteAndDisjoint verifies the union of all
// Tier-4 package lists equals the frozen workload set exactly, and that no
// package appears in two steps (parallel steps installing the same package
// would race on the same node-agent).
func TestNodeJoin_WorkloadCoverageCompleteAndDisjoint(t *testing.T) {
	seen := map[string]string{} // package → step id
	for _, s := range workloadSteps(t) {
		for _, p := range s.With.Packages {
			if prev, dup := seen[p.Name]; dup {
				t.Errorf("package %s appears in both %s and %s — Tier-4 steps run in parallel and must be disjoint", p.Name, prev, s.ID)
			}
			seen[p.Name] = s.ID
		}
	}
	var got []string
	for name := range seen {
		got = append(got, name)
	}
	sort.Strings(got)

	want := append([]string(nil), allTier4Workloads...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Tier-4 workload coverage drifted\n got: %v\nwant: %v\n"+
			"update node.join.yaml and allTier4Workloads together", got, want)
	}
}

// TestNodeJoin_VerificationMirrorsInstallList verifies each Tier-4 step's
// verification package list is identical to its install list — verifying a
// different set than was installed makes the receipt meaningless.
func TestNodeJoin_VerificationMirrorsInstallList(t *testing.T) {
	for _, s := range workloadSteps(t) {
		install := make([]string, 0, len(s.With.Packages))
		for _, p := range s.With.Packages {
			install = append(install, p.Name+"/"+p.Kind)
		}
		verify := make([]string, 0, len(s.Verification.With.Packages))
		for _, p := range s.Verification.With.Packages {
			verify = append(verify, p.Name+"/"+p.Kind)
		}
		sort.Strings(install)
		sort.Strings(verify)
		if strings.Join(install, ",") != strings.Join(verify, ",") {
			t.Errorf("step %s: verification packages %v != install packages %v", s.ID, verify, install)
		}
	}
}

// TestNodeJoin_ReportInstalledDependsOnAllWorkloadSteps verifies the final
// report_installed step waits on every Tier-4 step, so installed-state sync
// cannot run before all profile groups have settled (skipped steps satisfy
// dependencies — engine treats SKIPPED as terminal).
func TestNodeJoin_ReportInstalledDependsOnAllWorkloadSteps(t *testing.T) {
	steps := loadNodeJoin(t)

	var raw struct {
		Spec struct {
			Steps []struct {
				ID        string   `yaml:"id"`
				DependsOn []string `yaml:"dependsOn"`
			} `yaml:"steps"`
		} `yaml:"spec"`
	}
	data, err := os.ReadFile("node.join.yaml")
	if err != nil {
		t.Fatalf("reading node.join.yaml: %v", err)
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing node.join.yaml: %v", err)
	}

	deps := map[string]bool{}
	for _, s := range raw.Spec.Steps {
		if s.ID == "report_installed" {
			for _, d := range s.DependsOn {
				deps[d] = true
			}
		}
	}
	if len(deps) == 0 {
		t.Fatal("report_installed step not found or has no dependsOn")
	}
	for _, s := range steps {
		if strings.HasPrefix(s.ID, workloadStepPrefix) && !deps[s.ID] {
			t.Errorf("report_installed must depend on %s — otherwise installed-state sync can run before that group settles", s.ID)
		}
	}
}

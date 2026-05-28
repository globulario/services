package intentaudit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestMetaIntentRequirementChildAlignmentContractIsActive guards the
// meta-rule that closes the awareness loop. Without this contract loaded
// and shaped correctly, child intents born from requirements have no
// alignment check — and architectures rot when a "useful" child intent
// silently inverts its parent's authority or direction.
//
// This test is the durable proof that the rule exists. It must pass before
// any agent (Claude/Codex/etc.) is allowed to write code in the repo.
//
// References:
//   - docs/intent/meta/intent_requirement_child_alignment_contract.yaml
//   - docs/awareness/intent_alignment_preflight.md
//   - docs/awareness/intent_alignment_definition.md
//   - invariant awareness.child_intent_must_align_with_parent
func TestMetaIntentRequirementChildAlignmentContractIsActive(t *testing.T) {
	path := metaContractPath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("meta-contract missing: %v — the alignment loop is open", err)
	}

	var doc struct {
		ID                string   `yaml:"id"`
		Kind              string   `yaml:"kind"`
		Title             string   `yaml:"title"`
		Status            string   `yaml:"status"`
		Severity          string   `yaml:"severity"`
		CoreLaw           string   `yaml:"core_law"`
		CanonicalLoop     []string `yaml:"canonical_loop"`
		AlignmentVerdicts map[string]struct {
			Meaning           string `yaml:"meaning"`
			AllowedToGenerate bool   `yaml:"allowed_to_generate"`
		} `yaml:"alignment_verdicts"`
		ForbiddenChildIntentDrift []string `yaml:"forbidden_child_intent_drift"`
		AgentEnforcement          struct {
			BeforeEdit []string `yaml:"before_edit"`
			HardBlocks []string `yaml:"hard_blocks"`
		} `yaml:"agent_enforcement"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("meta-contract is malformed YAML: %v", err)
	}

	if doc.ID != "meta.intent_requirement_child_alignment_contract" {
		t.Errorf("id=%q, want meta.intent_requirement_child_alignment_contract", doc.ID)
	}
	if doc.Kind != "meta_intent_contract" {
		t.Errorf("kind=%q, want meta_intent_contract", doc.Kind)
	}
	if doc.Severity != "critical" {
		t.Errorf("severity=%q, want critical (this rule is non-negotiable)", doc.Severity)
	}
	if doc.Status != "active" && doc.Status != "accepted" {
		t.Errorf("status=%q, want active or accepted — proposed status means the rule is not enforced", doc.Status)
	}
	if !strings.Contains(doc.CoreLaw, "child intent") || !strings.Contains(doc.CoreLaw, "alignment") {
		t.Errorf("core_law must explicitly bind child intent + alignment; got: %q", doc.CoreLaw)
	}

	// The 5 verdict states are the closure of all possible outcomes.
	// Missing any one means an agent could land in an unhandled state.
	wantVerdicts := []string{"aligned", "partially_aligned", "conflicting", "duplicate", "unknown_impact"}
	for _, v := range wantVerdicts {
		if _, ok := doc.AlignmentVerdicts[v]; !ok {
			t.Errorf("missing alignment verdict: %s", v)
		}
	}

	// Only "aligned" permits code generation. Any other verdict that
	// silently allows generation would defeat the contract.
	for name, v := range doc.AlignmentVerdicts {
		if name == "aligned" {
			if !v.AllowedToGenerate {
				t.Errorf("verdict 'aligned' must allow generation; got false")
			}
			continue
		}
		if v.AllowedToGenerate {
			t.Errorf("verdict %q must NOT allow generation (only 'aligned' may); got true", name)
		}
	}

	if len(doc.ForbiddenChildIntentDrift) < 5 {
		t.Errorf("forbidden_child_intent_drift has only %d patterns; the rule needs explicit drift coverage",
			len(doc.ForbiddenChildIntentDrift))
	}
	if len(doc.AgentEnforcement.BeforeEdit) == 0 {
		t.Errorf("agent_enforcement.before_edit is empty; agents have no preflight steps")
	}
	if len(doc.AgentEnforcement.HardBlocks) == 0 {
		t.Errorf("agent_enforcement.hard_blocks is empty; nothing stops a conflicting child intent")
	}
}

// metaContractPath locates the meta-contract YAML relative to this test
// file. Running `go test` from any directory must find the same file.
func metaContractPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate repo root")
	}
	// thisFile = .../golang/awareness/intentaudit/alignment_contract_test.go
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(repoRoot, "docs", "intent", "meta", "intent_requirement_child_alignment_contract.yaml")
}

package rules

// globular:tested_by destructive_action_guards

// destructive_action_audit.go — doctor invariant for unguarded runtime-stop intent.
//
// Invariant: destructive_actions.require_explicit_guard
//
// The ingress desired spec may carry mode=disabled. Node agents refuse to stop
// keepalived unless the spec has a fully-qualified explicit-disable guard:
//   - explicit_disabled = true
//   - reason non-empty
//   - generation > 0
//
// This rule inspects the *desired spec* (IngressSpecRaw from etcd) and emits a
// CRITICAL finding when a disable intent is present but the guard is incomplete.
// The finding fires proactively — before any node has processed the spec —
// giving the operator a window to correct the spec without a runtime disruption.
//
// This is complementary to ingressAmbiguousDisableRejected (in
// critical_state_guardians.go), which fires *after* a node has already seen the
// bad spec and reported phase=DEGRADED_SPEC_INVALID. That rule requires node
// status; this rule requires only the desired spec key.
//
// This rule does NOT mutate etcd and does NOT stop or start any runtime.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ingressSpecDisableGuard is a local mirror of the fields from
// node_agent/internal/ingress.Spec that are needed to evaluate the explicit-
// disable guard invariant. We cannot import the node_agent internal package
// from the doctor — keep this in sync with ingress.Spec.IsExplicitDisable().
type ingressSpecDisableGuard struct {
	Mode             string `json:"mode"`
	ExplicitDisabled bool   `json:"explicit_disabled"`
	Reason           string `json:"reason"`
	Generation       int64  `json:"generation"`
	WriterLeaderID   string `json:"writer_leader_id"`
	Source           string `json:"source"`
	Authoritative    bool   `json:"authoritative"`
}

// isExplicitDisable mirrors ingress.Spec.IsExplicitDisable().
// Must stay in sync with that function (Case 11: UNGUARDED_RUNTIME_DESTRUCTIVE_ACTION).
func (g ingressSpecDisableGuard) isExplicitDisable() bool {
	return g.Mode == "disabled" &&
		g.ExplicitDisabled &&
		g.Reason != "" &&
		g.Generation > 0
}

// missingGuardFields returns a human-readable list of which required guard
// fields are absent or zero-valued. Used only when mode=disabled and
// isExplicitDisable() returns false.
func (g ingressSpecDisableGuard) missingGuardFields() string {
	var parts []string
	if !g.ExplicitDisabled {
		parts = append(parts, "explicit_disabled=false (must be true)")
	}
	if g.Reason == "" {
		parts = append(parts, "reason is empty")
	}
	if g.Generation <= 0 {
		parts = append(parts, "generation <= 0")
	}
	if len(parts) == 0 {
		return "(none — guard is complete)"
	}
	return strings.Join(parts, "; ")
}

// ingressUnguardedDisableIntent is the doctor invariant that fires when the
// ingress desired spec carries mode=disabled without a valid explicit-disable
// guard. Node agents hold last-known-good and keepalived keeps running, but the
// controller intent is malformed and must be corrected.
type ingressUnguardedDisableIntent struct{}

func (ingressUnguardedDisableIntent) ID() string       { return "ingress.unguarded_disable_intent" }
func (ingressUnguardedDisableIntent) Category() string { return "safety" }
func (ingressUnguardedDisableIntent) Scope() string    { return "cluster" }

func (ingressUnguardedDisableIntent) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if !snap.IngressSpecPresent || snap.IngressSpecRaw == "" {
		return nil
	}

	var spec ingressSpecDisableGuard
	if err := json.Unmarshal([]byte(snap.IngressSpecRaw), &spec); err != nil {
		// Parse failure is surfaced by other invariants (etcd integrity).
		return nil
	}

	// Only applies to specs carrying a disable intent.
	if spec.Mode != "disabled" {
		return nil
	}

	// Guard complete — transition is intentional and auditable.
	if spec.isExplicitDisable() {
		return nil
	}

	missing := spec.missingGuardFields()

	return []Finding{{
		FindingID:   FindingID("ingress.unguarded_disable_intent", "cluster", "ingress_spec"),
		InvariantID: "ingress.unguarded_disable_intent",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "safety",
		EntityRef:   "cluster",
		Summary: fmt.Sprintf(
			"Ingress desired spec carries mode=disabled without a valid explicit-disable guard. "+
				"Node agents will hold last-known-good and keepalived will NOT stop. "+
				"Missing guard fields: %s. "+
				"Invariant: destructive_actions.require_explicit_guard.",
			missing,
		),
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("etcd", "Get(/globular/ingress/v1/spec)", map[string]string{
				"mode":              spec.Mode,
				"explicit_disabled": fmt.Sprintf("%t", spec.ExplicitDisabled),
				"reason":            spec.Reason,
				"generation":        fmt.Sprintf("%d", spec.Generation),
				"writer_leader_id":  spec.WriterLeaderID,
				"source":            spec.Source,
				"authoritative":     fmt.Sprintf("%t", spec.Authoritative),
				"missing_guard":     missing,
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Inspect the current ingress spec to confirm the disable intent is accidental or malformed.",
				"globular config get /globular/ingress/v1/spec | jq '{mode, explicit_disabled, reason, generation}'",
			),
			step(2,
				"If the disable is accidental, republish the spec with the correct active mode.",
				"globular ingress publish --mode vip_failover",
			),
			step(3,
				"If the disable is intentional, re-issue with all required guard fields: explicit_disabled=true, non-empty reason, positive generation.",
				"globular ingress disable --reason '<operator-provided reason>'",
			),
		},
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}

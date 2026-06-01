package main

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/remediation"
	"github.com/spf13/cobra"
)

// OverrideFlags adds the structured operator-override flag set to any
// command that has a --force (or equivalent gate-bypass) flag. The
// resulting remediation.Override carries Actor / Reason / PolicyID /
// Scope / IssuedAt / Expiry / CorrelationID so a bare "--force" is no
// longer a magic bypass — operators must say why, what they're
// overriding, and for how long. See docs/intent/operator.override_intent.yaml.
type OverrideFlags struct {
	Actor    string
	Reason   string
	PolicyID string
	Scope    string
	Lifetime time.Duration
}

// AttachOverrideFlags registers the shared override flag set on a cobra
// command. Use this on every command that has a --force / --bypass /
// --skip-* gate. Pair with BuildOverride at RunE time.
func AttachOverrideFlags(cmd *cobra.Command, f *OverrideFlags) {
	cmd.Flags().StringVar(&f.Actor, "override-actor", "", "Operator identity issuing the override")
	cmd.Flags().StringVar(&f.Reason, "override-reason", "", "Why the policy gate is being bypassed (≥10 characters)")
	cmd.Flags().StringVar(&f.PolicyID, "override-policy", "", "Policy ID being bypassed (e.g. node.recovery.quorum_safety)")
	cmd.Flags().StringVar(&f.Scope, "override-scope", "", "Override scope (typically node-id, finding-id, or service-id)")
	cmd.Flags().DurationVar(&f.Lifetime, "override-lifetime", 15*time.Minute, "How long the override is valid (max 1h)")
}

// separatorIfNonEmpty returns " | " when s is non-empty, else "". Used
// when concatenating override metadata onto an existing operator note.
func separatorIfNonEmpty(s string) string {
	if s == "" {
		return ""
	}
	return " | "
}

// BuildOverride assembles a remediation.Override from the flag set and
// validates it. Use this only when the operator passed --force (or
// equivalent). The override carries a derived correlation id so the
// server-side audit can join to it.
//
// Returns an error when any required field is missing — that error must
// stop the command before the gate-bypassing RPC is sent. A bare --force
// without --override-* fields is exactly what this contract forbids.
func BuildOverride(f OverrideFlags) (remediation.Override, error) {
	now := time.Now()
	o := remediation.Override{
		Actor:         f.Actor,
		Reason:        f.Reason,
		PolicyID:      f.PolicyID,
		Scope:         f.Scope,
		IssuedAt:      now,
		Expiry:        now.Add(f.Lifetime),
		CorrelationID: fmt.Sprintf("override-%d", now.UnixNano()),
	}
	if err := o.Validate(now); err != nil {
		return remediation.Override{}, fmt.Errorf("override flags incomplete: %w (pass --override-actor, --override-reason, --override-policy, --override-scope)", err)
	}
	return o, nil
}

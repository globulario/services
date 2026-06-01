// Package remediation defines cluster-wide policy primitives shared between
// the cluster_doctor remediation handler and the workflow remediation
// actors. The policy lives in this neutral package so both surfaces apply
// identical thresholds and a future operator-tunable layer can override
// them in one place. See docs/intent/remediation.failure_rate_policy.yaml.
package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ActionClass identifies a remediation action category. The string values
// match the doctor's cluster_doctorpb.ActionType.String() output so callers
// can convert from proto to ActionClass via simple casting.
type ActionClass string

// ClassPolicy is the failure-rate budget for one action class.
type ClassPolicy struct {
	// Threshold is the maximum number of failed attempts within Window
	// before the circuit breaker trips for this action class.
	Threshold int `json:"threshold"`

	// Window is the rolling window over which failures are counted.
	Window time.Duration `json:"window"`
}

// FailureRatePolicy is the cluster-wide failure-rate contract. It is the
// single source of throttling truth: every remediation surface (doctor,
// workflow, future controllers) consults the same struct.
type FailureRatePolicy struct {
	// ClassPolicies overrides defaults for named action classes.
	ClassPolicies map[ActionClass]ClassPolicy `json:"class_policies"`

	// Default applies when an action class has no specific override.
	Default ClassPolicy `json:"default"`
}

// FailureRatePolicyEtcdKey is where operators publish a cluster-wide
// override. Absent key means defaults apply.
const FailureRatePolicyEtcdKey = "/globular/cluster_doctor/failure_rate_policy"

// DefaultFailureRatePolicy returns the built-in baseline. Action classes
// not listed here fall back to Default. The numbers come from operational
// experience: low-risk restarts can retry several times, but topology- or
// reseed-class actions should escalate after very few failures because
// each attempt has a much higher blast radius.
func DefaultFailureRatePolicy() *FailureRatePolicy {
	return &FailureRatePolicy{
		Default: ClassPolicy{Threshold: 3, Window: 30 * time.Minute},
		ClassPolicies: map[ActionClass]ClassPolicy{
			"SYSTEMCTL_RESTART":  {Threshold: 5, Window: 30 * time.Minute},
			"SYSTEMCTL_STOP":     {Threshold: 2, Window: 60 * time.Minute},
			"SYSTEMCTL_DISABLE":  {Threshold: 2, Window: 60 * time.Minute},
			"PACKAGE_REINSTALL":  {Threshold: 2, Window: 24 * time.Hour},
			"PACKAGE_REPAIR":     {Threshold: 3, Window: 60 * time.Minute},
			"FILE_DELETE":        {Threshold: 1, Window: 24 * time.Hour},
			"OBJECTSTORE_REPAIR": {Threshold: 1, Window: 24 * time.Hour},
		},
	}
}

// For returns the ClassPolicy that applies to class. Unknown classes get
// Default — the policy is total: every action class has an answer.
func (p *FailureRatePolicy) For(class ActionClass) ClassPolicy {
	if p == nil {
		return DefaultFailureRatePolicy().Default
	}
	if cp, ok := p.ClassPolicies[class]; ok {
		return cp
	}
	return p.Default
}

// Allow reports whether one more attempt of class is permitted given the
// observed failure count within the class's window. When the breaker is
// open, the second return value names the policy that fired.
func (p *FailureRatePolicy) Allow(class ActionClass, recentFailures int) (bool, string) {
	cp := p.For(class)
	if recentFailures >= cp.Threshold {
		return false, fmt.Sprintf(
			"failure-rate breaker open for %s: %d failures within %s ≥ threshold %d",
			class, recentFailures, cp.Window, cp.Threshold,
		)
	}
	return true, ""
}

// PolicyEtcdGetter is the minimal etcd surface LoadFromEtcd needs. The
// real cluster client (clientv3.KV) satisfies it; tests pass a fake.
type PolicyEtcdGetter interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// LoadFromEtcd reads the operator-published policy at FailureRatePolicyEtcdKey
// and merges it onto DefaultFailureRatePolicy. Absent key, parse error, or
// nil getter all return defaults so the policy is always defined — a missing
// etcd value must never silently disable throttling.
func LoadFromEtcd(ctx context.Context, getter PolicyEtcdGetter) *FailureRatePolicy {
	defaults := DefaultFailureRatePolicy()
	if getter == nil {
		return defaults
	}
	raw, err := getter.Get(ctx, FailureRatePolicyEtcdKey)
	if err != nil || len(raw) == 0 {
		return defaults
	}
	override := &FailureRatePolicy{}
	if err := json.Unmarshal(raw, override); err != nil {
		return defaults
	}
	// Merge: override wins on each named class, defaults fill the rest.
	merged := DefaultFailureRatePolicy()
	if override.Default.Threshold > 0 && override.Default.Window > 0 {
		merged.Default = override.Default
	}
	for k, v := range override.ClassPolicies {
		merged.ClassPolicies[k] = v
	}
	return merged
}

// NormalizeActionClass uppercases and trims a raw proto string so doctor
// and workflow callers can pass either ActionType.String() or a string
// constant without case sensitivity surprises.
func NormalizeActionClass(s string) ActionClass {
	return ActionClass(strings.ToUpper(strings.TrimSpace(s)))
}

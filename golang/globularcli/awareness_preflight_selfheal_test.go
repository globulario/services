package main

import (
	"testing"

	"github.com/globulario/awareness/assurance"
	"github.com/globulario/awareness/preflight"
)

func TestPreflightNeedsSelfHeal_UnknownNotSafe(t *testing.T) {
	r := &preflight.Report{SafetyStatus: preflight.SafetyStatusUnknownNotSafe}
	if !preflightNeedsSelfHeal(r) {
		t.Fatal("expected self-heal for UNKNOWN_NOT_SAFE")
	}
}

func TestPreflightNeedsSelfHeal_StaleTrust(t *testing.T) {
	r := &preflight.Report{
		Trust: &assurance.TrustEnvelope{
			Verdict:   assurance.TrustStale,
			Freshness: assurance.FreshnessStaleRepo,
		},
	}
	if !preflightNeedsSelfHeal(r) {
		t.Fatal("expected self-heal for stale trust")
	}
}

func TestPreflightNeedsSelfHeal_ProceedFresh(t *testing.T) {
	r := &preflight.Report{
		SafetyStatus: preflight.SafetyStatusProceed,
		Trust: &assurance.TrustEnvelope{
			Verdict:   assurance.TrustUsable,
			Freshness: assurance.FreshnessFresh,
		},
	}
	if preflightNeedsSelfHeal(r) {
		t.Fatal("did not expect self-heal for PROCEED + fresh trust")
	}
}


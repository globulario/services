package collector

import (
	"testing"
)

// Ingress-disabled cluster: the keepalived target must have
// RuntimeNeeded cleared so the verifier does not raise
// service.runtime_identity_unproven every sweep when the unit is
// expected to be inactive by policy.
func TestApplyIngressPolicyToTargets_DisabledClearsKeepalivedRuntimeNeeded(t *testing.T) {
	snap := &Snapshot{
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"mode":"disabled","generation":1}`,
		DesiredServiceTargets: map[string]*DesiredServiceTarget{
			"keepalived": {Service: "keepalived", RuntimeNeeded: true},
			"envoy":      {Service: "envoy", RuntimeNeeded: true},
		},
	}
	applyIngressPolicyToTargets(snap)
	if snap.DesiredServiceTargets["keepalived"].RuntimeNeeded {
		t.Errorf("keepalived target must have RuntimeNeeded=false under ingress disabled (got true)")
	}
	if !snap.DesiredServiceTargets["envoy"].RuntimeNeeded {
		t.Errorf("envoy target must NOT be touched by ingress policy (got RuntimeNeeded=false)")
	}
}

// explicit_disabled=true is honoured even when mode is something else
// (some operator paths set the flag without rewriting the mode field).
func TestApplyIngressPolicyToTargets_ExplicitDisabledHonoured(t *testing.T) {
	snap := &Snapshot{
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"mode":"active","explicit_disabled":true}`,
		DesiredServiceTargets: map[string]*DesiredServiceTarget{
			"keepalived": {Service: "keepalived", RuntimeNeeded: true},
		},
	}
	applyIngressPolicyToTargets(snap)
	if snap.DesiredServiceTargets["keepalived"].RuntimeNeeded {
		t.Errorf("explicit_disabled=true must clear RuntimeNeeded even when mode=active")
	}
}

// Ingress active + keepalived target: RuntimeNeeded stays true so any
// real keepalived outage surfaces as a normal drift finding. This is
// the load-bearing case — a hot ingress with keepalived missing IS an
// incident, and we must not silence it.
func TestApplyIngressPolicyToTargets_ActiveKeepsRuntimeNeeded(t *testing.T) {
	snap := &Snapshot{
		IngressSpecPresent: true,
		IngressSpecRaw:     `{"mode":"active","generation":2}`,
		DesiredServiceTargets: map[string]*DesiredServiceTarget{
			"keepalived": {Service: "keepalived", RuntimeNeeded: true},
		},
	}
	applyIngressPolicyToTargets(snap)
	if !snap.DesiredServiceTargets["keepalived"].RuntimeNeeded {
		t.Errorf("ingress active must NOT clear keepalived RuntimeNeeded — real outages must still surface")
	}
}

// Missing / malformed ingress spec: fail-open (no policy override).
// Conservative — we only relax RuntimeNeeded on a confirmed disabled
// state, never on a read or parse failure.
func TestApplyIngressPolicyToTargets_FailOpen(t *testing.T) {
	cases := []struct {
		name string
		snap *Snapshot
	}{
		{"nil snap", nil},
		{"no spec key", &Snapshot{IngressSpecPresent: false}},
		{"empty raw", &Snapshot{IngressSpecPresent: true, IngressSpecRaw: ""}},
		{"malformed JSON", &Snapshot{IngressSpecPresent: true, IngressSpecRaw: "not-json"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.snap != nil && c.snap.DesiredServiceTargets == nil {
				c.snap.DesiredServiceTargets = map[string]*DesiredServiceTarget{
					"keepalived": {Service: "keepalived", RuntimeNeeded: true},
				}
			}
			applyIngressPolicyToTargets(c.snap)
			if c.snap != nil {
				if !c.snap.DesiredServiceTargets["keepalived"].RuntimeNeeded {
					t.Errorf("read/parse failure must NOT clear RuntimeNeeded (fail-open)")
				}
			}
		})
	}
}

// Wrapper-package detection: manifest entrypoints that consist of only
// "bin/noop" mean the Globular package ships no real binary — the OS
// supplies it. The detector must be conservative: only the literal
// no-op sentinel counts as a wrapper signal.
func TestManifestEntrypointsAreNoopOnly(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want bool
	}{
		{"nil", nil, false},
		{"empty slice", []string{}, false},
		{"bin/noop alone", []string{"bin/noop"}, true},
		{"bin/noop with whitespace", []string{"  bin/noop  "}, true},
		{"bin/noop plus real entrypoint", []string{"bin/noop", "bin/keepalived"}, false},
		{"real entrypoint alone", []string{"bin/keepalived"}, false},
		{"multiple no-ops same value", []string{"bin/noop", "bin/noop"}, true},
		{"trailing-slash variant not matched", []string{"bin/noop/"}, false},
		{"empty string in slice", []string{""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := manifestEntrypointsAreNoopOnly(c.in); got != c.want {
				t.Errorf("manifestEntrypointsAreNoopOnly(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestIngressDisabledFromSnapshot(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"mode disabled", `{"mode":"disabled"}`, true},
		{"mode DISABLED case insensitive", `{"mode":"DISABLED"}`, true},
		{"explicit_disabled true", `{"mode":"active","explicit_disabled":true}`, true},
		{"mode active", `{"mode":"active"}`, false},
		{"empty", "", false},
		{"malformed", "not-json", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			snap := &Snapshot{IngressSpecPresent: c.raw != "", IngressSpecRaw: c.raw}
			if got := ingressDisabledFromSnapshot(snap); got != c.want {
				t.Errorf("ingressDisabledFromSnapshot(%q) = %v, want %v", c.raw, got, c.want)
			}
		})
	}
}

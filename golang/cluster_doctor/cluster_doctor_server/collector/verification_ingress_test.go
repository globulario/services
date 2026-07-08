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
		{"none alone", []string{"none"}, true},
		{"none with mixed case", []string{" None "}, true},
		{"bin/noop plus real entrypoint", []string{"bin/noop", "bin/keepalived"}, false},
		{"none plus real entrypoint", []string{"none", "bin/scylla_manager"}, false},
		{"real entrypoint alone", []string{"bin/keepalived"}, false},
		{"multiple no-ops same value", []string{"bin/noop", "bin/noop"}, true},
		{"multiple explicit none values", []string{"none", "none"}, true},
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

// COMMAND-kind packages: RuntimeNeeded must be cleared so the verifier does
// not emit runtime_identity_unproven for CLI binaries (mc, restic, etc.).
func TestApplyCommandKindPolicyToTargets_ClearsRuntimeNeeded(t *testing.T) {
	snap := &Snapshot{
		NodePackageKinds: map[string]map[string]string{
			"node-1": {"mc": "COMMAND", "restic": "COMMAND", "envoy": "SERVICE"},
		},
		DesiredServiceTargets: map[string]*DesiredServiceTarget{
			"mc":     {Service: "mc", RuntimeNeeded: true},
			"restic": {Service: "restic", RuntimeNeeded: true},
			"envoy":  {Service: "envoy", RuntimeNeeded: true},
		},
	}
	applyCommandKindPolicyToTargets(snap)
	if snap.DesiredServiceTargets["mc"].RuntimeNeeded {
		t.Error("mc (COMMAND) must have RuntimeNeeded=false")
	}
	if snap.DesiredServiceTargets["restic"].RuntimeNeeded {
		t.Error("restic (COMMAND) must have RuntimeNeeded=false")
	}
	if !snap.DesiredServiceTargets["envoy"].RuntimeNeeded {
		t.Error("envoy (SERVICE) must NOT have RuntimeNeeded cleared")
	}
}

func TestApplyCommandKindPolicyToTargets_NilSafe(t *testing.T) {
	// Must not panic on nil snap or empty maps.
	applyCommandKindPolicyToTargets(nil)
	applyCommandKindPolicyToTargets(&Snapshot{})
	applyCommandKindPolicyToTargets(&Snapshot{NodePackageKinds: map[string]map[string]string{}})
}

func TestApplyCommandKindPolicyToTargets_MultipleNodes(t *testing.T) {
	// If any node says COMMAND, RuntimeNeeded is cleared (kind is uniform across nodes).
	snap := &Snapshot{
		NodePackageKinds: map[string]map[string]string{
			"node-1": {"ffmpeg": "COMMAND"},
			"node-2": {"ffmpeg": "COMMAND"},
		},
		DesiredServiceTargets: map[string]*DesiredServiceTarget{
			"ffmpeg": {Service: "ffmpeg", RuntimeNeeded: true},
		},
	}
	applyCommandKindPolicyToTargets(snap)
	if snap.DesiredServiceTargets["ffmpeg"].RuntimeNeeded {
		t.Error("ffmpeg (COMMAND on all nodes) must have RuntimeNeeded=false")
	}
}

func TestApplyCommandKindPolicyToTargets_LegacyInfraCommandPackages(t *testing.T) {
	snap := &Snapshot{
		NodePackageKinds: map[string]map[string]string{
			"node-1": {
				"mc":      "INFRASTRUCTURE",
				"restic":  "INFRASTRUCTURE",
				"etcdctl": "INFRASTRUCTURE",
				"envoy":   "INFRASTRUCTURE",
			},
		},
		DesiredServiceTargets: map[string]*DesiredServiceTarget{
			"mc":      {Service: "mc", RuntimeNeeded: true},
			"restic":  {Service: "restic", RuntimeNeeded: true},
			"etcdctl": {Service: "etcdctl", RuntimeNeeded: true},
			"envoy":   {Service: "envoy", RuntimeNeeded: true},
		},
	}
	applyCommandKindPolicyToTargets(snap)
	if snap.DesiredServiceTargets["mc"].RuntimeNeeded {
		t.Error("mc (legacy INFRASTRUCTURE command package) must have RuntimeNeeded=false")
	}
	if snap.DesiredServiceTargets["restic"].RuntimeNeeded {
		t.Error("restic (legacy INFRASTRUCTURE command package) must have RuntimeNeeded=false")
	}
	if snap.DesiredServiceTargets["etcdctl"].RuntimeNeeded {
		t.Error("etcdctl (legacy INFRASTRUCTURE command package) must have RuntimeNeeded=false")
	}
	if !snap.DesiredServiceTargets["envoy"].RuntimeNeeded {
		t.Error("envoy (real INFRASTRUCTURE service) must keep RuntimeNeeded=true")
	}
}

func TestEntrypointCacheKey_UsesBuildIDWhenPresent(t *testing.T) {
	key := entrypointCacheKey(&DesiredServiceTarget{
		PublisherID:    "core@globular.io",
		Service:        "mcp",
		DesiredVersion: "1.2.64",
		DesiredBuildID: "b-123",
	})
	if key != "build:b-123" {
		t.Fatalf("entrypointCacheKey() = %q, want %q", key, "build:b-123")
	}
}

func TestEntrypointCacheKey_FallsBackToServiceTupleWhenBuildIDMissing(t *testing.T) {
	key := entrypointCacheKey(&DesiredServiceTarget{
		PublisherID:    "core@globular.io",
		Service:        "log",
		DesiredVersion: "1.2.64",
	})
	want := "svc:core@globular.io/log/1.2.64"
	if key != want {
		t.Fatalf("entrypointCacheKey() = %q, want %q", key, want)
	}
}

func TestRejectManifestForBuildMismatch(t *testing.T) {
	cases := []struct {
		name       string
		desiredBID string
		manifestID string
		want       bool
	}{
		{"same", "bid-1", "bid-1", false},
		{"different", "bid-1", "bid-2", true},
		{"desired empty", "", "bid-2", false},
		{"manifest empty", "bid-1", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRejectManifestForBuildMismatch(c.desiredBID, c.manifestID); got != c.want {
				t.Fatalf("shouldRejectManifestForBuildMismatch(%q,%q)=%v want=%v",
					c.desiredBID, c.manifestID, got, c.want)
			}
		})
	}
}

package evidence

import (
	"testing"
	"time"
)

func TestClassify_AuthoritativeWhenFresh(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	p := Provenance{
		Source:     SourceEtcdContract,
		WriterID:   "cluster_controller",
		ObservedAt: now.Add(-30 * time.Second),
	}
	if got := Classify(p, now); got != TrustAuthoritative {
		t.Fatalf("fresh etcd contract: got %s, want AUTHORITATIVE", got)
	}
}

func TestClassify_DegradedPastFreshnessWindow(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	// SourceTelemetry has 90s window. 2 minutes is past 1× but inside 2×.
	p := Provenance{
		Source:     SourceTelemetry,
		WriterID:   "prometheus",
		ObservedAt: now.Add(-2 * time.Minute),
	}
	if got := Classify(p, now); got != TrustDegraded {
		t.Fatalf("2min-old telemetry: got %s, want DEGRADED", got)
	}
}

func TestClassify_StalePastTwiceWindow(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	// SourceServiceLog has 1min window. 5 minutes is past 2×.
	p := Provenance{
		Source:     SourceServiceLog,
		WriterID:   "node_agent",
		ObservedAt: now.Add(-5 * time.Minute),
	}
	if got := Classify(p, now); got != TrustStale {
		t.Fatalf("5min-old log: got %s, want STALE", got)
	}
}

func TestClassify_UntrustedWhenProvenanceIncomplete(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		p    Provenance
	}{
		{"missing writer", Provenance{Source: SourceEtcdContract, ObservedAt: now}},
		{"missing timestamp", Provenance{Source: SourceEtcdContract, WriterID: "cluster_controller"}},
		{"unknown source", Provenance{Source: Source("unknown_made_up"), WriterID: "x", ObservedAt: now}},
		{"empty source", Provenance{WriterID: "x", ObservedAt: now}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.p, now); got != TrustUntrusted {
				t.Fatalf("got %s, want UNTRUSTED", got)
			}
		})
	}
}

func TestWorst_ReturnsLowest(t *testing.T) {
	got := Worst(TrustAuthoritative, TrustDegraded, TrustAuthoritative)
	if got != TrustDegraded {
		t.Fatalf("Worst with one DEGRADED: got %s, want DEGRADED", got)
	}
	got = Worst(TrustDegraded, TrustStale)
	if got != TrustStale {
		t.Fatalf("STALE beats DEGRADED: got %s, want STALE", got)
	}
	got = Worst()
	if got != TrustUntrusted {
		t.Fatalf("empty Worst: got %s, want UNTRUSTED (silence is not freshness)", got)
	}
}

// TestPreflightReportsWeakEvidenceAsUncertain — contract test.
// Preflight callers must surface DEGRADED evidence as "uncertain", and STALE
// or UNTRUSTED as "reject". An agent that asks "may I proceed?" needs an
// answer it can act on — silently dropping the trust level back to "ok"
// would let stale facts authorize privileged actions.
func TestPreflightReportsWeakEvidenceAsUncertain(t *testing.T) {
	cases := []struct {
		level TrustLevel
		want  string
	}{
		{TrustAuthoritative, "ok"},
		{TrustDegraded, "uncertain"},
		{TrustStale, "reject"},
		{TrustUntrusted, "reject"},
	}
	for _, tc := range cases {
		t.Run(string(tc.level), func(t *testing.T) {
			if got := PreflightVerdict(tc.level); got != tc.want {
				t.Fatalf("PreflightVerdict(%s) = %q, want %q", tc.level, got, tc.want)
			}
		})
	}
	// And: weak levels MUST not authorize remediation.
	if AuthorizesRemediation(TrustStale) {
		t.Fatal("AuthorizesRemediation(STALE) must be false")
	}
	if AuthorizesRemediation(TrustUntrusted) {
		t.Fatal("AuthorizesRemediation(UNTRUSTED) must be false")
	}
	if !AuthorizesRemediation(TrustDegraded) {
		t.Fatal("AuthorizesRemediation(DEGRADED) must be true with operator caution")
	}
}

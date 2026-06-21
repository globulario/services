package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// simulatedGroup0Failure models the exact trap that fooled the event-name
// watcher: CQL is still ready (reads/writes to existing keyspaces work) while
// group0 has lost voter quorum, with no live peers and a critical violation.
func simulatedGroup0Failure() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:     scyllaComponent,
		NodeId:        "globule-test",
		Installed:     true,
		DaemonActive:  true,
		Healthy:       false,
		ExpectedPeers: []string{"globule-a", "globule-b", "globule-test"},
		ObservedPeers: []string{}, // no live peers seen from this node
		PeersMatch:    false,
		Runtime: map[string]string{
			"cql_ready":   "true", // the trap: CQL up, group0 down
			"gossip_live": "0",
		},
		Violations: []*cluster_controllerpb.InfraViolation{
			{Id: "scylla.group0.quorum_lost", Severity: "CRITICAL", Message: "group0 lost voter quorum"},
		},
		ProbedAtUnix: 1700000000,
	}
}

// Acceptance test for PR-14: given a simulated Scylla group0/quorum failure,
// the probe emits a structured runtime finding that preserves
// source/authority/severity/evidence, and behavioral-memory ingests it as a raw
// signal + evidence only — never as a promoted principle.
func TestScyllaGroup0Probe_EmitsStructuredFindingOnQuorumLoss(t *testing.T) {
	p := &scyllaGroup0Probe{
		acquire: func(ctx context.Context) (*cluster_controllerpb.InfraProbeResult, error) {
			return simulatedGroup0Failure(), nil
		},
	}

	res := p.Run(context.Background())

	if res.Healthy {
		t.Fatalf("expected unhealthy probe result on group0 quorum loss")
	}
	if res.Component != scyllaComponent {
		t.Errorf("component=%q, want %q", res.Component, scyllaComponent)
	}
	if !strings.Contains(res.Condition, "group0") {
		t.Errorf("condition=%q, want a group0 quorum condition", res.Condition)
	}
	if res.Severity != "critical" {
		t.Errorf("severity=%q, want critical", res.Severity)
	}
	if len(res.Evidence) == 0 {
		t.Errorf("expected structured evidence, got none")
	}
	if res.TruthPlaneRef == "" {
		t.Errorf("expected the claim to cite its truth-plane source ref")
	}
	if res.CandidateInvariant == "" {
		t.Errorf("expected a candidate invariant for the group0 voter gap")
	}

	// Map into a governed observation bundle and assert provenance + authority.
	bundle := observation.FromWatcherProbe("globular-services", api.DomainRef("cluster_operator"),
		observation.WatcherProbeFinding{
			Source:             "ai_watcher",
			ProbeName:          res.ProbeName,
			Component:          res.Component,
			Condition:          res.Condition,
			Observed:           res.Observed,
			Expected:           res.Expected,
			Severity:           res.Severity,
			Evidence:           res.Evidence,
			CandidateInvariant: res.CandidateInvariant,
			RecommendedProbe:   res.RecommendedProbe,
			EntityRef:          res.EntityRef,
			ClusterID:          "c-test",
			TruthPlaneRef:      res.TruthPlaneRef,
			ObservedAtUnix:     res.ObservedAtUnix,
		})

	// source preserved
	if bundle.Signal.SourceKind != observation.SourceKindAIWatcherProbe {
		t.Errorf("source kind=%q, want %q", bundle.Signal.SourceKind, observation.SourceKindAIWatcherProbe)
	}
	// authority preserved — a diagnostic CLAIM, never truth-plane
	if bundle.Signal.AuthorityLevel != api.ObservationAuthorityDiagnostic {
		t.Errorf("signal authority=%q, want DIAGNOSTIC_CLAIM", bundle.Signal.AuthorityLevel)
	}
	if bundle.Signal.Kind != api.SignalAutomatedHealth {
		t.Errorf("signal kind=%q, want AUTOMATED_HEALTH", bundle.Signal.Kind)
	}
	// severity preserved
	if bundle.Signal.Severity != "critical" {
		t.Errorf("severity not preserved on signal: %q", bundle.Signal.Severity)
	}
	// evidence preserved + at diagnostic authority + cites truth plane
	if len(bundle.Evidence) == 0 {
		t.Fatalf("expected evidence on the bundle")
	}
	if bundle.Evidence[0].AuthorityLevel != api.ObservationAuthorityDiagnostic {
		t.Errorf("evidence authority=%q, want DIAGNOSTIC_CLAIM", bundle.Evidence[0].AuthorityLevel)
	}
	if bundle.Evidence[0].Provenance.SourceRef != res.TruthPlaneRef {
		t.Errorf("evidence must cite truth-plane ref %q; got %q", res.TruthPlaneRef, bundle.Evidence[0].Provenance.SourceRef)
	}
	// candidate invariant is recorded as a CANDIDATE, never auto-applied
	if bundle.Signal.Metadata["candidate_invariant"] != res.CandidateInvariant {
		t.Errorf("candidate invariant not recorded in metadata")
	}

	// ingested WITHOUT promotion: a raw signal + evidence only. The bundle has no
	// concept of a Principle, so ingestion can never promote.
	if bundle.Signal.Status != api.StatusRawSignal {
		t.Errorf("signal status=%q, want RAW_SIGNAL (no promotion)", bundle.Signal.Status)
	}
}

// A healthy cluster must not produce a finding (no false alarms / no flooding).
func TestScyllaGroup0Probe_HealthyClusterIsNotEmitted(t *testing.T) {
	healthy := &cluster_controllerpb.InfraProbeResult{
		Component:     scyllaComponent,
		NodeId:        "globule-test",
		Installed:     true,
		DaemonActive:  true,
		Healthy:       true,
		ExpectedPeers: []string{"a", "b", "globule-test"},
		ObservedPeers: []string{"a", "b"}, // both peers live; +self = full quorum
		PeersMatch:    true,
		Runtime:       map[string]string{"cql_ready": "true", "gossip_live": "2"},
		ProbedAtUnix:  1700000000,
	}
	p := &scyllaGroup0Probe{
		acquire: func(ctx context.Context) (*cluster_controllerpb.InfraProbeResult, error) { return healthy, nil },
	}
	res := p.Run(context.Background())
	if !res.Healthy {
		t.Fatalf("healthy cluster flagged unhealthy: condition=%q evidence=%v", res.Condition, res.Evidence)
	}
}

// A probe that cannot reach its data source must surface the blind spot as an
// indeterminate warning — tool failure is evidence, not silence.
func TestScyllaGroup0Probe_AcquireFailureIsIndeterminate(t *testing.T) {
	p := &scyllaGroup0Probe{
		acquire: func(ctx context.Context) (*cluster_controllerpb.InfraProbeResult, error) {
			return nil, context.DeadlineExceeded
		},
	}
	res := p.Run(context.Background())
	if res.Healthy || !res.Indeterminate {
		t.Fatalf("acquire failure should be unhealthy+indeterminate; got healthy=%t indeterminate=%t", res.Healthy, res.Indeterminate)
	}
	if res.Severity != "warning" {
		t.Errorf("indeterminate severity=%q, want warning", res.Severity)
	}
	if len(res.Evidence) == 0 {
		t.Errorf("expected the failure to be recorded as evidence")
	}
}

// Below founding quorum (dev / single-node), membership math must not fire — only
// an explicit violation should. Guards against false positives.
func TestScyllaGroup0Probe_BelowFoundingQuorumNoMembershipFalsePositive(t *testing.T) {
	single := &cluster_controllerpb.InfraProbeResult{
		Component:     scyllaComponent,
		NodeId:        "globule-dev",
		Installed:     true,
		DaemonActive:  true,
		Healthy:       true,
		ExpectedPeers: []string{"globule-dev"}, // single-node
		ObservedPeers: []string{},
		PeersMatch:    true,
		Runtime:       map[string]string{"cql_ready": "true", "gossip_live": "0"},
		ProbedAtUnix:  1700000000,
	}
	p := &scyllaGroup0Probe{
		acquire: func(ctx context.Context) (*cluster_controllerpb.InfraProbeResult, error) { return single, nil },
	}
	res := p.Run(context.Background())
	if !res.Healthy {
		t.Fatalf("single-node dev cluster must not be flagged via membership math: condition=%q", res.Condition)
	}
}

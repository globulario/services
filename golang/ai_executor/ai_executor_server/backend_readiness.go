package main

import "sync/atomic"

// backendReadiness is the honest, layered capability report for the AI
// diagnosis backend. The four states are reported independently so an operator
// can see exactly where the chain stops, instead of a single bool that
// conflated "a claude binary exists on disk" with "the executor can actually
// reason".
//
// This is the operational form of meta.authority_must_express_uncertainty: the
// owner of "can this executor reason?" must be able to say *which* level it has
// reached, otherwise callers turn a bare ai_available bool into a lie. That is
// precisely what happened — ai_available was computed as
//
//	anthropic.isAvailable() || claude.isAvailable()
//
// and claude.isAvailable() is merely "the CLI binary is present". On the live
// cluster that reported ai_available=true while every diagnosis was the 0.2
// deterministic fallback — the binary existed, no credentials did. The
// autonomous diagnose() path only ever uses the anthropic backend (the CLI is
// intentionally excluded; see
// ai_executor.repeat_diagnosis_drains_personal_subscription), so "AI is ready"
// means exactly "the anthropic backend is usable".
type backendReadiness struct {
	BinaryPresent      bool // the interactive Claude CLI binary exists on disk
	CredentialsPresent bool // credential material is configured (may be expired)
	BackendReady       bool // the autonomous-diagnosis backend (anthropic) is usable
	AnalysisAvailable  bool // at least one real AI analysis has succeeded at runtime
}

// Mode returns a human-readable operating mode for status/logs: "ai" when the
// backend is ready, otherwise "deterministic_fallback" — never masquerading.
func (r backendReadiness) Mode() string {
	if r.BackendReady {
		return "ai"
	}
	return "deterministic_fallback"
}

// readiness computes the honest backend readiness for this diagnoser.
func (d *diagnoser) readiness() backendReadiness {
	r := backendReadiness{}
	if d == nil {
		return r
	}
	if d.claude != nil {
		r.BinaryPresent = d.claude.isAvailable() // "binary present" only
	}
	if d.anthropic != nil {
		r.CredentialsPresent = d.anthropic.credentialsPresent()
		r.BackendReady = d.anthropic.isAvailable()
	}
	r.AnalysisAvailable = atomic.LoadInt64(&d.aiAnalysesOK) > 0
	return r
}

// aiReady is the honest meaning of "ai_available": the autonomous diagnose()
// path has a usable AI backend. It is NOT "a claude binary exists on disk".
// Safe to call on a nil diagnoser.
func (d *diagnoser) aiReady() bool {
	return d != nil && d.anthropic != nil && d.anthropic.isAvailable()
}

// logReadiness emits an explicit, greppable backend-readiness line so operators
// see the real mode instead of inferring it from a single bool.
func (d *diagnoser) logReadiness(context string) {
	r := d.readiness()
	logger.Info("ai-executor backend readiness",
		"context", context,
		"mode", r.Mode(),
		"binary_present", r.BinaryPresent,
		"credentials_present", r.CredentialsPresent,
		"backend_ready", r.BackendReady,
		"analysis_available", r.AnalysisAvailable,
	)
}

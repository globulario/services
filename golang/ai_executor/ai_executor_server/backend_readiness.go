package main

import "sync/atomic"

// backendReadiness is the honest, layered capability report for the AI
// diagnosis backend. The states are reported independently so an operator can
// see exactly where the chain stops, instead of a single bool that conflates
// "a CLI binary exists on disk" with "the executor can actually reason".
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
// autonomous diagnose() path now accepts Anthropic API/Max credentials and
// explicitly-provisioned Codex auth, but still excludes the Claude CLI. "AI is
// ready" therefore means exactly "an autonomous backend is usable".
type backendReadiness struct {
	ClaudeBinaryPresent bool
	CodexBinaryPresent  bool
	CredentialsPresent  bool
	BackendReady        bool
	AnalysisAvailable   bool
	Backend             string
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
		r.ClaudeBinaryPresent = d.claude.isAvailable()
	}
	if d.codex != nil {
		r.CodexBinaryPresent = d.codex.binaryPresent()
		r.CredentialsPresent = r.CredentialsPresent || d.codex.credentialsPresent()
		if d.codex.isAvailable() {
			r.BackendReady = true
			r.Backend = "codex"
		}
	}
	if d.anthropic != nil {
		r.CredentialsPresent = r.CredentialsPresent || d.anthropic.credentialsPresent()
		if d.anthropic.isAvailable() {
			r.BackendReady = true
			r.Backend = "anthropic"
		}
	}
	r.AnalysisAvailable = atomic.LoadInt64(&d.aiAnalysesOK) > 0
	return r
}

// aiReady is the honest meaning of "ai_available": the autonomous diagnose()
// path has a usable AI backend. It is NOT "a claude binary exists on disk".
// Safe to call on a nil diagnoser.
func (d *diagnoser) aiReady() bool {
	return d != nil && ((d.anthropic != nil && d.anthropic.isAvailable()) || (d.codex != nil && d.codex.isAvailable()))
}

// logReadiness emits an explicit, greppable backend-readiness line so operators
// see the real mode instead of inferring it from a single bool.
func (d *diagnoser) logReadiness(context string) {
	r := d.readiness()
	logger.Info("ai-executor backend readiness",
		"context", context,
		"mode", r.Mode(),
		"backend", r.Backend,
		"claude_binary_present", r.ClaudeBinaryPresent,
		"codex_binary_present", r.CodexBinaryPresent,
		"credentials_present", r.CredentialsPresent,
		"backend_ready", r.BackendReady,
		"analysis_available", r.AnalysisAvailable,
	)
}

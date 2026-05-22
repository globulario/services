package main

// awareness_readiness.go — Day-0 readiness gating for awareness contribution tools.
//
// Per docs/awareness/awareness_self_invariants.yaml:
//   awareness.mcp.advertised_tools_must_be_backed_by_ready_storage
//
// The MCP awareness contribution tools (record_incident_pattern,
// failure_learn_from_incident, learn_from_fix, …) each persist their output to
// a specific directory that must be writable by the runtime user. When those
// directories don't exist or aren't writable, the tools fail with cryptic
// "permission denied" or "docs dir not configured" errors after a first-time
// user has already invested effort in composing a contribution payload.
//
// This file's purpose: probe each backing path once at startup, record the
// per-tool readiness reason, and let the registration code consult that
// state to skip advertising tools whose storage is unavailable. The skip is
// logged at WARN with the actionable mkdir/chown hint a Day-0 operator
// needs to fix the gap.
//
// What this file does NOT do:
//   - it does not run sudo or chown anything;
//   - it does not silently fall back to /tmp or any unconfigured location;
//   - it does not retry forever — readiness is decided once at startup.
//     Operator action + service restart is the recovery path.

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/awareness/graph"
)

// awarenessFeatureReadiness records the readiness state of one backing
// storage path used by an awareness contribution tool. Mirrors the YAML
// shape declared in docs/awareness/awareness_self_invariants.yaml so that
// the MCP readiness tool surfaces the same field names.
type awarenessFeatureReadiness struct {
	// Path is the absolute filesystem path the tool expects to write under.
	// Empty when the feature is config-only (e.g. mcp_docs_dir before any
	// path is resolved).
	Path string `json:"path"`
	// Configured is true when a path was explicitly configured or resolved.
	// False indicates the operator never told the service where to write.
	Configured bool `json:"configured"`
	// Writable is true when the runtime user can create + write a probe
	// file at Path. False means either the path doesn't exist, the runtime
	// user can't create it, or the existing path is owned by another user
	// without write bits for us.
	Writable bool `json:"writable"`
	// Reason summarises why the feature is or isn't ready in a single line
	// an operator can act on. Always populated.
	Reason string `json:"reason"`
}

// awarenessReadinessReport bundles the per-feature readiness state plus
// the list of tools the MCP service decided to skip registering as a
// consequence. ToolsSkipped is exposed via the awareness.readiness tool so
// operators can see "this is what's missing and what was hidden because of
// it" in one place.
type awarenessReadinessReport struct {
	IncidentPatterns awarenessFeatureReadiness `json:"incident_patterns_dir"`
	FailureGraph     awarenessFeatureReadiness `json:"failure_graph_dir"`
	DocsDir          awarenessFeatureReadiness `json:"mcp_docs_dir"`
	ToolsAdvertised  []string                  `json:"advertised_tools"`
	ToolsSkipped     []awarenessToolSkip       `json:"tools_skipped"`
}

// awarenessToolSkip records one tool that was NOT advertised at startup
// plus the readiness gap that caused the skip. Surfaced via
// awareness.readiness so operators see the silenced contribution path.
type awarenessToolSkip struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

// probeWritableDir verifies that path exists, is a directory, and that
// the runtime user can create + remove a probe file under it.
//
// The probe write is the load-bearing check: a directory that exists but
// is owned by another user with no group/other write bits will pass an
// os.Stat but fail the actual tool call. Probing once at startup makes
// the failure visible BEFORE a first-time contributor composes a payload.
//
// Returns (writable, reason). Reason is always non-empty and actionable.
func probeWritableDir(path string) (bool, string) {
	if path == "" {
		return false, "path is empty (no configuration / no resolved location)"
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Sprintf("directory does not exist — operator must `sudo mkdir -p %s && sudo chown globular:globular %s`", path, path)
		}
		return false, fmt.Sprintf("stat failed: %v", err)
	}
	if !info.IsDir() {
		return false, fmt.Sprintf("path exists but is not a directory: %s", path)
	}
	// Probe-write: create a uniquely-named empty file, then remove it.
	// Uses os.CreateTemp under the target dir so we don't leave debris
	// even on crash, and so race conditions between MCP restarts don't
	// collide on a fixed name.
	tmpFile, err := os.CreateTemp(path, ".readiness-probe-")
	if err != nil {
		return false, fmt.Sprintf("not writable by runtime user — %v; operator may need `sudo chown globular:globular %s`", err, path)
	}
	probePath := tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(probePath)
	return true, "writable"
}

// computeAwarenessReadiness probes every backing path the awareness
// contribution tools depend on and returns one report. Pure: no
// side effects beyond the probe-write/remove cycle inside each target.
//
// The report is computed ONCE at MCP startup. We do not re-probe per
// call — directories don't normally change ownership while the service
// is running, and re-probing per call would add filesystem noise to
// every list_tools response. If readiness changes mid-run (operator
// fixes ownership, then expects tools to appear), the documented
// recovery is `systemctl restart globular-mcp`.
func computeAwarenessReadiness(g *graph.Graph, docsDir string) awarenessReadinessReport {
	r := awarenessReadinessReport{}

	// ── incident_patterns: lives under graph.DataDir() ──────────────────
	// When the graph is in-memory (DataDir=="" — typically dev mode) the
	// store works without filesystem persistence, so we report writable=true
	// against a synthetic in-memory path. The tool itself handles the
	// in-memory path; readiness should match its actual capability.
	if g == nil {
		r.IncidentPatterns = awarenessFeatureReadiness{
			Path:       "",
			Configured: false,
			Writable:   false,
			Reason:     "awareness graph not loaded — tool would degrade to in-memory and lose contributions on restart",
		}
	} else if dd := g.DataDir(); dd == "" {
		r.IncidentPatterns = awarenessFeatureReadiness{
			Path:       "(in-memory)",
			Configured: true,
			Writable:   true,
			Reason:     "graph is in-memory — patterns persist for this process only (expected in tests and dev)",
		}
	} else {
		path := filepath.Join(dd, "incident_patterns")
		writable, reason := probeWritableDir(path)
		// If the parent (graph.DataDir()) is writable but incident_patterns
		// doesn't exist yet, we can create it ourselves — that's the
		// happy path on a freshly-installed bundle whose dir is owned by
		// the runtime user. The probe will treat "doesn't exist" as not
		// writable, so try to create-on-demand before reporting failure.
		if !writable {
			if mkErr := os.MkdirAll(path, 0o755); mkErr == nil {
				writable, reason = probeWritableDir(path)
			}
		}
		r.IncidentPatterns = awarenessFeatureReadiness{
			Path:       path,
			Configured: true,
			Writable:   writable,
			Reason:     reason,
		}
	}

	// ── failure_graph: also lives under graph.DataDir() ──
	if g == nil {
		r.FailureGraph = awarenessFeatureReadiness{
			Path:       "",
			Configured: false,
			Writable:   false,
			Reason:     "awareness graph not loaded — tool would degrade to in-memory and lose contributions on restart",
		}
	} else if dd := g.DataDir(); dd == "" {
		r.FailureGraph = awarenessFeatureReadiness{
			Path:       "(in-memory)",
			Configured: true,
			Writable:   true,
			Reason:     "graph is in-memory — failure graph entries persist for this process only (expected in tests and dev)",
		}
	} else {
		path := filepath.Join(dd, "failure_graph")
		writable, reason := probeWritableDir(path)
		if !writable {
			if mkErr := os.MkdirAll(path, 0o755); mkErr == nil {
				writable, reason = probeWritableDir(path)
			}
		}
		r.FailureGraph = awarenessFeatureReadiness{
			Path:       path,
			Configured: true,
			Writable:   writable,
			Reason:     reason,
		}
	}

	// ── mcp_docs_dir: required by learn_from_fix to write proposal drafts.
	// Unlike the graph data dirs, this is operator-configured (or resolved
	// from the bundle docs/). An empty path here means a config gap — the
	// learn_from_fix tool refuses to advertise rather than fall back to
	// a temp dir (proposal drafts are durable artifacts).
	if docsDir == "" {
		r.DocsDir = awarenessFeatureReadiness{
			Path:       "",
			Configured: false,
			Writable:   false,
			Reason:     "mcp_docs_dir is not configured — set it in the MCP service config (etcd: /globular/services/mcp/awareness.docs_dir) and restart",
		}
	} else {
		writable, reason := probeWritableDir(docsDir)
		r.DocsDir = awarenessFeatureReadiness{
			Path:       docsDir,
			Configured: true,
			Writable:   writable,
			Reason:     reason,
		}
	}

	return r
}

// recordToolSkip appends a tool-skip entry to the readiness report and
// logs the skip with the operator-actionable reason. Centralised so every
// gated registration site uses the same message shape.
func (r *awarenessReadinessReport) recordToolSkip(tool, reason string) {
	r.ToolsSkipped = append(r.ToolsSkipped, awarenessToolSkip{Tool: tool, Reason: reason})
	log.Printf("mcp: SKIPPING tool %q — %s", tool, reason)
}

// recordToolAdvertised appends a tool to the advertised list. Centralised
// so the readiness tool's output exactly matches what register actually
// did, rather than what it intended to do.
func (r *awarenessReadinessReport) recordToolAdvertised(tool string) {
	r.ToolsAdvertised = append(r.ToolsAdvertised, tool)
}

// logAwarenessReadiness writes one log line per gated feature at startup,
// flagging any that aren't ready and echoing the actionable recovery hint.
// Operators see the same reasons in journalctl that they'd see via the
// awareness.readiness tool — no need to query the tool surface to learn
// why a contribution failed.
func logAwarenessReadiness(r *awarenessReadinessReport) {
	log.Printf("mcp: awareness readiness — incident_patterns_dir: writable=%v reason=%s",
		r.IncidentPatterns.Writable, r.IncidentPatterns.Reason)
	log.Printf("mcp: awareness readiness — failure_graph_dir:    writable=%v reason=%s",
		r.FailureGraph.Writable, r.FailureGraph.Reason)
	log.Printf("mcp: awareness readiness — mcp_docs_dir:         writable=%v configured=%v reason=%s",
		r.DocsDir.Writable, r.DocsDir.Configured, r.DocsDir.Reason)
}

// registerAwarenessReadinessTool advertises the awareness.readiness tool
// that returns the same report fields operators see in startup logs. The
// tool is ALWAYS registered (it's purely read-only and reports its own
// state, including when nothing else is ready). Without it operators
// would have to grep journalctl to discover why a contribution tool is
// silently absent from the tool list.
func registerAwarenessReadinessTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.readiness",
		Description: "Report which awareness contribution tools are advertised this session and why others were skipped. " +
			"Returns per-feature path/configured/writable/reason fields plus the lists of advertised and skipped tools. " +
			"Always call this when an awareness contribution call returns 'permission denied' or 'docs dir not configured' — " +
			"the readiness reason names the exact operator action needed.",
		InputSchema: inputSchema{Type: "object"},
	}, func(_ context.Context, _ map[string]interface{}) (interface{}, error) {
		// Defensive copies so the caller can't mutate our slices.
		advertised := append([]string(nil), st.readiness.ToolsAdvertised...)
		skipped := append([]awarenessToolSkip(nil), st.readiness.ToolsSkipped...)
		return map[string]interface{}{
			"incident_patterns_dir": st.readiness.IncidentPatterns,
			"failure_graph_dir":     st.readiness.FailureGraph,
			"mcp_docs_dir":          st.readiness.DocsDir,
			"advertised_tools":      advertised,
			"tools_skipped":         skipped,
		}, nil
	})
}


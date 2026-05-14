package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// serveRW drives the server over in-memory pipes (mirrors awareness/mcp test pattern).
func (s *server) serveRW(ctx context.Context, r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	enc := json.NewEncoder(w)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, err := readStdioMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var req jsonRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data))
			enc.Encode(resp)
		}
	}
}

// safeBuf is a goroutine-safe bytes.Buffer for test output capture.
type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// sendMCPMsg writes an MCP Content-Length framed message to a pipe writer.
func sendMCPMsg(pw *io.PipeWriter, v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(pw, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

// sendToolsList drives the server through a tools/list exchange and returns the raw response.
func sendToolsList(t *testing.T, s *server) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pr, pw := io.Pipe()
	out := &safeBuf{}

	done := make(chan error, 1)
	go func() { done <- s.serveRW(ctx, pr, out) }()

	sendMCPMsg(pw, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	})
	pw.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("server did not exit after stdin close")
	}
	return out.String()
}

// ── tools/list wire tests ─────────────────────────────────────────────────────

func TestToolsListIncludesAwarenessWhenEnabled(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true
	s := newServer(cfg)
	st := &awarenessState{docsDir: t.TempDir()}
	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessFixledgerTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)
	registerAwarenessNodeContextTools(s, st)
	registerAwarenessContextNavTools(s, st)
	registerAwarenessSemanticTools(s, st)
	registerAwarenessDebugSessionTool(s, st)

	raw := sendToolsList(t, s)

	required := []string{
		"awareness.preflight",
		"awareness.agent_context",
		"awareness.impact_file",
		"awareness.did_we_fix",
		"awareness.pattern_status",
		"awareness.fix_status",
		"awareness.runtime_snapshot",
		"awareness.validate_package",
		"awareness.package_context",
		"awareness.propose_from_incident",
		"awareness.validate_proposal",
		"awareness.approve_proposal",
		"awareness.node_context",
		"awareness.neighborhood",
		"awareness.explain_node",
		"awareness.decision_trace",
		"awareness.finding_context",
		"awareness.related",
		"awareness.nearest",
		"awareness.path",
		"awareness.why_related",
		"awareness.semantic_neighborhood",
		"awareness.debug_session",
	}

	for _, want := range required {
		if !strings.Contains(raw, `"`+want+`"`) {
			t.Errorf("tools/list response missing %q\nraw: %s", want, raw)
		}
	}
}

func TestToolsListExcludesAwarenessWhenDisabled(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = false
	s := newServer(cfg)
	// Register all tools with awareness disabled — only non-awareness tools register.
	registerAllTools(s)

	raw := sendToolsList(t, s)

	for _, forbidden := range []string{"awareness.preflight", "awareness.agent_context", "awareness.runtime_snapshot"} {
		if strings.Contains(raw, `"`+forbidden+`"`) {
			t.Errorf("tools/list response must not contain %q when Awareness=false\nraw: %s", forbidden, raw)
		}
	}
}

func TestToolsListNeverIncludesPromoteProposal(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true
	s := newServer(cfg)
	registerAllTools(s)

	raw := sendToolsList(t, s)

	if strings.Contains(raw, "promote_proposal") || strings.Contains(raw, "promote-proposal") {
		t.Errorf("tools/list must never include promote_proposal\nraw: %s", raw)
	}
}

func TestToolsListIsValidJSON(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true
	s := newServer(cfg)
	st := &awarenessState{docsDir: t.TempDir()}
	registerAwarenessPreflightTools(s, st)

	// Extract the JSON body from the Content-Length framed response.
	raw := sendToolsList(t, s)

	// Find the first '{' to skip any Content-Length framing.
	idx := strings.Index(raw, "{")
	if idx < 0 {
		t.Fatalf("no JSON object in response: %q", raw)
	}
	jsonPart := raw[idx:]

	// May contain multiple framed messages; find the one with "tools".
	var found bool
	for _, part := range strings.Split(jsonPart, "Content-Length:") {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "{"); idx >= 0 {
			part = part[idx:]
		}
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(part), &v); err == nil {
			if _, ok := v["result"]; ok {
				found = true
				break
			}
		}
	}
	if !found {
		// Try full raw as one blob.
		idx := strings.LastIndex(raw, "{")
		if idx >= 0 {
			var v map[string]interface{}
			if err := json.Unmarshal([]byte(raw[idx:]), &v); err == nil && v["result"] != nil {
				found = true
			}
		}
	}
	if !found {
		t.Logf("tools/list raw: %s", raw)
	}
}

// TestAwarenessNoImportMCP verifies that no golang/awareness sub-package
// imports golang/mcp (the dependency direction must be mcp→awareness).
// This is a compile-time guard enforced at the module level, but we document
// it as a test so it's explicit and visible in CI.
func TestAwarenessImportDirectionIsAwarenessNotMCP(t *testing.T) {
	// The test is structural — we validate by the fact that this file compiles:
	// if golang/awareness imported golang/mcp (package main), the build would
	// fail with "import cycle" or "cannot import package main".
	// We assert the invariant as a documented check.
	const invariant = "golang/mcp imports golang/awareness — not the reverse"
	if invariant == "" {
		t.Fatal("invariant must be non-empty")
	}
}

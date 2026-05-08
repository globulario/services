package main_test

import (
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

// TestStartupSmoke verifies the server registers all required tools and
// excludes the forbidden promote-proposal tool immediately after creation.
func TestStartupSmoke(t *testing.T) {
	s, _ := makeTestServer(t)

	names := s.ToolNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	if !nameSet["awareness.preflight"] {
		t.Error("startup: awareness.preflight not registered")
	}
	for _, forbidden := range []string{"awareness.promote_proposal", "awareness.promote-proposal"} {
		if nameSet[forbidden] {
			t.Errorf("startup: forbidden tool %q must not be registered", forbidden)
		}
	}
}

// TestStartupSmokeViaJSONRPC exercises the full MCP JSON-RPC wire path:
// initialize → tools/list → verify awareness.preflight present, promote-proposal absent.
func TestStartupSmokeViaJSONRPC(t *testing.T) {
	s, _ := makeTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pr, pw := io.Pipe()
	out := &safeBuffer{}

	srvDone := make(chan error, 1)
	go func() {
		srvDone <- s.ServeRW(ctx, pr, out)
	}()

	encode := func(v interface{}) []byte {
		b, _ := json.Marshal(v)
		return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), b))
	}

	_, _ = pw.Write(encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
		},
	}))
	_, _ = pw.Write(encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}))
	pw.Close()

	select {
	case <-srvDone:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("server did not exit after stdin close")
	}

	raw := out.String()

	if !strings.Contains(raw, `"awareness.preflight"`) {
		t.Errorf("tools/list response missing awareness.preflight\nraw: %s", raw)
	}
	if strings.Contains(raw, "promote_proposal") || strings.Contains(raw, "promote-proposal") {
		t.Errorf("tools/list response contains forbidden promote_proposal\nraw: %s", raw)
	}
}

// safeBuffer is a goroutine-safe bytes.Buffer.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

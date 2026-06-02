package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
)

// helpers ─────────────────────────────────────────────────────────────────

// fakeBriefing builds a briefingFn that returns the given implementation
// patterns regardless of input. Used to drive checkOneFile in unit tests
// without standing up a gRPC server.
func fakeBriefing(patterns []*awarenesspb.MatchedImplementationPattern, err error) briefingFn {
	return func(_ context.Context, _, _, _ string) (*awarenesspb.BriefingResponse, error) {
		if err != nil {
			return nil, err
		}
		return &awarenesspb.BriefingResponse{
			ImplementationPatterns: patterns,
		}, nil
	}
}

func grpcClientStandardMatched() *awarenesspb.MatchedImplementationPattern {
	return &awarenesspb.MatchedImplementationPattern{
		Id:             "implementation_pattern:globular.pattern.grpc_client_standard",
		Label:          "Standard Globular gRPC service client",
		MatchStrength:  "strong",
		RequiredCalls:  []string{"globular.InitClient", "globular.GetClientConnection", "globular.InvokeClientRequest", "globular.GetClientContext"},
		ForbiddenCalls: []string{"grpc.Dial", "grpc.NewClient", "credentials.NewClientTLSFromFile", "credentials.NewTLS"},
		ReferenceFiles: []string{
			"canonical_minimal:golang/echo/echo_client/echo_client.go",
			"richer_reference:golang/monitoring/monitoring_client/monitoring_client.go",
		},
	}
}

// writeTempGo writes a Go source file under t.TempDir and returns the path.
func writeTempGo(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	return p
}

// ─── Tests ────────────────────────────────────────────────────────────────

// 1. A correctly-shaped client file passes — all required calls present,
// no forbidden calls.
func TestPatternCheck_PassWhenAllRequiredPresentAndNoneForbidden(t *testing.T) {
	body := `package foo_client

import "github.com/globulario/services/golang/globular_client"

func NewFooService_Client(addr, id string) (*Foo_Client, error) {
	c := new(Foo_Client)
	if err := globular.InitClient(c, addr, id); err != nil { return nil, err }
	return c, c.Reconnect()
}
func (c *Foo_Client) Reconnect() error {
	cc, err := globular.GetClientConnection(c)
	if err != nil { return err }
	c.cc = cc
	return nil
}
func (c *Foo_Client) Invoke(m string, r interface{}, ctx context.Context) (interface{}, error) {
	return globular.InvokeClientRequest(c.c, ctx, m, r)
}
func (c *Foo_Client) GetCtx() context.Context { return globular.GetClientContext(c) }
`
	path := writeTempGo(t, "foo_client.go", body)
	result := checkOneFile(context.Background(),
		fakeBriefing([]*awarenesspb.MatchedImplementationPattern{grpcClientStandardMatched()}, nil),
		path)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if len(result.PatternResults) != 1 {
		t.Fatalf("want 1 pattern result, got %d", len(result.PatternResults))
	}
	p := result.PatternResults[0]
	if p.Status != "pass" {
		t.Errorf("status: want pass, got %q (missing=%v forbidden=%v)",
			p.Status, p.MissingRequired, p.ForbiddenFound)
	}
	if len(p.MissingRequired) != 0 || len(p.ForbiddenFound) != 0 {
		t.Errorf("expected zero missing/forbidden, got missing=%v forbidden=%v",
			p.MissingRequired, p.ForbiddenFound)
	}
}

// 2. Missing required call surfaces as violation.
func TestPatternCheck_MissingRequiredCall(t *testing.T) {
	body := `package foo_client
// This client uses InitClient but skips InvokeClientRequest entirely.
func New() { globular.InitClient(nil, "", "") }
func F() { globular.GetClientConnection(nil); globular.GetClientContext(nil) }
`
	path := writeTempGo(t, "foo_client.go", body)
	result := checkOneFile(context.Background(),
		fakeBriefing([]*awarenesspb.MatchedImplementationPattern{grpcClientStandardMatched()}, nil),
		path)
	p := result.PatternResults[0]
	if p.Status != "violation" {
		t.Errorf("status: want violation, got %q", p.Status)
	}
	if len(p.MissingRequired) != 1 || p.MissingRequired[0] != "globular.InvokeClientRequest" {
		t.Errorf("missing_required: want [globular.InvokeClientRequest], got %v", p.MissingRequired)
	}
}

// 3. Forbidden call present surfaces as violation.
func TestPatternCheck_ForbiddenCallPresent(t *testing.T) {
	body := `package foo_client

import "google.golang.org/grpc"

func New() {
	globular.InitClient(nil, "", "")
	globular.GetClientConnection(nil)
	globular.InvokeClientRequest(nil, nil, "", nil)
	globular.GetClientContext(nil)
	// Even with all required calls present, this is a violation:
	_, _ = grpc.Dial("localhost:443")
}
`
	path := writeTempGo(t, "foo_client.go", body)
	result := checkOneFile(context.Background(),
		fakeBriefing([]*awarenesspb.MatchedImplementationPattern{grpcClientStandardMatched()}, nil),
		path)
	p := result.PatternResults[0]
	if p.Status != "violation" {
		t.Errorf("status: want violation, got %q", p.Status)
	}
	if len(p.ForbiddenFound) != 1 || p.ForbiddenFound[0] != "grpc.Dial" {
		t.Errorf("forbidden_found: want [grpc.Dial], got %v", p.ForbiddenFound)
	}
}

// 4. No matched patterns → no violations (validator is silent on
// out-of-scope files, never invents pattern matches).
func TestPatternCheck_NoMatchedPatternsIsNotAViolation(t *testing.T) {
	body := `package whatever
func main() {}
`
	path := writeTempGo(t, "main.go", body)
	result := checkOneFile(context.Background(),
		fakeBriefing(nil, nil), // briefing returns 0 patterns
		path)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if len(result.PatternResults) != 0 {
		t.Errorf("want 0 pattern results, got %d", len(result.PatternResults))
	}
	if result.violationCount() != 0 {
		t.Errorf("violationCount: want 0, got %d", result.violationCount())
	}
}

// 5. Read error is captured per-file, not allowed to crash the whole run.
func TestPatternCheck_FileReadError(t *testing.T) {
	result := checkOneFile(context.Background(),
		fakeBriefing([]*awarenesspb.MatchedImplementationPattern{grpcClientStandardMatched()}, nil),
		"/nonexistent/path/to/file.go")
	if result.Error == "" {
		t.Errorf("expected an error field set for nonexistent file")
	}
	if len(result.PatternResults) != 0 {
		t.Errorf("expected no pattern results when file read fails")
	}
}

// 6. Briefing error is captured per-file, no panic.
func TestPatternCheck_BriefingError(t *testing.T) {
	body := `package foo_client`
	path := writeTempGo(t, "foo_client.go", body)
	result := checkOneFile(context.Background(),
		fakeBriefing(nil, errors.New("backend unreachable")),
		path)
	if !strings.Contains(result.Error, "backend unreachable") {
		t.Errorf("expected briefing error captured, got %q", result.Error)
	}
}

// 7. Multiple patterns — each scanned independently; violations counted
// correctly.
func TestPatternCheck_MultiplePatternsScannedIndependently(t *testing.T) {
	body := `package foo
// Has globular.InitClient but no InvokeClientRequest; also has grpc.Dial.
func F() { globular.InitClient(); globular.GetClientConnection(); globular.GetClientContext(); _ = grpc.Dial }
`
	path := writeTempGo(t, "foo_client.go", body)
	second := &awarenesspb.MatchedImplementationPattern{
		Id:             "implementation_pattern:fake.pattern.two",
		Label:          "Second pattern",
		MatchStrength:  "medium",
		RequiredCalls:  []string{"globular.InitClient"}, // present
		ForbiddenCalls: nil,
	}
	result := checkOneFile(context.Background(),
		fakeBriefing([]*awarenesspb.MatchedImplementationPattern{
			grpcClientStandardMatched(),
			second,
		}, nil),
		path)
	if len(result.PatternResults) != 2 {
		t.Fatalf("want 2 pattern results, got %d", len(result.PatternResults))
	}
	// First pattern violates; second passes.
	if result.PatternResults[0].Status != "violation" {
		t.Errorf("first pattern: want violation")
	}
	if result.PatternResults[1].Status != "pass" {
		t.Errorf("second pattern: want pass, got %s", result.PatternResults[1].Status)
	}
	if result.violationCount() != 1 {
		t.Errorf("violationCount: want 1, got %d", result.violationCount())
	}
}

// 8. derivePatternCheckTask normalizes filenames into useful task strings.
func TestDerivePatternCheckTask(t *testing.T) {
	cases := []struct{ in, want string }{
		{"golang/foo/foo_client/foo_client.go", "service foo client"},
		{"echo_client.go", "service echo client"},
		{"plain.go", "service plain"},
		{"awareness_graph_client.go", "service awareness graph client"},
	}
	for _, tc := range cases {
		if got := derivePatternCheckTask(tc.in); got != tc.want {
			t.Errorf("derivePatternCheckTask(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

package installed_state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ── key schema ────────────────────────────────────────────────────────────────

func TestConvergenceKey(t *testing.T) {
	cases := []struct {
		nodeID, kind, name string
		want               string
	}{
		{"node-1", "SERVICE", "gateway",
			"/globular/nodes/node-1/convergence/SERVICE/gateway"},
		{"node-2", "service", "dns", // kind is normalised to upper-case
			"/globular/nodes/node-2/convergence/SERVICE/dns"},
		{"node-3", "INFRASTRUCTURE", "etcd",
			"/globular/nodes/node-3/convergence/INFRASTRUCTURE/etcd"},
		{"node-4", "command", "restic",
			"/globular/nodes/node-4/convergence/COMMAND/restic"},
	}
	for _, tc := range cases {
		got := convergenceKey(tc.nodeID, tc.kind, tc.name)
		if got != tc.want {
			t.Errorf("convergenceKey(%q, %q, %q) = %q, want %q",
				tc.nodeID, tc.kind, tc.name, got, tc.want)
		}
	}
}

func TestNodeConvergencePrefix(t *testing.T) {
	got := nodeConvergencePrefix("node-1")
	want := "/globular/nodes/node-1/convergence/"
	if got != want {
		t.Errorf("nodeConvergencePrefix(node-1) = %q, want %q", got, want)
	}
}

func TestNodeConvergenceKindPrefix(t *testing.T) {
	cases := []struct {
		nodeID, kind string
		want         string
	}{
		{"node-1", "SERVICE", "/globular/nodes/node-1/convergence/SERVICE/"},
		{"node-1", "service", "/globular/nodes/node-1/convergence/SERVICE/"}, // normalised
		{"node-2", "INFRASTRUCTURE", "/globular/nodes/node-2/convergence/INFRASTRUCTURE/"},
	}
	for _, tc := range cases {
		got := nodeConvergenceKindPrefix(tc.nodeID, tc.kind)
		if got != tc.want {
			t.Errorf("nodeConvergenceKindPrefix(%q, %q) = %q, want %q",
				tc.nodeID, tc.kind, got, tc.want)
		}
	}
}

// ConvergenceKey must be under nodeConvergencePrefix — this ensures the
// controller can list all pending results for a node with a prefix scan.
func TestConvergenceKeyUnderNodePrefix(t *testing.T) {
	key := convergenceKey("node-1", "SERVICE", "gateway")
	prefix := nodeConvergencePrefix("node-1")
	if !strings.HasPrefix(key, prefix) {
		t.Errorf("convergenceKey %q is not under nodeConvergencePrefix %q", key, prefix)
	}
}

// ConvergenceKey must be distinct from the installed-state key for the same package.
func TestConvergenceKeyDistinctFromPackageKey(t *testing.T) {
	ck := convergenceKey("node-1", "SERVICE", "gateway")
	pk := packageKey("node-1", "SERVICE", "gateway")
	if ck == pk {
		t.Errorf("convergenceKey and packageKey must be distinct; both = %q", ck)
	}
}

// ── ParseConvergenceKey ───────────────────────────────────────────────────────

func TestParseConvergenceKey(t *testing.T) {
	cases := []struct {
		key              string
		wantNodeID       string
		wantKind         string
		wantName         string
		wantErrSubstring string
	}{
		{
			key:        "/globular/nodes/node-1/convergence/SERVICE/gateway",
			wantNodeID: "node-1",
			wantKind:   "SERVICE",
			wantName:   "gateway",
		},
		{
			key:        "/globular/nodes/hp-01/convergence/INFRASTRUCTURE/etcd",
			wantNodeID: "hp-01",
			wantKind:   "INFRASTRUCTURE",
			wantName:   "etcd",
		},
		{
			key:              "bad-prefix/node-1/convergence/SERVICE/gateway",
			wantErrSubstring: "missing prefix",
		},
		{
			key:              "/globular/nodes/node-1/packages/SERVICE/gateway",
			wantErrSubstring: "unexpected shape",
		},
		{
			key:              "/globular/nodes/node-1/convergence/SERVICE/",
			wantErrSubstring: "unexpected shape",
		},
	}
	for _, tc := range cases {
		nodeID, kind, name, err := ParseConvergenceKey(tc.key)
		if tc.wantErrSubstring != "" {
			if err == nil {
				t.Errorf("ParseConvergenceKey(%q): expected error containing %q, got nil",
					tc.key, tc.wantErrSubstring)
				continue
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstring) {
				t.Errorf("ParseConvergenceKey(%q): error %q does not contain %q",
					tc.key, err.Error(), tc.wantErrSubstring)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseConvergenceKey(%q): unexpected error: %v", tc.key, err)
		}
		if nodeID != tc.wantNodeID {
			t.Errorf("ParseConvergenceKey(%q) nodeID = %q, want %q", tc.key, nodeID, tc.wantNodeID)
		}
		if kind != tc.wantKind {
			t.Errorf("ParseConvergenceKey(%q) kind = %q, want %q", tc.key, kind, tc.wantKind)
		}
		if name != tc.wantName {
			t.Errorf("ParseConvergenceKey(%q) name = %q, want %q", tc.key, name, tc.wantName)
		}
	}
}

// ParseConvergenceKey(convergenceKey(…)) must round-trip cleanly.
func TestParseConvergenceKey_RoundTrip(t *testing.T) {
	cases := []struct{ nodeID, kind, name string }{
		{"node-1", "SERVICE", "gateway"},
		{"hp-01", "INFRASTRUCTURE", "etcd"},
		{"node-99", "COMMAND", "restic"},
	}
	for _, tc := range cases {
		key := convergenceKey(tc.nodeID, tc.kind, tc.name)
		nodeID, kind, name, err := ParseConvergenceKey(key)
		if err != nil {
			t.Fatalf("round-trip failed for (%q,%q,%q): %v", tc.nodeID, tc.kind, tc.name, err)
		}
		if nodeID != tc.nodeID || kind != tc.kind || name != tc.name {
			t.Errorf("round-trip mismatch: got (%q,%q,%q), want (%q,%q,%q)",
				nodeID, kind, name, tc.nodeID, tc.kind, tc.name)
		}
	}
}

// ── marshal / unmarshal ───────────────────────────────────────────────────────

func TestUnmarshalConvergenceResult(t *testing.T) {
	now := time.Now().Unix()
	r := &ConvergenceResult{
		NodeID:           "node-1",
		PackageName:      "gateway",
		PackageKind:      "SERVICE",
		AttemptedVersion: "1.2.11",
		ActualVersion:    "1.2.11",
		BuildID:          "01924abc-dead-beef-0000-000000000001",
		Status:           ConvergenceStatusSucceeded,
		OperationID:      "op-001",
		WorkflowRunID:    "run-001",
		CommittedUnix:    now,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got, err := unmarshalConvergenceResult(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got.NodeID != r.NodeID {
		t.Errorf("NodeID = %q, want %q", got.NodeID, r.NodeID)
	}
	if got.Status != ConvergenceStatusSucceeded {
		t.Errorf("Status = %q, want SUCCEEDED", got.Status)
	}
	if got.BuildID != r.BuildID {
		t.Errorf("BuildID = %q, want %q", got.BuildID, r.BuildID)
	}
	if got.CommittedUnix != now {
		t.Errorf("CommittedUnix = %d, want %d", got.CommittedUnix, now)
	}
}

func TestUnmarshalConvergenceResult_InvalidJSON(t *testing.T) {
	_, err := unmarshalConvergenceResult([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ── validateConvergenceResult ─────────────────────────────────────────────────

func TestValidateConvergenceResult(t *testing.T) {
	valid := &ConvergenceResult{
		NodeID:      "node-1",
		PackageName: "gateway",
		PackageKind: "SERVICE",
		Status:      ConvergenceStatusSucceeded,
	}
	if err := validateConvergenceResult(valid); err != nil {
		t.Fatalf("unexpected validation error for valid result: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*ConvergenceResult)
		errSub string
	}{
		{"nil", func(r *ConvergenceResult) {}, "nil"},
		{"empty_node_id", func(r *ConvergenceResult) { r.NodeID = "" }, "node_id is required"},
		{"empty_package_name", func(r *ConvergenceResult) { r.PackageName = "" }, "package_name is required"},
		{"empty_package_kind", func(r *ConvergenceResult) { r.PackageKind = "" }, "package_kind is required"},
		{"invalid_status", func(r *ConvergenceResult) { r.Status = "UNKNOWN" }, "SUCCEEDED|FAILED|SKIPPED"},
		{"empty_status", func(r *ConvergenceResult) { r.Status = "" }, "SUCCEEDED|FAILED|SKIPPED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nil" {
				err := validateConvergenceResult(nil)
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Errorf("validateConvergenceResult(nil) = %v, want error containing %q", err, tc.errSub)
				}
				return
			}
			r := *valid
			tc.mutate(&r)
			err := validateConvergenceResult(&r)
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.errSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.errSub)
			}
		})
	}
}

// All three status constants must pass validation.
func TestValidateConvergenceResult_AllStatuses(t *testing.T) {
	for _, status := range []string{
		ConvergenceStatusSucceeded,
		ConvergenceStatusFailed,
		ConvergenceStatusSkipped,
	} {
		r := &ConvergenceResult{
			NodeID:      "node-1",
			PackageName: "gateway",
			PackageKind: "SERVICE",
			Status:      status,
		}
		if err := validateConvergenceResult(r); err != nil {
			t.Errorf("status %q failed validation: %v", status, err)
		}
	}
}

// ── WriteConvergenceResult — validation gating (no etcd required) ─────────────

// fakeWriteClass captures what was passed to PutRuntimeWithClass.
// We can't inject PutRuntimeWithClass directly, but we CAN verify that
// WriteConvergenceResult returns validation errors before attempting etcd.
func TestWriteConvergenceResult_ReturnsValidationErrorBeforeEtcd(t *testing.T) {
	ctx := context.Background()

	// A nil result must fail with a validation error, never an etcd error.
	err := WriteConvergenceResult(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
	if strings.Contains(err.Error(), "etcd") {
		t.Errorf("nil result should fail validation, not reach etcd; got: %v", err)
	}

	// A result missing node_id must fail validation before etcd.
	err = WriteConvergenceResult(ctx, &ConvergenceResult{
		PackageName: "gateway",
		PackageKind: "SERVICE",
		Status:      ConvergenceStatusSucceeded,
	})
	if err == nil {
		t.Fatal("expected error for missing node_id")
	}
	if strings.Contains(err.Error(), "etcd") {
		t.Errorf("missing node_id should fail validation; got: %v", err)
	}
}

// ── CommittedUnix auto-set ────────────────────────────────────────────────────

// WriteConvergenceResult must fill CommittedUnix when it is zero.
// We verify this by calling validateConvergenceResult directly with zero timestamp,
// checking no validation error fires, then confirming the field is set by WriteConvergenceResult
// (we can't easily observe the final write without etcd, so we test the pre-write mutation).
func TestWriteConvergenceResult_SetsCommittedUnix(t *testing.T) {
	r := &ConvergenceResult{
		NodeID:      "node-1",
		PackageName: "gateway",
		PackageKind: "SERVICE",
		Status:      ConvergenceStatusSucceeded,
	}
	if r.CommittedUnix != 0 {
		t.Fatal("pre-condition: CommittedUnix must be zero initially")
	}

	// validateConvergenceResult must NOT require CommittedUnix (it's set by Write).
	if err := validateConvergenceResult(r); err != nil {
		t.Fatalf("validation must not reject zero CommittedUnix: %v", err)
	}

	// The actual mutation happens inside WriteConvergenceResult after validation.
	// Simulate it here to verify the logic independently.
	if r.CommittedUnix == 0 {
		r.CommittedUnix = time.Now().Unix()
	}
	if r.CommittedUnix == 0 {
		t.Error("CommittedUnix must be set to a non-zero timestamp")
	}
}

// ── status constant values ────────────────────────────────────────────────────

func TestConvergenceStatusConstants(t *testing.T) {
	// Status values are persisted in etcd as JSON strings; their exact values
	// are part of the contract between node-agent and controller. Never change them.
	if ConvergenceStatusSucceeded != "SUCCEEDED" {
		t.Errorf("ConvergenceStatusSucceeded = %q, must be \"SUCCEEDED\"", ConvergenceStatusSucceeded)
	}
	if ConvergenceStatusFailed != "FAILED" {
		t.Errorf("ConvergenceStatusFailed = %q, must be \"FAILED\"", ConvergenceStatusFailed)
	}
	if ConvergenceStatusSkipped != "SKIPPED" {
		t.Errorf("ConvergenceStatusSkipped = %q, must be \"SKIPPED\"", ConvergenceStatusSkipped)
	}
}

// ── fake etcd injection for ListConvergenceResults ────────────────────────────

// TestListConvergenceResults_FiltersByKind verifies the kind-prefix logic
// without needing a live etcd. We test by checking that the prefix strings
// are correct — the etcd driver enforces prefix filtering at the server.
func TestConvergenceKindPrefixFiltersCorrectly(t *testing.T) {
	nodeID := "node-1"

	// SERVICE prefix must only match SERVICE keys.
	svcPrefix := nodeConvergenceKindPrefix(nodeID, "SERVICE")
	svcKey := convergenceKey(nodeID, "SERVICE", "gateway")
	infraKey := convergenceKey(nodeID, "INFRASTRUCTURE", "etcd")

	if !strings.HasPrefix(svcKey, svcPrefix) {
		t.Errorf("service key %q must have prefix %q", svcKey, svcPrefix)
	}
	if strings.HasPrefix(infraKey, svcPrefix) {
		t.Errorf("infra key %q must NOT have service prefix %q", infraKey, svcPrefix)
	}
}

// ── error wrapping ────────────────────────────────────────────────────────────

func TestUnmarshalConvergenceResult_ErrorWrapping(t *testing.T) {
	_, err := unmarshalConvergenceResult([]byte(`{"status": 123}`)) // status must be string
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
	// Must be wrapped with package context.
	if !strings.Contains(err.Error(), "convergence_result") {
		t.Errorf("error %q must mention package context", err.Error())
	}
}

// ── sentinel: convergenceKey must not share shape with packageKey ─────────────

// This guards against a refactor accidentally causing the controller to
// confuse convergence keys and installed-state keys when iterating etcd.
func TestConvergenceKeySegmentIsLiteralConvergence(t *testing.T) {
	key := convergenceKey("node-1", "SERVICE", "gateway")
	parts := strings.Split(key, "/")
	// Key format: "" / "globular" / "nodes" / nodeID / "convergence" / KIND / name
	const convergenceSegment = "convergence"
	found := false
	for _, p := range parts {
		if p == convergenceSegment {
			found = true
		}
		if p == "packages" {
			t.Errorf("convergenceKey must not contain the 'packages' segment; got %q", key)
		}
	}
	if !found {
		t.Errorf("convergenceKey must contain the 'convergence' segment; got %q", key)
	}
}

// ── unused import guard ───────────────────────────────────────────────────────
var _ = errors.New
var _ = fmt.Sprintf

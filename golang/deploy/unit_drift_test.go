package deploy

// unit_drift_test.go — Phase 5b of the Diagnostic Honesty Refactor.
//
// Pins the contract of ParseSystemdUnit and DetectEffectiveUnitDrift.
// The two failure shapes the brief calls out:
//
//   - effective Type differs from expected   → drift, finding emitted
//   - effective ExecStart points to unexpected binary → drift, finding emitted
//
// The verdict's Drifts list is the per-disagreement evidence operators
// see in doctor output — keep those strings stable so a UI parsing them
// doesn't regress.

import (
	"reflect"
	"strings"
	"testing"
)

// Rendered unit fixture used across drift tests. Real Globular units carry
// many more directives; the comparison is field-level, so this minimal
// shape is enough to exercise every code path.
const renderedUnitFixture = `[Unit]
Description=Globular foo
After=network.target

[Service]
Type=simple
User=globular
Group=globular
ExecStart=/usr/lib/globular/bin/foo_server --flag=1
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`

// ─────────────────────────────────────────────────────────────────────────
// ParseSystemdUnit — extracts directive values per section. Pins the seam
// the drift detector reads through.
// ─────────────────────────────────────────────────────────────────────────

func TestParseSystemdUnit_ExtractsServiceDirectives(t *testing.T) {
	d := ParseSystemdUnit(renderedUnitFixture)
	cases := map[string]string{
		"Service.Type":      "simple",
		"Service.User":      "globular",
		"Service.ExecStart": "/usr/lib/globular/bin/foo_server --flag=1",
		"Service.Restart":   "on-failure",
		"Unit.Description":  "Globular foo",
	}
	for key, want := range cases {
		got := d[key]
		if len(got) != 1 || got[0] != want {
			t.Errorf("ParseSystemdUnit[%q]=%v want=[%q]", key, got, want)
		}
	}
}

func TestParseSystemdUnit_PreservesRepeatedDirectives(t *testing.T) {
	unit := `[Service]
ExecStartPre=/usr/bin/mkdir -p /var/lib/x
ExecStartPre=/usr/bin/touch /var/lib/x/.ready
Environment=A=1
Environment=B=2
`
	d := ParseSystemdUnit(unit)
	if len(d["Service.ExecStartPre"]) != 2 {
		t.Errorf("repeated ExecStartPre lost: %v", d["Service.ExecStartPre"])
	}
	if len(d["Service.Environment"]) != 2 {
		t.Errorf("repeated Environment lost: %v", d["Service.Environment"])
	}
}

func TestParseSystemdUnit_IgnoresCommentsAndBlanks(t *testing.T) {
	unit := `# leading comment
; another comment style

[Service]
# inline comment
Type=simple
`
	d := ParseSystemdUnit(unit)
	if got, want := d["Service.Type"], []string{"simple"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Type lost to comment handling: %v want=%v", got, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// DetectEffectiveUnitDrift — the brief's two signature drift cases.
// ─────────────────────────────────────────────────────────────────────────

func TestDetectEffectiveUnitDrift_AllAgree_Verified(t *testing.T) {
	eff := EffectiveUnitProperties{
		Type:         "simple",
		ExecStart:    "{ path=/usr/lib/globular/bin/foo_server ; argv[]=foo_server --flag=1 ; ignore_errors=no }",
		FragmentPath: "/etc/systemd/system/globular-foo.service",
		ActiveState:  "active",
		SubState:     "running",
	}
	v := DetectEffectiveUnitDrift(renderedUnitFixture, eff)
	if v.Status != UnitDriftVerified {
		t.Errorf("Status=%q want=%q (drifts=%v)", v.Status, UnitDriftVerified, v.Drifts)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want empty when verified", v.FindingID)
	}
}

func TestDetectEffectiveUnitDrift_TypeDiffers_Drift(t *testing.T) {
	eff := EffectiveUnitProperties{
		Type:         "forking", // rendered says simple
		ExecStart:    "{ path=/usr/lib/globular/bin/foo_server ; argv[]=foo_server --flag=1 }",
		FragmentPath: "/etc/systemd/system/globular-foo.service",
	}
	v := DetectEffectiveUnitDrift(renderedUnitFixture, eff)
	if v.Status != UnitDriftDrift {
		t.Fatalf("Status=%q want=%q (drifts=%v)", v.Status, UnitDriftDrift, v.Drifts)
	}
	if v.FindingID != FindingUnitEffectiveConfigDrift {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingUnitEffectiveConfigDrift)
	}
	wantDrift := "Type: rendered=simple, effective=forking"
	if !containsExact(v.Drifts, wantDrift) {
		t.Errorf("drifts %v missing %q", v.Drifts, wantDrift)
	}
}

func TestDetectEffectiveUnitDrift_ExecStartPathDiffers_Drift(t *testing.T) {
	eff := EffectiveUnitProperties{
		Type:         "simple",
		ExecStart:    "{ path=/usr/local/bin/old_foo ; argv[]=old_foo --flag=1 }",
		FragmentPath: "/etc/systemd/system/globular-foo.service",
	}
	v := DetectEffectiveUnitDrift(renderedUnitFixture, eff)
	if v.Status != UnitDriftDrift {
		t.Fatalf("Status=%q want=%q (drifts=%v)", v.Status, UnitDriftDrift, v.Drifts)
	}
	if v.FindingID != FindingUnitEffectiveConfigDrift {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingUnitEffectiveConfigDrift)
	}
	wantDrift := "ExecStart: rendered=/usr/lib/globular/bin/foo_server, effective=/usr/local/bin/old_foo"
	if !containsExact(v.Drifts, wantDrift) {
		t.Errorf("drifts %v missing %q", v.Drifts, wantDrift)
	}
}

func TestDetectEffectiveUnitDrift_MissingFragmentPath_Drift(t *testing.T) {
	// systemd returned no FragmentPath and no unit file hash — this is
	// the "no on-disk unit file" case the brief calls out.
	eff := EffectiveUnitProperties{
		Type:        "simple",
		ExecStart:   "{ path=/usr/lib/globular/bin/foo_server ; argv[]=foo_server --flag=1 }",
		ActiveState: "active",
	}
	v := DetectEffectiveUnitDrift(renderedUnitFixture, eff)
	if v.Status != UnitDriftDrift {
		t.Fatalf("Status=%q want=drift", v.Status)
	}
	found := false
	for _, d := range v.Drifts {
		if strings.HasPrefix(d, "FragmentPath:") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing FragmentPath drift line: %v", v.Drifts)
	}
}

func TestDetectEffectiveUnitDrift_EmptyEffective_UnknownNotDrift(t *testing.T) {
	// systemctl show failed entirely. The detector must NOT assert drift
	// against an empty effective bag — that would be exactly the lie this
	// refactor is correcting (treating missing proof as failure-or-success).
	v := DetectEffectiveUnitDrift(renderedUnitFixture, EffectiveUnitProperties{})
	if v.Status != UnitDriftUnknown {
		t.Errorf("Status=%q want=%q for empty effective bag", v.Status, UnitDriftUnknown)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want empty when unknown", v.FindingID)
	}
}

func TestDetectEffectiveUnitDrift_EmptyRendered_Unknown(t *testing.T) {
	v := DetectEffectiveUnitDrift("", EffectiveUnitProperties{Type: "simple"})
	if v.Status != UnitDriftUnknown {
		t.Errorf("Status=%q want=%q for empty rendered content", v.Status, UnitDriftUnknown)
	}
}

func TestDetectEffectiveUnitDrift_MultipleDrifts_AllReported(t *testing.T) {
	eff := EffectiveUnitProperties{
		Type:         "forking",
		ExecStart:    "{ path=/wrong/path ; argv[]=wrong --flag=1 }",
		FragmentPath: "/etc/systemd/system/globular-foo.service",
	}
	v := DetectEffectiveUnitDrift(renderedUnitFixture, eff)
	if v.Status != UnitDriftDrift {
		t.Fatalf("Status=%q want=drift", v.Status)
	}
	if len(v.Drifts) < 2 {
		t.Errorf("expected at least 2 drift lines; got %v", v.Drifts)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// extractExecStartPath — robust to both the bracketed systemctl-show format
// and a literal path fallback.
// ─────────────────────────────────────────────────────────────────────────

func TestExtractExecStartPath_BracketedFormat(t *testing.T) {
	in := "{ path=/usr/bin/foo ; argv[]=foo a b ; ignore_errors=no ; start_time=foo }"
	if got, want := extractExecStartPath(in), "/usr/bin/foo"; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestExtractExecStartPath_LiteralPathFallback(t *testing.T) {
	// Older systemd or odd configurations may return the path literally.
	in := "/usr/bin/foo --flag"
	if got, want := extractExecStartPath(in), "/usr/bin/foo"; got != want {
		t.Errorf("got=%q want=%q", got, want)
	}
}

func TestExtractExecStartPath_Empty(t *testing.T) {
	if got := extractExecStartPath(""); got != "" {
		t.Errorf("got=%q want empty", got)
	}
}

func containsExact(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

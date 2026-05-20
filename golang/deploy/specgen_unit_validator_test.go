package deploy

// specgen_unit_validator_test.go — Phase 5 (Diagnostic Honesty Refactor).
//
// Pins the contract of ValidateSystemdUnit. The function catches duplicate
// singleton directives in [Service] (the exact bug `acdcb436` hotfixed
// downstream) so the spec build fails before the bad unit reaches /etc/.
//
// The init() in specgen.go runs ValidateSystemdUnit against a maximal probe
// render of the template literal at package load — these tests verify that
// path stays green AND that the validator catches regressions.

import (
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────
// Detection — the bug that started Phase 5: a duplicate Type= line where
// systemd silently picks the last one. Must produce a failure citing the
// directive and the line number for fast triage.
// ─────────────────────────────────────────────────────────────────────────

func TestValidateSystemdUnit_RejectsDuplicateType(t *testing.T) {
	unit := strings.Join([]string{
		"[Unit]",
		"Description=Globular bad",
		"[Service]",
		"Type=simple",
		"ExecStart=/usr/bin/true",
		"Type=notify", // silently overrides — the bug.
		"[Install]",
		"WantedBy=multi-user.target",
	}, "\n")

	err := ValidateSystemdUnit(unit)
	if err == nil {
		t.Fatal("expected error for duplicate Type=, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "duplicate") || !strings.Contains(msg, "Type") {
		t.Errorf("error message missing directive name: %q", msg)
	}
	if !strings.Contains(msg, "systemd.unit_duplicate_directive") {
		t.Errorf("error message must surface the finding id so doctor / awareness can correlate: %q", msg)
	}
}

func TestValidateSystemdUnit_RejectsDuplicateUser(t *testing.T) {
	unit := strings.Join([]string{
		"[Service]",
		"User=globular",
		"ExecStart=/usr/bin/true",
		"User=root",
	}, "\n")
	if err := ValidateSystemdUnit(unit); err == nil {
		t.Fatal("expected error for duplicate User=, got nil")
	}
}

func TestValidateSystemdUnit_RejectsDuplicateRestart(t *testing.T) {
	unit := strings.Join([]string{
		"[Service]",
		"Restart=on-failure",
		"Restart=always",
	}, "\n")
	if err := ValidateSystemdUnit(unit); err == nil {
		t.Fatal("expected error for duplicate Restart=")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Negative-space tests — legitimate repetitions of MULTI-OK directives must
// NOT trigger the validator. If they did, every real spec would fail and
// the gate would be useless.
// ─────────────────────────────────────────────────────────────────────────

func TestValidateSystemdUnit_AllowsRepeatedExecStartPre(t *testing.T) {
	unit := strings.Join([]string{
		"[Service]",
		"Type=simple",
		"ExecStartPre=/bin/mkdir -p /var/lib/x",
		"ExecStartPre=/bin/chown x:x /var/lib/x",
		"ExecStartPre=+/bin/sh -c 'pkill -9 -x foo || true'",
		"ExecStart=/usr/bin/foo",
	}, "\n")
	if err := ValidateSystemdUnit(unit); err != nil {
		t.Errorf("ExecStartPre is multi-OK; validator must not flag it: %v", err)
	}
}

func TestValidateSystemdUnit_AllowsRepeatedEnvironment(t *testing.T) {
	unit := strings.Join([]string{
		"[Service]",
		"Environment=A=1",
		"Environment=B=2",
		"Environment=PATH=/x:/y",
		"ExecStart=/usr/bin/true",
	}, "\n")
	if err := ValidateSystemdUnit(unit); err != nil {
		t.Errorf("Environment is multi-OK; validator must not flag it: %v", err)
	}
}

// Duplicate directives outside [Service] are not flagged — the validator
// is scoped to where systemd's last-wins behaviour bites us. (After/Wants
// in [Unit] are repeatable by design.)
func TestValidateSystemdUnit_IgnoresUnitSectionDuplicates(t *testing.T) {
	unit := strings.Join([]string{
		"[Unit]",
		"After=network-online.target",
		"After=etcd.service",
		"[Service]",
		"Type=simple",
		"ExecStart=/usr/bin/true",
	}, "\n")
	if err := ValidateSystemdUnit(unit); err != nil {
		t.Errorf("[Unit].After duplicates are legal; validator must not flag them: %v", err)
	}
}

func TestValidateSystemdUnit_TolerantOfEmptyAndCommentLines(t *testing.T) {
	unit := strings.Join([]string{
		"# comment block",
		"",
		"[Service]",
		"",
		"# another comment",
		"Type=simple",
		"ExecStart=/usr/bin/true",
	}, "\n")
	if err := ValidateSystemdUnit(unit); err != nil {
		t.Errorf("blank/comment lines must not confuse the validator: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Embedded-in-YAML — the canonical input to ValidateSystemdUnit is the
// rendered YAML spec (the unit is inside a `content: |` block). The
// validator must work on that without needing the unit pre-extracted.
// ─────────────────────────────────────────────────────────────────────────

func TestValidateSystemdUnit_HandlesYAMLEmbeddedUnit(t *testing.T) {
	yamlSpec := `
service:
  name: bad
steps:
  - id: install
    type: install_services
    units:
      - name: globular-bad.service
        content: |
          [Unit]
          Description=Globular bad
          [Service]
          Type=simple
          ExecStart=/usr/bin/true
          Type=notify
          [Install]
          WantedBy=multi-user.target
`
	if err := ValidateSystemdUnit(yamlSpec); err == nil {
		t.Fatal("expected error for duplicate Type= inside YAML-embedded unit")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// isLikelyDirective — guards against false positives on shell strings.
// ─────────────────────────────────────────────────────────────────────────

func TestIsLikelyDirective(t *testing.T) {
	cases := map[string]bool{
		"Type":             true,
		"ExecStart":        true,
		"WorkingDirectory": true,
		"":                 false,
		"lowercase":        false, // systemd directives are CamelCase
		"With Space":       false,
		"With-Dash":        false,
		"foo123":           false, // first char lower
		"Type123":          true,
	}
	for name, want := range cases {
		if got := isLikelyDirective(name); got != want {
			t.Errorf("isLikelyDirective(%q)=%v want=%v", name, got, want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// End-to-end — GenerateSpec for a real entry must survive validation. If
// this test fails, the template literal has a duplicate directive and the
// init() panic would prevent the binary from starting; this test surfaces
// the failure mode with a clearer message.
// ─────────────────────────────────────────────────────────────────────────

func TestGenerateSpec_TemplateProducesValidUnit(t *testing.T) {
	// Pick an entry from the catalog if available; otherwise build a
	// minimal one. The catalog has helpers for "echo" used elsewhere.
	echo := &ServiceEntry{
		Name:         "validator-probe",
		Profiles:     []string{"core"},
		Priority:     0,
		NeedsScylla:  false,
		ExtraPath:    false,
		Capabilities: nil,
	}
	spec, err := GenerateSpec(echo)
	if err != nil {
		t.Fatalf("GenerateSpec failed (init() probe would also fail): %v", err)
	}
	if err := ValidateSystemdUnit(spec); err != nil {
		t.Fatalf("rendered template fails validator — duplicate directive in template: %v", err)
	}
}

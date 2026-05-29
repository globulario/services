package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// Project O.5 invariant: systemd.working_directory.must_be_optional
//
// Catches any future regression where a `globular-*.service` ships a bare
// `WorkingDirectory=/var/lib/globular/...` (no `-` prefix). Without the `-`,
// systemd evaluates the dir before ExecStartPre and the unit fails with
// status=200/CHDIR if the dir is missing — the exact failure mode that took
// down 5 services in Phase 1.

func withSystemdUnitDir(t *testing.T, root string) {
	t.Helper()
	old := systemdUnitDir
	systemdUnitDir = root
	t.Cleanup(func() { systemdUnitDir = old })
}

func TestSystemdWD_BareGlobularWDIsFlagged(t *testing.T) {
	td := t.TempDir()
	unit := strings.Join([]string{
		"[Service]",
		"WorkingDirectory=/var/lib/globular/example",
		"ExecStart=/usr/bin/example",
	}, "\n")
	if err := os.WriteFile(filepath.Join(td, "globular-example.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}
	withSystemdUnitDir(t, td)

	findings := (systemdWorkingDirectoryMustBeOptional{}).Evaluate(nil, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("severity=%v want WARN", findings[0].Severity)
	}
	if !strings.Contains(findings[0].Summary, "globular-example.service") {
		t.Errorf("summary missing offender: %s", findings[0].Summary)
	}
}

func TestSystemdWD_OptionalWDIsSilent(t *testing.T) {
	td := t.TempDir()
	unit := strings.Join([]string{
		"[Service]",
		"WorkingDirectory=-/var/lib/globular/example",
		"ExecStart=/usr/bin/example",
	}, "\n")
	if err := os.WriteFile(filepath.Join(td, "globular-example.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}
	withSystemdUnitDir(t, td)

	findings := (systemdWorkingDirectoryMustBeOptional{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Errorf("optional WD must not fire; got %d findings: %+v", len(findings), findings)
	}
}

func TestSystemdWD_NoWDIsSilent(t *testing.T) {
	td := t.TempDir()
	unit := "[Service]\nExecStart=/usr/bin/example\n"
	if err := os.WriteFile(filepath.Join(td, "globular-example.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}
	withSystemdUnitDir(t, td)

	findings := (systemdWorkingDirectoryMustBeOptional{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Errorf("unit without WD must be silent; got %d", len(findings))
	}
}

func TestSystemdWD_CommentedWDIsSilent(t *testing.T) {
	td := t.TempDir()
	unit := strings.Join([]string{
		"[Service]",
		"# WorkingDirectory=/var/lib/globular/example",
		"; WorkingDirectory=/var/lib/globular/example",
		"WorkingDirectory=-/var/lib/globular/real",
		"ExecStart=/usr/bin/example",
	}, "\n")
	if err := os.WriteFile(filepath.Join(td, "globular-example.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}
	withSystemdUnitDir(t, td)

	findings := (systemdWorkingDirectoryMustBeOptional{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Errorf("comments must not trigger detection; got %d", len(findings))
	}
}

func TestSystemdWD_NonGlobularUnitIgnored(t *testing.T) {
	td := t.TempDir()
	// Not a globular-*.service — out of scope, must be ignored.
	unit := "[Service]\nWorkingDirectory=/var/lib/globular/example\n"
	if err := os.WriteFile(filepath.Join(td, "third-party.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}
	withSystemdUnitDir(t, td)

	findings := (systemdWorkingDirectoryMustBeOptional{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Errorf("non-globular unit must be ignored; got %d", len(findings))
	}
}

func TestSystemdWD_MultipleOffendersAggregated(t *testing.T) {
	td := t.TempDir()
	for _, n := range []string{"globular-a.service", "globular-b.service", "globular-c.service"} {
		body := "[Service]\nWorkingDirectory=/var/lib/globular/" + strings.TrimSuffix(strings.TrimPrefix(n, "globular-"), ".service") + "\n"
		if err := os.WriteFile(filepath.Join(td, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	withSystemdUnitDir(t, td)

	findings := (systemdWorkingDirectoryMustBeOptional{}).Evaluate(nil, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 aggregated finding, got %d", len(findings))
	}
	s := findings[0].Summary
	for _, n := range []string{"globular-a.service", "globular-b.service", "globular-c.service"} {
		if !strings.Contains(s, n) {
			t.Errorf("summary should mention %s; got: %s", n, s)
		}
	}
}

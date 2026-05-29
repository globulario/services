package systemdutil

import (
	"strings"
	"testing"
)

func TestNormalize_BareGlobularRewritten(t *testing.T) {
	in := strings.Join([]string{
		"[Service]",
		"WorkingDirectory=/var/lib/globular/ai_memory",
		"ExecStart=/usr/lib/globular/bin/ai_memory_server",
	}, "\n")
	got := NormalizeUnitWorkingDirectoryString(in)
	if !strings.Contains(got, "WorkingDirectory=-/var/lib/globular/ai_memory") {
		t.Errorf("expected WD rewrite, got:\n%s", got)
	}
	if strings.Contains(got, "WorkingDirectory=/var/lib/globular/ai_memory") {
		t.Errorf("bare WD still present:\n%s", got)
	}
}

func TestNormalize_AlreadyOptional_LeftAlone(t *testing.T) {
	in := strings.Join([]string{
		"[Service]",
		"WorkingDirectory=-/var/lib/globular/cluster_doctor",
		"ExecStart=/usr/lib/globular/bin/cluster_doctor_server",
	}, "\n")
	got := NormalizeUnitWorkingDirectoryString(in)
	if got != in {
		t.Errorf("idempotent normalize should preserve already-optional WD; got:\n%s", got)
	}
}

func TestNormalize_NonGlobularPath_Untouched(t *testing.T) {
	in := strings.Join([]string{
		"[Service]",
		"WorkingDirectory=/etc/something",
		"ExecStart=/usr/bin/whatever",
	}, "\n")
	got := NormalizeUnitWorkingDirectoryString(in)
	if got != in {
		t.Errorf("non-Globular WD should be left untouched; got:\n%s", got)
	}
}

func TestNormalize_CommentedLine_NotRewritten(t *testing.T) {
	in := strings.Join([]string{
		"[Service]",
		"# WorkingDirectory=/var/lib/globular/example",
		"; WorkingDirectory=/var/lib/globular/example",
		"WorkingDirectory=-/var/lib/globular/real",
	}, "\n")
	got := NormalizeUnitWorkingDirectoryString(in)
	if got != in {
		t.Errorf("comments must not be rewritten; got:\n%s", got)
	}
}

func TestNormalize_LeadingWhitespace_Preserved(t *testing.T) {
	in := "    WorkingDirectory=/var/lib/globular/x"
	got := NormalizeUnitWorkingDirectoryString(in)
	want := "    WorkingDirectory=-/var/lib/globular/x"
	if got != want {
		t.Errorf("leading whitespace not preserved\n got=%q\nwant=%q", got, want)
	}
}

func TestNormalize_NoWDSection_Untouched(t *testing.T) {
	in := strings.Join([]string{
		"[Unit]",
		"Description=Example",
		"[Service]",
		"ExecStart=/usr/bin/example",
	}, "\n")
	got := NormalizeUnitWorkingDirectoryString(in)
	if got != in {
		t.Errorf("unit without WD must be unchanged; got:\n%s", got)
	}
}

func TestNormalize_MultipleWDLines(t *testing.T) {
	in := strings.Join([]string{
		"[Service]",
		"WorkingDirectory=/var/lib/globular/a",
		"WorkingDirectory=/var/lib/globular/b", // unusual but valid input
		"ExecStart=/usr/bin/x",
	}, "\n")
	got := NormalizeUnitWorkingDirectoryString(in)
	if strings.Count(got, "WorkingDirectory=-/var/lib/globular/") != 2 {
		t.Errorf("expected both WD lines rewritten; got:\n%s", got)
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	in := "WorkingDirectory=/var/lib/globular/example"
	once := NormalizeUnitWorkingDirectoryString(in)
	twice := NormalizeUnitWorkingDirectoryString(once)
	if once != twice {
		t.Errorf("normalize must be idempotent\n once=%q\ntwice=%q", once, twice)
	}
}

func TestHasBareGlobularWorkingDirectory_TruePositive(t *testing.T) {
	in := []byte("[Service]\nWorkingDirectory=/var/lib/globular/foo\n")
	if !HasBareGlobularWorkingDirectory(in) {
		t.Error("expected bare WD detection")
	}
}

func TestHasBareGlobularWorkingDirectory_FalsePositive_OptionalForm(t *testing.T) {
	in := []byte("WorkingDirectory=-/var/lib/globular/foo\n")
	if HasBareGlobularWorkingDirectory(in) {
		t.Error("optional WD must not trigger bare detection")
	}
}

func TestHasBareGlobularWorkingDirectory_NonGlobularDoesNotTrigger(t *testing.T) {
	in := []byte("WorkingDirectory=/srv/other\n")
	if HasBareGlobularWorkingDirectory(in) {
		t.Error("non-Globular WD must not trigger bare detection")
	}
}

func TestHasBareGlobularWorkingDirectory_CommentDoesNotTrigger(t *testing.T) {
	in := []byte("# WorkingDirectory=/var/lib/globular/x\n")
	if HasBareGlobularWorkingDirectory(in) {
		t.Error("commented bare WD must not trigger detection")
	}
}

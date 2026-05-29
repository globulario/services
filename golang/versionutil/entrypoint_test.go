package versionutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Project T entrypoint sidecar tests.

func withTmpBaseDir(t *testing.T) string {
	t.Helper()
	td := t.TempDir()
	old := baseDir
	SetBaseDir(td)
	t.Cleanup(func() { baseDir = old })
	return td
}

func TestEntrypointPath_HyphenPackageName_PathConstructed(t *testing.T) {
	td := withTmpBaseDir(t)
	got := EntrypointPath("scylla-manager")
	want := filepath.Join(td, "scylla-manager", "entrypoint")
	if got != want {
		t.Errorf("EntrypointPath\n got=%q\nwant=%q", got, want)
	}
}

func TestWriteEntrypoint_StripsBinPrefix(t *testing.T) {
	withTmpBaseDir(t)
	cases := []struct {
		in, want string
	}{
		{"bin/scylla_manager", "scylla_manager"},
		{"./bin/scylla_manager", "scylla_manager"},
		{"scylla_manager", "scylla_manager"},
		{"  bin/scylla_manager  ", "scylla_manager"}, // trimmed
	}
	for _, c := range cases {
		if err := WriteEntrypoint("scylla-manager", c.in); err != nil {
			t.Fatalf("WriteEntrypoint(%q): %v", c.in, err)
		}
		got := ReadEntrypoint("scylla-manager")
		if got != c.want {
			t.Errorf("Write/Read roundtrip for %q\n got=%q\nwant=%q", c.in, got, c.want)
		}
	}
}

func TestWriteEntrypoint_Empty_NoOp(t *testing.T) {
	td := withTmpBaseDir(t)
	if err := WriteEntrypoint("scylla-manager", ""); err != nil {
		t.Fatalf("empty entrypoint should not error: %v", err)
	}
	if err := WriteEntrypoint("scylla-manager", "   "); err != nil {
		t.Fatalf("whitespace-only entrypoint should not error: %v", err)
	}
	// No sidecar file should exist.
	if _, err := os.Stat(filepath.Join(td, "scylla-manager", "entrypoint")); !os.IsNotExist(err) {
		t.Errorf("empty entrypoint must not create a sidecar; err=%v", err)
	}
}

func TestReadEntrypoint_NoSidecar_ReturnsEmpty(t *testing.T) {
	withTmpBaseDir(t)
	got := ReadEntrypoint("legacy-pkg")
	if got != "" {
		t.Errorf("missing sidecar must return empty; got %q", got)
	}
}

func TestWriteEntrypoint_OverwritesExisting(t *testing.T) {
	withTmpBaseDir(t)
	if err := WriteEntrypoint("scylla-manager", "bin/old_name"); err != nil {
		t.Fatal(err)
	}
	if err := WriteEntrypoint("scylla-manager", "bin/new_name"); err != nil {
		t.Fatal(err)
	}
	if got := ReadEntrypoint("scylla-manager"); got != "new_name" {
		t.Errorf("last write should win; got %q", got)
	}
}

func TestEntrypointSidecar_RoundtripWithSanitizedName(t *testing.T) {
	withTmpBaseDir(t)
	// `sanitize` is called inside the helpers; underscore names should round-trip too.
	if err := WriteEntrypoint("scylla_manager_agent", "bin/scylla_manager_agent"); err != nil {
		t.Fatal(err)
	}
	got := ReadEntrypoint("scylla_manager_agent")
	// sanitize converts underscores to hyphens, so the lookup happens under
	// the hyphenated dir — write and read are symmetric so both call sanitize.
	if got != "scylla_manager_agent" {
		t.Errorf("roundtrip failed; got %q", got)
	}
	// The sidecar should live under the sanitized dir name.
	if !strings.Contains(EntrypointPath("scylla_manager_agent"), "scylla-manager-agent") {
		t.Errorf("EntrypointPath should sanitize underscores to hyphens; got %q",
			EntrypointPath("scylla_manager_agent"))
	}
}

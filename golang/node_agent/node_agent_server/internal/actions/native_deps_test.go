package actions

import (
	"testing"
)

func TestParseLDDOutput_MissingLib(t *testing.T) {
	output := `	linux-vdso.so.1 (0x00007ffc0e9ff000)
	libodbc.so.2 => not found
	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f1a2b3c4000)
`
	missing := ParseLDDOutput(output)
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing lib, got %d: %v", len(missing), missing)
	}
	if missing[0] != "libodbc.so.2" {
		t.Errorf("expected libodbc.so.2, got %q", missing[0])
	}
}

func TestParseLDDOutput_MultipleMissing(t *testing.T) {
	output := `	libodbc.so.2 => not found
	libltdl.so.7 => not found
	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f...)
`
	missing := ParseLDDOutput(output)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing libs, got %d: %v", len(missing), missing)
	}
}

func TestParseLDDOutput_AllPresent(t *testing.T) {
	output := `	linux-vdso.so.1 (0x00007ffc0e9ff000)
	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f1a2b3c4000)
	libpthread.so.0 => /lib/x86_64-linux-gnu/libpthread.so.0 (0x00007f...)
`
	missing := ParseLDDOutput(output)
	if len(missing) != 0 {
		t.Errorf("expected 0 missing libs, got %d: %v", len(missing), missing)
	}
}

func TestParseLDDOutput_EmptyOutput(t *testing.T) {
	missing := ParseLDDOutput("")
	if len(missing) != 0 {
		t.Errorf("expected 0 for empty input, got %d", len(missing))
	}
}

func TestParseLDDOutput_StaticallyLinked(t *testing.T) {
	// Statically linked binaries produce this output.
	output := `	not a dynamic executable`
	missing := ParseLDDOutput(output)
	if len(missing) != 0 {
		t.Errorf("expected 0 for static binary, got %d: %v", len(missing), missing)
	}
}

func TestParseLDDOutput_LibNameExtracted(t *testing.T) {
	// Ensure leading whitespace in ldd output is handled.
	output := "\t\tlibssl.so.1.1 => not found\n"
	missing := ParseLDDOutput(output)
	if len(missing) != 1 || missing[0] != "libssl.so.1.1" {
		t.Errorf("expected [libssl.so.1.1], got %v", missing)
	}
}

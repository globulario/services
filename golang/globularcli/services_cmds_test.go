package main

import "testing"

func TestExtractUnitWorkingDirectories(t *testing.T) {
	unit := []byte(`[Service]
WorkingDirectory=/var/lib/globular/ai_router
ExecStart=/usr/lib/globular/bin/ai_router_server
WorkingDirectory=-/var/lib/globular/optional
WorkingDirectory=relative/path
`)
	got := extractUnitWorkingDirectories(unit)
	if len(got) != 2 {
		t.Fatalf("expected 2 working directories, got %d (%v)", len(got), got)
	}
	if got[0] != "/var/lib/globular/ai_router" {
		t.Fatalf("unexpected first workdir: %q", got[0])
	}
	if got[1] != "/var/lib/globular/optional" {
		t.Fatalf("unexpected second workdir: %q", got[1])
	}
}


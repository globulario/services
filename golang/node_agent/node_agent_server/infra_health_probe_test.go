package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequireReadableByUnixUser(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "service.key")
	if err := os.WriteFile(p, []byte("k"), 0o600); err != nil {
		t.Fatalf("write temp key: %v", err)
	}
	uid := os.Getuid()
	gid := os.Getgid()
	gids := []int{gid}

	if err := requireReadableByUnixUser(p, uid, gids); err != nil {
		t.Fatalf("expected readable file to pass: %v", err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatalf("chmod 000: %v", err)
	}
	if err := requireReadableByUnixUser(p, uid, gids); err == nil {
		t.Fatal("expected unreadable file to fail")
	}

	// Supplementary group check: file owned by a different group (gid+1) with
	// group-read bit set. Should pass when gid+1 is in the supplementary list.
	if err := os.Chmod(p, 0o040); err != nil {
		t.Fatalf("chmod 040: %v", err)
	}
	supplementaryOnly := []int{gid + 1} // primary gid doesn't match file gid
	if err := requireReadableByUnixUser(p, uid+1, supplementaryOnly); err == nil {
		t.Fatal("expected failure when neither uid nor gids match file owner/group")
	}
	// Now include the file's actual gid as a supplementary gid.
	withSupplementary := []int{gid + 1, gid}
	if err := requireReadableByUnixUser(p, uid+1, withSupplementary); err != nil {
		t.Fatalf("expected pass when file gid is in supplementary list: %v", err)
	}
}

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

	if err := requireReadableByUnixUser(p, uid, gid); err != nil {
		t.Fatalf("expected readable file to pass: %v", err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatalf("chmod 000: %v", err)
	}
	if err := requireReadableByUnixUser(p, uid, gid); err == nil {
		t.Fatal("expected unreadable file to fail")
	}
}

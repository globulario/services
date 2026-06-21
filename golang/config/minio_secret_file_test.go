package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveSecretKey covers the Tier-3 credential-relocation read model:
// the MinIO root secret_key is read from a node-local file in preference to the
// inline etcd value, and falls back to the inline value when the file is
// absent/empty/unreadable (so migration is non-breaking).
func TestResolveSecretKey(t *testing.T) {
	dir := t.TempDir()

	// File present with content -> file wins over the inline value.
	good := filepath.Join(dir, "root_secret_key")
	if err := os.WriteFile(good, []byte("  file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveSecretKey("inline-secret", good); got != "file-secret" {
		t.Errorf("file should win and be trimmed, got %q", got)
	}

	// No file path -> inline value.
	if got := resolveSecretKey("inline-secret", ""); got != "inline-secret" {
		t.Errorf("no path should use inline value, got %q", got)
	}

	// Path set but file missing -> fall back to inline (non-breaking migration).
	if got := resolveSecretKey("inline-secret", filepath.Join(dir, "absent")); got != "inline-secret" {
		t.Errorf("missing file must fall back to inline value, got %q", got)
	}

	// File present but empty -> fall back to inline.
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveSecretKey("inline-secret", empty); got != "inline-secret" {
		t.Errorf("empty file must fall back to inline value, got %q", got)
	}
}

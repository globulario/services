package lkg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/globular_service/lkg"
)

func init() {
	// Redirect LKG base dir to a temp directory so tests don't touch /var.
	tmp, _ := os.MkdirTemp("", "lkg-test-*")
	lkg.OverrideBaseDir(tmp)
}

func TestStoreAndLoad(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"hello": "world"})
	if err := lkg.Store("test-sub", "mykey", 1, data); err != nil {
		t.Fatal(err)
	}
	entry, err := lkg.Load("test-sub", "mykey")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Generation != 1 {
		t.Fatalf("expected generation 1, got %d", entry.Generation)
	}
}

func TestCorruptRejected(t *testing.T) {
	data, _ := json.Marshal("valid")
	if err := lkg.Store("test-sub", "corrupt", 1, data); err != nil {
		t.Fatal(err)
	}
	// Corrupt the file directly.
	path := filepath.Join(lkg.BaseDir(), "test-sub", "corrupt-last-known-good.json")
	raw, _ := os.ReadFile(path)
	raw = append(raw[:len(raw)-2], []byte("XX")...)
	_ = os.WriteFile(path, raw, 0o640)

	if _, err := lkg.Load("test-sub", "corrupt"); err != lkg.ErrCorrupt {
		t.Fatalf("expected ErrCorrupt, got %v", err)
	}
}

func TestNotFoundReturnsErrNotFound(t *testing.T) {
	if _, err := lkg.Load("test-sub", "nonexistent-key-xyz"); err != lkg.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMonotonicGenerationGuard(t *testing.T) {
	data1, _ := json.Marshal("gen2")
	if err := lkg.Store("test-sub", "mono", 2, data1); err != nil {
		t.Fatal(err)
	}
	// Attempt to overwrite with lower generation — must be a no-op.
	data2, _ := json.Marshal("gen1-stale")
	if err := lkg.Store("test-sub", "mono", 1, data2); err != nil {
		t.Fatal(err)
	}
	entry, err := lkg.Load("test-sub", "mono")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Generation != 2 {
		t.Fatalf("stale generation should not overwrite: got %d", entry.Generation)
	}
}

func TestStoreRawAndLoadRaw(t *testing.T) {
	payload := []byte(`{"mode":"vip_failover"}`)
	if err := lkg.StoreRaw("test-sub", "raw", 1, payload); err != nil {
		t.Fatal(err)
	}
	out, err := lkg.LoadRaw("test-sub", "raw")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(payload) {
		t.Fatalf("payload mismatch: got %s", out)
	}
}

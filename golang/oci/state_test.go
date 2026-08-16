package oci

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStateStoreRoundTripAndPermissions(t *testing.T) {
	root := t.TempDir()
	store := FileStateStore{Root: root}
	want := ObservedState{APIVersion: APIVersionV1Alpha1, ServiceName: "demo", Instance: "blue", Phase: PhaseReady, Ready: true}
	if err := store.Write(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read("demo", "blue")
	if err != nil {
		t.Fatal(err)
	}
	if got.ServiceName != want.ServiceName || got.Phase != want.Phase || !got.Ready {
		t.Fatalf("state = %+v", got)
	}
	path := filepath.Join(root, "demo", "blue", "observed.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}
